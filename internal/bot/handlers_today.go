package bot

import (
	"context"
	"errors"
	"fmt"
	"time"

	tele "gopkg.in/telebot.v3"

	"fitlog/internal/domain"
)

// handleInfo handles `/info [YYYY-MM-DD]`. Without args it defaults to today.
func (b *Bot) handleInfo(c tele.Context) error {
	loc := b.deps.Location
	var day time.Time
	if args := c.Args(); len(args) > 0 {
		parsed, err := time.ParseInLocation("2006-01-02", args[0], loc)
		if err != nil {
			return b.reply(c, mdv2Escape("Не понял дату. Формат: /info 2026-05-11"))
		}
		day = parsed
	} else {
		now := time.Now().In(loc)
		day = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	}
	return b.sendDayReport(c, day)
}

// sendDayReport fetches everything for the calendar day `day` (midnight in
// the configured TZ) and replies with the verbose narrative report, split
// into multiple messages if necessary.
func (b *Bot) sendDayReport(c tele.Context, day time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	payload := InfoPayload{Day: day, Loc: b.deps.Location}
	dayEnd := day.AddDate(0, 0, 1)

	wc, err := b.loadWhoopClient(ctx)
	switch {
	case errors.Is(err, errWhoopNotConnected):
		payload.WhoopStatus = "Whoop не подключён — /connect_whoop"
	case err != nil:
		b.deps.Logger.Error("load whoop client", "err", err)
		payload.WhoopStatus = "Whoop недоступен: " + err.Error()
	default:
		// 36h window centred on the target day captures the sleep that
		// started the night before plus any straggling end-of-day data.
		rng := domain.TimeRange{From: day.Add(-18 * time.Hour), To: dayEnd.Add(6 * time.Hour)}

		var apiErr error
		var totalRecords int

		// Sleep first — used to anchor recovery via sleep_id.
		sleeps, err := wc.Sleeps(ctx, rng, 25)
		if err != nil {
			b.deps.Logger.Warn("whoop sleeps", "err", err)
			apiErr = err
		}
		totalRecords += len(sleeps)
		if s := pickSleep(sleeps, day, dayEnd); s != nil {
			payload.Sleep = s
		}

		// Recoveries: 14-day window so we can both pick the target AND compute
		// the rolling baseline used for ↑/↓ deltas.
		baseRng := domain.TimeRange{From: dayEnd.AddDate(0, 0, -14), To: dayEnd.Add(6 * time.Hour)}
		recs, err := wc.Recoveries(ctx, baseRng, 25)
		if err != nil {
			b.deps.Logger.Warn("whoop recoveries", "err", err)
			apiErr = err
		}
		totalRecords += len(recs)
		if rec := pickRecovery(recs, nil, payload.Sleep); rec != nil {
			payload.Recovery = rec
		}
		payload.Baseline = computeBaseline(recs, payload.Recovery)

		// Cycle last — use recovery.CycleID as the authoritative hint.
		var hintID int64
		if payload.Recovery != nil {
			hintID = payload.Recovery.CycleID
		}
		cycles, err := wc.Cycles(ctx, rng, 25)
		if err != nil {
			b.deps.Logger.Warn("whoop cycles", "err", err)
			apiErr = err
		}
		totalRecords += len(cycles)
		if cy := pickCycle(cycles, day, dayEnd, hintID); cy != nil {
			payload.Cycle = cy
		}

		wos, err := wc.Workouts(ctx, rng, 25)
		if err != nil {
			b.deps.Logger.Warn("whoop workouts", "err", err)
			apiErr = err
		}
		totalRecords += len(wos)
		for _, w := range wos {
			if !w.Start.Before(day) && w.Start.Before(dayEnd) {
				payload.Workouts = append(payload.Workouts, w)
			}
		}

		switch {
		case apiErr != nil && totalRecords == 0:
			payload.WhoopStatus = "Whoop API вернул ошибку: " + apiErr.Error()
		case totalRecords > 0 && payload.Cycle == nil && payload.Sleep == nil &&
			payload.Recovery == nil && len(payload.Workouts) == 0:
			payload.WhoopStatus = fmt.Sprintf(
				"Whoop отдал %d записей, но ни одна не попала в выбранный день",
				totalRecords)
		}
	}

	meals, err := b.deps.FatSecret.FoodEntriesForDay(ctx, day)
	if err != nil {
		b.deps.Logger.Warn("fatsecret food entries", "err", err)
	} else {
		payload.Meals = meals
	}

	report := FormatInfo(payload)
	for i, chunk := range SplitForTelegram(report) {
		if i > 0 {
			// Tiny separator so multi-message reports look intentional.
			chunk = "\\.\\.\\.\n" + chunk
		}
		if err := b.reply(c, chunk); err != nil {
			return fmt.Errorf("send chunk %d: %w", i, err)
		}
	}
	return nil
}

// pickSleep returns the non-nap sleep whose END is within [day, dayEnd).
// Falls back to the latest nap ending on the day if no main sleep is present.
func pickSleep(ss []domain.Sleep, day, dayEnd time.Time) *domain.Sleep {
	var main, nap *domain.Sleep
	for i := range ss {
		s := ss[i]
		if s.End.Before(day) || !s.End.Before(dayEnd) {
			continue
		}
		if s.IsNap {
			if nap == nil || s.End.After(nap.End) {
				nap = &s
			}
			continue
		}
		if main == nil || s.End.After(main.End) {
			main = &s
		}
	}
	if main != nil {
		return main
	}
	return nap
}

// pickCycle finds the cycle that represents the target day. Whoop's
// cycle.Start is at the SLEEP ONSET of the previous night, not at wakeup,
// so a strict "Start within [day, dayEnd)" filter would reject the right
// record. Strategy:
//  1. If hintID != 0 (passed from recovery.CycleID), match by ID — that's
//     the authoritative link.
//  2. Otherwise pick the cycle that overlaps the day, preferring the one
//     whose Start is most recent before dayEnd.
func pickCycle(cs []domain.Cycle, day, dayEnd time.Time, hintID int64) *domain.Cycle {
	if hintID != 0 {
		for i := range cs {
			if cs[i].ID == hintID {
				return &cs[i]
			}
		}
	}
	var out *domain.Cycle
	for i := range cs {
		c := cs[i]
		if !c.Start.Before(dayEnd) {
			continue
		}
		if c.End != nil && !c.End.After(day) {
			continue
		}
		if out == nil || c.Start.After(out.Start) {
			out = &c
		}
	}
	return out
}

// computeBaseline averages recoveries OTHER than the target day's, taking up
// to 7 of them. Returns nil if there's nothing to average.
func computeBaseline(recs []domain.Recovery, target *domain.Recovery) *Baseline {
	var rs, hs, rrs float64
	var n int
	for _, r := range recs {
		if r.Score == 0 && r.HRVMilli == 0 {
			continue // unscored
		}
		if target != nil && r.CycleID == target.CycleID {
			continue
		}
		rs += r.Score
		hs += r.HRVMilli
		rrs += r.RestingHR
		n++
		if n >= 7 {
			break
		}
	}
	if n == 0 {
		return nil
	}
	return &Baseline{
		Days:        n,
		RecoveryAvg: rs / float64(n),
		HRVAvg:      hs / float64(n),
		RHRAvg:      rrs / float64(n),
	}
}

// pickRecovery picks the recovery matching the chosen sleep (preferred)
// or cycle. Sleep_id is the more reliable link — recoveries are scored per
// sleep, and sleep_id is populated whether the recovery has been scored or not.
func pickRecovery(rs []domain.Recovery, cy *domain.Cycle, sl *domain.Sleep) *domain.Recovery {
	if sl != nil {
		for i := range rs {
			if rs[i].SleepID == sl.ExternalID {
				return &rs[i]
			}
		}
	}
	if cy != nil {
		for i := range rs {
			if rs[i].CycleID == cy.ID {
				return &rs[i]
			}
		}
	}
	return nil
}
