package bot

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"

	"fitlog/internal/auth"
)

func (b *Bot) handleStatus(c tele.Context) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var sb strings.Builder
	sb.WriteString("📡 *Status*\n\n")

	// Whoop
	tok, err := b.deps.Tokens.Get(ctx, auth.SourceWhoop)
	switch {
	case errors.Is(err, auth.ErrNotFound):
		sb.WriteString("Whoop: не подключён\n")
	case err != nil:
		fmt.Fprintf(&sb, "Whoop: ошибка чтения токена — %s\n", mdv2Escape(err.Error()))
	default:
		expIn := time.Until(tok.ExpiresAt).Round(time.Minute)
		fmt.Fprintf(&sb, "Whoop: ✓ \\(expires in %s\\)\n", mdv2Escape(expIn.String()))
	}

	// FatSecret
	uid, err := b.deps.FatSecret.ProfileGet(ctx)
	switch {
	case err != nil:
		fmt.Fprintf(&sb, "FatSecret: ✗ %s\n", mdv2Escape(err.Error()))
	case uid != "":
		fmt.Fprintf(&sb, "FatSecret: ✓ user %s\n", mdv2Escape(uid))
	default:
		sb.WriteString("FatSecret: ✓\n")
	}

	return b.reply(c, strings.TrimRight(sb.String(), "\n"))
}
