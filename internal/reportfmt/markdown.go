// Package reportfmt contains the small set of presentation helpers shared by
// provider-specific report modules. It knows nothing about Telegram handlers
// or API clients; it only produces Telegram MarkdownV2-safe fragments.
package reportfmt

import (
	"fmt"
	"strings"
	"time"
)

const MessageLimit = 4000

// Escape escapes Telegram MarkdownV2 control characters.
func Escape(s string) string {
	const reserved = "_*[]()~`>#+-=|{}.!"
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if strings.ContainsRune(reserved, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Number formats a number without insignificant trailing zeroes.
func Number(v float64, decimals int) string {
	s := fmt.Sprintf("%.*f", decimals, v)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	return Escape(s)
}

func Date(t time.Time, loc *time.Location) string {
	months := []string{"", "января", "февраля", "марта", "апреля", "мая", "июня",
		"июля", "августа", "сентября", "октября", "ноября", "декабря"}
	t = t.In(loc)
	return fmt.Sprintf("%d %s", t.Day(), months[t.Month()])
}

func DateLong(t time.Time, loc *time.Location) string {
	return fmt.Sprintf("%s %d", Date(t, loc), t.In(loc).Year())
}

func Clock(t time.Time, loc *time.Location) string { return t.In(loc).Format("15:04") }

func Duration(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	return fmt.Sprintf("%dч %dм", int(d/time.Hour), int((d%time.Hour)/time.Minute))
}

func PluralDays(n int) string {
	mod100, mod10 := n%100, n%10
	if mod100 >= 11 && mod100 <= 14 {
		return "дней"
	}
	switch mod10 {
	case 1:
		return "день"
	case 2, 3, 4:
		return "дня"
	default:
		return "дней"
	}
}

// Split breaks a formatted report into Telegram-safe chunks, preferring
// paragraph and line boundaries.
func Split(s string) []string {
	if len(s) <= MessageLimit {
		return []string{s}
	}
	var out []string
	for len(s) > MessageLimit {
		cut := strings.LastIndex(s[:MessageLimit+1], "\n\n")
		if cut <= 0 {
			cut = strings.LastIndex(s[:MessageLimit+1], "\n")
		}
		if cut <= 0 {
			cut = MessageLimit
		}
		out = append(out, strings.TrimRight(s[:cut], "\n"))
		s = strings.TrimLeft(s[cut:], "\n")
	}
	if s != "" {
		out = append(out, s)
	}
	return out
}
