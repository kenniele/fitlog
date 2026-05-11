package bot

import (
	"log/slog"

	tele "gopkg.in/telebot.v3"
)

const denyMessage = "Извини, бот не для тебя."

// Allowlist is a set of Telegram user IDs that may interact with the bot.
type Allowlist struct {
	ids    map[int64]struct{}
	logger *slog.Logger
}

// NewAllowlist freezes the given IDs into a set.
func NewAllowlist(ids []int64, logger *slog.Logger) *Allowlist {
	m := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		m[id] = struct{}{}
	}
	return &Allowlist{ids: m, logger: logger}
}

// Allowed reports whether uid is on the list.
func (a *Allowlist) Allowed(uid int64) bool {
	_, ok := a.ids[uid]
	return ok
}

// Middleware returns a telebot middleware that drops non-whitelisted users
// with a fixed reply and stops further handler chain execution.
func (a *Allowlist) Middleware() tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			sender := c.Sender()
			if sender == nil || !a.Allowed(sender.ID) {
				if sender != nil {
					a.logger.Warn("rejecting non-whitelisted user", "user_id", sender.ID)
				}
				return c.Send(denyMessage)
			}
			return next(c)
		}
	}
}
