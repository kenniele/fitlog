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
		rng := domain.Days(now, days)

		// Accumulators for the digest section, populated as we fetch.
		burnedByDay := map[int]float64{}
		consumedByDay := map[int]float64{}

		var (
			header strings.Builder
			body   strings.Builder
		)
		fmt.Fprintf(&header, "📅 *Сводка за %d %s*\n\n", days, mdv2Escape(pluralDays(days)))

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
				if c.Kilojoule > 0 {
					di := fatsecret.ToDateInt(c.Start.In(loc))
					burnedByDay[di] = kjToKcal(c.Kilojoule)
				}
			}
			sleeps, err := wc.Sleeps(ctx, rng, 25)
			if err != nil {
				b.deps.Logger.Warn("whoop sleeps", "err", err)
			}
			workouts, err := wc.Workouts(ctx, rng, 25)
			if err != nil {
				b.deps.Logger.Warn("whoop workouts", "err", err)
			}

			if len(recs) > 0 {
				var sum, hrv float64
				for _, r := range recs {
					sum += r.Score
					hrv += r.HRVMilli
				}
				avg := sum / float64(len(recs))
				fmt.Fprintf(&body, "💪 Recovery avg %s%% %s • HRV avg %s ms\n",
					fmtFloat(avg, 0), recoveryEmoji(avg), fmtFloat(hrv/float64(len(recs)), 0))
			}

			if len(cycles) > 0 {
				var total float64
				for _, c := range cycles {
					total += c.Strain
				}
				fmt.Fprintf(&body, "⚡ Total strain %s • avg %s\n",
					fmtFloat(total, 1), fmtFloat(total/float64(len(cycles)), 1))
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

		// Macros — sum DailyNutrition from FatSecret over the period.
		if monthly, err := b.deps.FatSecret.FoodEntriesMonth(ctx, now); err != nil {
			b.deps.Logger.Warn("fatsecret month", "err", err)
		} else {
			fromInt := fatSecretCutoff(now, days, loc)
			var cal, prot, fat, carbs float64
			n := 0
			for _, d := range monthly {
				if d.DateInt < fromInt {
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

// fatSecretCutoff returns the inclusive lower bound dateInt for last-N-days
// using the calendar date in now's location (delegates to fatsecret.ToDateInt
// so the local-vs-UTC quirk lives in one place).
func fatSecretCutoff(now time.Time, days int, _ *time.Location) int {
	return fatsecret.ToDateInt(now.AddDate(0, 0, -days))
}

func (b *Bot) handleSleep(c tele.Context) error {
	days := argInt(c, 7)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	wc, err := b.loadWhoopClient(ctx)
	if err != nil {
		return b.reply(c, "Whoop не подключён")
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
	if err != nil {
		return b.reply(c, "Whoop не подключён")
	}
	recs, err := wc.Recoveries(ctx, domain.Days(time.Now(), days), 25)
	if err != nil {
		return b.reply(c, "Ошибка Whoop: "+mdv2Escape(err.Error()))
	}

	// HRV trend: compare today's HRV to the mean of the last 7 (excluding today).
	var trend string
	if len(recs) >= 2 {
		var sum float64
		denom := 0
		for i := 0; i < len(recs)-1 && denom < 7; i++ {
			sum += recs[i].HRVMilli
			denom++
		}
		if denom > 0 {
			baseline := sum / float64(denom)
			last := recs[len(recs)-1].HRVMilli
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
	if err != nil {
		return b.reply(c, "Whoop не подключён")
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
