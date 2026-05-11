package bot

import (
	"context"
	"errors"
	"fmt"
	"time"

	tele "gopkg.in/telebot.v3"

	"fitlog/internal/auth"
)

const startMessage = `Привет\. Список команд:
/connect\_whoop — подключить Whoop
/info \[YYYY\-MM\-DD\] — подробный отчёт за день \(без даты — сегодня\)
/week, /month — агрегаты с дайджестом калорий
/sleep, /recovery, /workouts \[N\] — только эти данные за N дней
/food \[today\|yesterday\] — питание за день
/log weight 105\.2 \| /log note текст \| /log symptom текст
/status — состояние подключений`

func (b *Bot) handleStart(c tele.Context) error {
	return b.reply(c, startMessage)
}

func (b *Bot) handleConnectWhoop(c tele.Context) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Already connected? Check by loading and seeing whether the token is
	// fresh or refreshable.
	if existing, err := b.deps.Tokens.Get(ctx, auth.SourceWhoop); err == nil {
		if existing.RefreshToken != "" || time.Now().Before(existing.ExpiresAt) {
			return b.reply(c, "Whoop уже подключён\\. Используй /status для проверки\\.")
		}
	} else if !errors.Is(err, auth.ErrNotFound) {
		b.deps.Logger.Error("read whoop token", "err", err)
	}

	state, err := b.deps.States.Issue(c.Chat().ID)
	if err != nil {
		b.deps.Logger.Error("issue oauth state", "err", err)
		return c.Send("Не получилось сгенерировать state")
	}
	authURL := b.deps.OAuthConfig.AuthCodeURL(state)

	markup := &tele.ReplyMarkup{}
	btn := markup.URL("Подключить Whoop", authURL)
	markup.Inline(markup.Row(btn))

	return c.Send(fmt.Sprintf("Жми кнопку, чтобы выдать доступ\\. State живёт 10 минут\\."),
		&tele.SendOptions{ParseMode: tele.ModeMarkdownV2}, markup)
}
