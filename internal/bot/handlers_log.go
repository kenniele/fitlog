package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"

	"fitlog/internal/domain"
)

const logUsage = "Использование:\n" +
	"  /log weight 105.2 — записать вес (кг)\n" +
	"  /log note <текст> — произвольная заметка\n" +
	"  /log symptom <текст> — симптом / самочувствие"

// handleLog stores a free-form note. Recognised kinds:
//
//	/log weight 105.2   → kind=weight, value=105.2
//	/log note <text>    → kind=note, body=text
//	/log symptom <text> → kind=symptom, body=text
//
// Without a recognised kind, everything after /log is stored as a generic note.
func (b *Bot) handleLog(c tele.Context) error {
	args := c.Args()
	if len(args) == 0 {
		return b.reply(c, mdv2Escape(logUsage))
	}

	kind := strings.ToLower(strings.TrimSpace(args[0]))
	tail := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(c.Message().Payload), args[0]))

	note := domain.Note{Ts: time.Now()}

	switch kind {
	case domain.NoteKindWeight:
		if tail == "" {
			return b.reply(c, mdv2Escape("Формат: /log weight <число>"))
		}
		// FatSecret-style "1,5" → "1.5".
		tail = strings.ReplaceAll(tail, ",", ".")
		v, err := strconv.ParseFloat(strings.Fields(tail)[0], 64)
		if err != nil {
			return b.reply(c, mdv2Escape("Не понял число веса: "+tail))
		}
		note.Kind = domain.NoteKindWeight
		note.Value = &v
		// Preserve any trailing comment, e.g. "/log weight 105.2 утром натощак".
		if rest := strings.TrimSpace(strings.TrimPrefix(tail, strings.Fields(tail)[0])); rest != "" {
			note.Body = rest
		}

	case domain.NoteKindNote, domain.NoteKindSymptom:
		if tail == "" {
			return b.reply(c, mdv2Escape("Формат: /log "+kind+" <текст>"))
		}
		note.Kind = kind
		note.Body = tail

	default:
		// Unknown kind → treat full message (incl. first arg) as a generic note.
		note.Kind = domain.NoteKindNote
		note.Body = strings.TrimSpace(c.Message().Payload)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := b.deps.Notes.Insert(ctx, note); err != nil {
		b.deps.Logger.Error("insert note", "err", err)
		return b.reply(c, mdv2Escape("Не получилось сохранить заметку"))
	}

	return b.reply(c, mdv2Escape(formatNoteAck(note)))
}

func formatNoteAck(n domain.Note) string {
	switch n.Kind {
	case domain.NoteKindWeight:
		s := fmt.Sprintf("✓ Записал вес: %.2f кг", *n.Value)
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
		if n.Body != "" {
			s += " — " + n.Body
		}
		return s
	case domain.NoteKindSymptom:
		return "✓ Записал симптом: " + n.Body
	default:
		return "✓ Записал: " + n.Body
	}
}
