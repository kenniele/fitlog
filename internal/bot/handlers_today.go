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
		// proceed without whoop data
	case err != nil:
		b.deps.Logger.Error("load whoop client", "err", err)
	default:
		// 36h window centred on the target day captures the sleep that
		// started the night before plus any straggling end-of-day data.
		rng := domain.TimeRange{From: day.Add(-18 * time.Hour), To: dayEnd.Add(6 * time.Hour)}

		cycles, err := wc.Cycles(ctx, rng, 25)
		if err != nil {
			b.deps.Logger.Warn("whoop cycles", "err", err)
		}
		if cy := pickCycle(cycles, day, dayEnd); cy != nil {
			payload.Cycle = cy
		}

		sleeps, err := wc.Sleeps(ctx, rng, 25)
		if err != nil {
			b.deps.Logger.Warn("whoop sleeps", "err", err)
		}
		if s := pickSleep(sleeps, day, dayEnd); s != nil {
			payload.Sleep = s
		}

		recs, err := wc.Recoveries(ctx, rng, 25)
		if err != nil {
			b.deps.Logger.Warn("whoop recoveries", "err", err)
		}
		if rec := pickRecovery(recs, payload.Cycle, payload.Sleep); rec != nil {
			payload.Recovery = rec
		}

		wos, err := wc.Workouts(ctx, rng, 25)
		if err != nil {
			b.deps.Logger.Warn("whoop workouts", "err", err)
		}
		for _, w := range wos {
			if !w.Start.Before(day) && w.Start.Before(dayEnd) {
				payload.Workouts = append(payload.Workouts, w)
			}
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

// pickCycle returns the cycle whose Start falls within [day, dayEnd).
func pickCycle(cs []domain.Cycle, day, dayEnd time.Time) *domain.Cycle {
	var out *domain.Cycle
	for i := range cs {
		c := cs[i]
		if c.Start.Before(day) || !c.Start.Before(dayEnd) {
			continue
		}
		if out == nil || c.Start.After(out.Start) {
			out = &c
		}
	}
	return out
}

// pickRecovery picks the recovery matching the chosen cycle (preferred) or sleep.
func pickRecovery(rs []domain.Recovery, cy *domain.Cycle, sl *domain.Sleep) *domain.Recovery {
	for i := range rs {
		r := rs[i]
		if cy != nil && r.CycleID == cy.ID {
			return &r
		}
	}
	for i := range rs {
		r := rs[i]
		if sl != nil && r.SleepID == sl.ExternalID {
			return &r
		}
	}
	return nil
}
