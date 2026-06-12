package bot

import (
	tele "gopkg.in/telebot.v3"
)

const startMessage = `*fitlog* — личный фитнес\-бот\.

📊 *Сводки*
/info — подробный отчёт за сегодня
/info 2026\-05\-13 — отчёт за конкретную дату
/week — агрегаты за 7 дней \+ дайджест калорий
/month — то же за 30 дней

🔍 *Срезы*
/sleep \[N\] — сон за N дней \(default 7\)
/recovery \[N\] — recovery с HRV\-трендом
/workouts \[N\] — тренировки
/food \[today\|yesterday\] — питание

📝 *Заметки*
/log weight 105\.2 — вес \(кг\)
/log waist 92\.5 — талия \(см\)
/log bodyfat 22\.4 — процент жира
/log note \<текст\> — произвольная заметка
/log symptom \<текст\> — симптом / самочувствие

⚙️ *Подключения*
/connect\_whoop — OAuth с Whoop
/status — проверка Whoop и FatSecret
/help — эта подсказка`

func (b *Bot) handleStart(c tele.Context) error {
	return b.reply(c, startMessage)
}

func (b *Bot) handleConnectWhoop(c tele.Context) error {
	// We deliberately don't short-circuit when a token already exists. The
	// OAuth callback persists via an upsert, so re-running the flow safely
	// heals a revoked or expired token — whereas a "уже подключён" guard keyed
	// on mere token presence would lock the user out of re-authenticating once
	// the stored refresh token went bad. /status reports the current state.
	state, err := b.deps.States.Issue(c.Chat().ID)
	if err != nil {
		b.deps.Logger.Error("issue oauth state", "err", err)
		return c.Send("Не получилось сгенерировать state")
	}
	authURL := b.deps.OAuthConfig.AuthCodeURL(state)

	markup := &tele.ReplyMarkup{}
	btn := markup.URL("Подключить Whoop", authURL)
	markup.Inline(markup.Row(btn))

	return c.Send("Жми кнопку, чтобы выдать доступ\\. State живёт 10 минут\\.",
		&tele.SendOptions{ParseMode: tele.ModeMarkdownV2}, markup)
}
