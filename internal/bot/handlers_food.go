package bot

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"

	"fitlog/internal/domain"
)

func (b *Bot) handleFood(c tele.Context) error {
	loc := b.deps.Location
	now := time.Now().In(loc)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	if len(c.Args()) > 0 {
		switch strings.ToLower(strings.TrimSpace(c.Args()[0])) {
		case "yesterday":
			day = day.AddDate(0, 0, -1)
		case "today":
			// already today
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	entries, err := b.deps.FatSecret.FoodEntriesForDay(ctx, day)
	if err != nil {
		return b.reply(c, "Ошибка FatSecret: "+mdv2Escape(err.Error()))
	}

	if len(entries) == 0 {
		return b.reply(c, fmt.Sprintf("🥗 *%s* — пусто", mdv2Escape(fmtDate(day, loc))))
	}

	groups := map[domain.MealKind][]domain.MealEntry{}
	for _, e := range entries {
		groups[e.Meal] = append(groups[e.Meal], e)
	}

	var b2 strings.Builder
	fmt.Fprintf(&b2, "🥗 *Питание %s*\n\n", mdv2Escape(fmtDate(day, loc)))
	order := []domain.MealKind{domain.MealBreakfast, domain.MealLunch, domain.MealDinner, domain.MealOther}
	for _, k := range order {
		es := groups[k]
		if len(es) == 0 {
			continue
		}
		var cal, prot, fat, carbs float64
		names := make([]string, 0, len(es))
		for _, e := range es {
			cal += pFloat(e.Calories)
			prot += pFloat(e.Protein)
			fat += pFloat(e.Fat)
			carbs += pFloat(e.Carbs)
			names = append(names, e.FoodName)
		}
		sort.Strings(names)
		escaped := make([]string, len(names))
		for i, n := range names {
			escaped[i] = mdv2Escape(n)
		}
		fmt.Fprintf(&b2, "%s *%s* — %s / %sб / %sж / %sу\n  %s\n",
			mealEmoji(k), mdv2Escape(mealLabel(k)),
			fmtFloat(cal, 0), fmtFloat(prot, 0), fmtFloat(fat, 0), fmtFloat(carbs, 0),
			strings.Join(escaped, ", "))
	}
	return b.reply(c, strings.TrimRight(b2.String(), "\n"))
}
