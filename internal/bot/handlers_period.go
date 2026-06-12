package bot

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"

	"fitlog/internal/domain"
	"fitlog/internal/fatsecret"
)

// makePeriodHandler renders an N-day aggregate (avg recovery, total strain,
// avg sleep, total/avg macros, top workouts).
func (b *Bot) makePeriodHandler(days int) tele.HandlerFunc {
	return func(c tele.Context) error {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()

		loc := b.deps.Location
		now := time.Now().In(loc)
		// Period covers the last N FULL days, not including today (today is
		// partial and would skew averages). To = midnight of today in user TZ
		// (i.e. end of yesterday), From = N days before that.
		to := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		from := to.AddDate(0, 0, -days)
		rng := domain.TimeRange{From: from, To: to}

		// Accumulators for the digest section, populated as we fetch.
		burnedByDay := map[int]float64{}
		consumedByDay := map[int]float64{}

		var (
			header strings.Builder
			body   strings.Builder
		)
		// Show the inclusive date range so it's obvious we don't include today.
		toLabel := to.AddDate(0, 0, -1) // last full day in the window
		fmt.Fprintf(&header, "📅 *Сводка за %d %s* \\(%s — %s\\)\n\n",
			days, mdv2Escape(pluralDays(days)),
			mdv2Escape(fmtDate(from, loc)), mdv2Escape(fmtDate(toLabel, loc)))

		wc, err := b.loadWhoopClient(ctx)
		switch {
		case errors.Is(err, errWhoopNotConnected):
			body.WriteString("Whoop не подключён \\(/connect\\_whoop\\)\n\n")
		case err != nil:
			b.deps.Logger.Error("load whoop client", "err", err)
		default:
			recs, err := wc.Recoveries(ctx, rng, 25)
			if err != nil {
				b.deps.Logger.Warn("whoop recoveries", "err", err)
			}
			cycles, err := wc.Cycles(ctx, rng, 25)
			if err != nil {
				b.deps.Logger.Warn("whoop cycles", "err", err)
			}
			for _, c := range cycles {
				if c.Kilojoule == 0 {
					continue
				}
				di := fatsecret.ToDateInt(cycleAnchor(c).In(loc))
				burnedByDay[di] = kjToKcal(c.Kilojoule)
			}
			sleeps, err := wc.Sleeps(ctx, rng, 25)
			if err != nil {
				b.deps.Logger.Warn("whoop sleeps", "err", err)
			}
			workouts, err := wc.Workouts(ctx, rng, 25)
			if err != nil {
				b.deps.Logger.Warn("whoop workouts", "err", err)
			}

			// Average only SCORED recoveries — unscored/pending ones come back
			// with Score==0 && HRV==0 and would drag both averages toward zero.
			var sum, hrv float64
			nRec := 0
			for _, r := range recs {
				if r.Score == 0 && r.HRVMilli == 0 {
					continue
				}
				sum += r.Score
				hrv += r.HRVMilli
				nRec++
			}
			if nRec > 0 {
				avg := sum / float64(nRec)
				fmt.Fprintf(&body, "💪 Recovery avg %s%% %s • HRV avg %s ms\n",
					fmtFloat(avg, 0), recoveryEmoji(avg), fmtFloat(hrv/float64(nRec), 0))
			}

			// Likewise skip unscored cycles (Strain==0) so they don't deflate avg.
			var total float64
			nStrain := 0
			for _, c := range cycles {
				if c.ScoreState != "SCORED" {
					continue
				}
				total += c.Strain
				nStrain++
			}
			if nStrain > 0 {
				fmt.Fprintf(&body, "⚡ Total strain %s • avg %s\n",
					fmtFloat(total, 1), fmtFloat(total/float64(nStrain), 1))
			}

			if len(sleeps) > 0 {
				var totalMs int64
				var perfSum float64
				count := 0
				for _, s := range sleeps {
					if s.IsNap {
						continue
					}
					totalMs += s.Stages.LightMs + s.Stages.SWSMs + s.Stages.REMMs
					perfSum += s.SleepPerformancePct
					count++
				}
				if count > 0 {
					avgMs := totalMs / int64(count)
					fmt.Fprintf(&body, "🌙 Sleep avg %s • performance %s%%\n",
						mdv2Escape(fmtDurationHM(avgMs)), fmtFloat(perfSum/float64(count), 0))
				}
			}

			if len(workouts) > 0 {
				body.WriteString("\n🏋 *Топ тренировок*\n")
				top := topWorkouts(workouts, 5)
				for _, w := range top {
					fmt.Fprintf(&body, "  • %s %s · strain %s · %s\n",
						mdv2Escape(fmtDate(w.Start, loc)),
						mdv2Escape(w.SportName),
						fmtFloat(w.Strain, 1),
						mdv2Escape(fmtDurationColon(w.End.Sub(w.Start).Milliseconds())))
				}
			}
		}

		// Macros — sum DailyNutrition from FatSecret over the period. A single
		// food_entries.get_month call returns only ONE calendar month, but a
		// 7- or 30-day window routinely straddles a month boundary, so fetch
		// every month the window touches and merge before filtering — otherwise
		// the earlier month's days silently count as 0 in the digest.
		monthly := map[int]domain.DailyNutrition{}
		for _, m := range monthsSpanned(from, toLabel, loc) {
			rows, err := b.deps.FatSecret.FoodEntriesMonth(ctx, m)
			if err != nil {
				b.deps.Logger.Warn("fatsecret month", "err", err, "month", m.Format("2006-01"))
				continue
			}
			for _, d := range rows {
				monthly[d.DateInt] = d
			}
		}
		fromInt := fatsecret.ToDateInt(from)
		toInt := fatsecret.ToDateInt(to) // exclusive — today not counted
		var cal, prot, fat, carbs float64
		n := 0
		for _, d := range monthly {
			if d.DateInt < fromInt || d.DateInt >= toInt {
				continue
			}
			consumedByDay[d.DateInt] = d.Calories
			cal += d.Calories
			prot += d.Protein
			fat += d.Fat
			carbs += d.Carbs
			n++
		}
		if n > 0 {
			fmt.Fprintf(&body, "\n🥗 *Питание avg* %s / %sб / %sж / %sу\n",
				fmtFloat(cal/float64(n), 0),
				fmtFloat(prot/float64(n), 0),
				fmtFloat(fat/float64(n), 0),
				fmtFloat(carbs/float64(n), 0))
		}

		// Notes for the period — newest first.
		if b.deps.Notes != nil {
			if ns, err := b.deps.Notes.ListBetween(ctx, from, to); err != nil {
				b.deps.Logger.Warn("list notes", "err", err)
			} else if len(ns) > 0 {
				body.WriteString(FormatNotes(ns, loc))
			}
		}

		out := header.String()
		if digest := FormatPeriodDigest(consumedByDay, burnedByDay, days); digest != "" {
			out += digest + "\n"
		}
		out += body.String()
		return b.reply(c, strings.TrimRight(out, "\n"))
	}
}

func pluralDays(n int) string {
	if n == 1 {
		return "день"
	}
	if n >= 2 && n <= 4 {
		return "дня"
	}
	return "дней"
}

// monthsSpanned returns one anchor time (the 1st) in each calendar month the
// inclusive window [from, toLabel] touches, so the caller can fetch every
// FatSecret monthly rollup the period overlaps — a 7- or 30-day window can
// straddle a month boundary.
func monthsSpanned(from, toLabel time.Time, loc *time.Location) []time.Time {
	var out []time.Time
	m := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, loc)
	last := time.Date(toLabel.Year(), toLabel.Month(), 1, 0, 0, 0, 0, loc)
	for !m.After(last) {
		out = append(out, m)
		m = m.AddDate(0, 1, 0)
	}
	return out
}

// topWorkouts returns the n workouts with the highest strain.
func topWorkouts(ws []domain.Workout, n int) []domain.Workout {
	sorted := make([]domain.Workout, len(ws))
	copy(sorted, ws)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].Strain > sorted[j-1].Strain; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	if len(sorted) > n {
		sorted = sorted[:n]
	}
	return sorted
}

// cycleAnchor returns the "anchor moment" for attributing a Whoop cycle to a
// calendar day. cycle.Start is the previous evening's sleep onset, so it
// belongs to the WRONG calendar day for day-level accounting. Better:
//   - If End is set (cycle finished), anchor 1h before End → squarely inside
//     the day the cycle represents.
//   - Otherwise (in-progress cycle), anchor Start + 12h → next morning, also
//     in the right calendar day.
func cycleAnchor(c domain.Cycle) time.Time {
	if c.End != nil {
		return c.End.Add(-time.Hour)
	}
	return c.Start.Add(12 * time.Hour)
}

func (b *Bot) handleSleep(c tele.Context) error {
	days := argInt(c, 7)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	wc, err := b.loadWhoopClient(ctx)
	switch {
	case errors.Is(err, errWhoopNotConnected):
		return b.reply(c, "Whoop не подключён")
	case err != nil:
		b.deps.Logger.Error("load whoop client", "err", err)
		return b.reply(c, "Whoop недоступен: "+mdv2Escape(err.Error()))
	}
	sleeps, err := wc.Sleeps(ctx, domain.Days(time.Now(), days), 25)
	if err != nil {
		return b.reply(c, "Ошибка Whoop: "+mdv2Escape(err.Error()))
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "🌙 *Сон за %d %s*\n\n", days, mdv2Escape(pluralDays(days)))
	for _, s := range sleeps {
		if s.IsNap {
			continue
		}
		total := s.Stages.LightMs + s.Stages.SWSMs + s.Stages.REMMs
		fmt.Fprintf(&sb, "%s · %s · perf %s%% · eff %s%%\n",
			mdv2Escape(fmtDate(s.Start, b.deps.Location)),
			mdv2Escape(fmtDurationHM(total)),
			fmtFloat(s.SleepPerformancePct, 0),
			fmtFloat(s.SleepEfficiencyPct, 0))
	}
	return b.reply(c, strings.TrimRight(sb.String(), "\n"))
}

func (b *Bot) handleRecovery(c tele.Context) error {
	days := argInt(c, 7)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	wc, err := b.loadWhoopClient(ctx)
	switch {
	case errors.Is(err, errWhoopNotConnected):
		return b.reply(c, "Whoop не подключён")
	case err != nil:
		b.deps.Logger.Error("load whoop client", "err", err)
		return b.reply(c, "Whoop недоступен: "+mdv2Escape(err.Error()))
	}
	recs, err := wc.Recoveries(ctx, domain.Days(time.Now(), days), 25)
	if err != nil {
		return b.reply(c, "Ошибка Whoop: "+mdv2Escape(err.Error()))
	}

	// HRV trend: compare today's HRV to the mean of the preceding up-to-7 days.
	// Whoop returns recoveries newest-first, so recs[0] is today and recs[1:]
	// are the preceding days.
	var trend string
	if len(recs) >= 2 {
		var sum float64
		denom := 0
		for i := 1; i < len(recs) && denom < 7; i++ {
			sum += recs[i].HRVMilli
			denom++
		}
		if denom > 0 {
			baseline := sum / float64(denom)
			last := recs[0].HRVMilli
			switch {
			case last > baseline*1.05:
				trend = "↑"
			case last < baseline*0.95:
				trend = "↓"
			default:
				trend = "→"
			}
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "💪 *Recovery за %d %s* %s\n\n", days, mdv2Escape(pluralDays(days)), trend)
	for _, r := range recs {
		fmt.Fprintf(&sb, "%s · %s%% %s · HRV %s · RHR %s\n",
			mdv2Escape(fmtSleepDate(r)),
			fmtFloat(r.Score, 0),
			recoveryEmoji(r.Score),
			fmtFloat(r.HRVMilli, 0),
			fmtFloat(r.RestingHR, 0))
	}
	return b.reply(c, strings.TrimRight(sb.String(), "\n"))
}

// fmtSleepDate falls back to the sleep id since Recovery has no date field
// of its own. We just print "cycle <id>" for brevity.
func fmtSleepDate(r domain.Recovery) string { return fmt.Sprintf("cycle %d", r.CycleID) }

func (b *Bot) handleWorkouts(c tele.Context) error {
	days := argInt(c, 7)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	wc, err := b.loadWhoopClient(ctx)
	switch {
	case errors.Is(err, errWhoopNotConnected):
		return b.reply(c, "Whoop не подключён")
	case err != nil:
		b.deps.Logger.Error("load whoop client", "err", err)
		return b.reply(c, "Whoop недоступен: "+mdv2Escape(err.Error()))
	}
	wos, err := wc.Workouts(ctx, domain.Days(time.Now(), days), 25)
	if err != nil {
		return b.reply(c, "Ошибка Whoop: "+mdv2Escape(err.Error()))
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "🏋 *Тренировки за %d %s*\n\n", days, mdv2Escape(pluralDays(days)))
	for _, w := range wos {
		dur := w.End.Sub(w.Start)
		kcal := w.Kilojoule / 4.184
		fmt.Fprintf(&sb, "%s %s %s · %s · strain %s · HR %s/%s · %s kcal\n",
			mdv2Escape(fmtDate(w.Start, b.deps.Location)),
			workoutEmoji(w.SportName),
			mdv2Escape(w.SportName),
			mdv2Escape(fmtDurationColon(dur.Milliseconds())),
			fmtFloat(w.Strain, 1),
			fmtFloat(w.AvgHR, 0),
			fmtFloat(w.MaxHR, 0),
			fmtFloat(kcal, 0))
	}
	return b.reply(c, strings.TrimRight(sb.String(), "\n"))
}
