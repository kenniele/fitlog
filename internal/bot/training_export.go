package bot

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"
)

func (b *Bot) handleTrainingExport(c tele.Context) error {
	b.respond(c)
	if c.Sender() == nil || b.deps.ExportTraining == nil {
		return c.Send("Экспорт тренировок пока недоступен.")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	ownerID := c.Sender().ID
	format := strings.TrimSpace(c.Data())
	if format == "" {
		markup := &tele.ReplyMarkup{}
		markup.Inline(
			markup.Row(
				markup.Data("📄 CSV", trainingCallbackExport, "csv"),
				markup.Data("📦 JSON", trainingCallbackExport, "json"),
			),
			markup.Row(markup.Data("‹ В тренировки", trainingCallbackHome)),
		)
		return b.editTrainingCard(ctx, c, ownerID,
			"<b>📤 Экспорт тренировок</b>\n\nВсе твои тренировки за всё время, включая запланированные и незавершённые.\n\n"+
				"CSV — таблица подходов с весом, повторами, RIR и заметками.\n"+
				"JSON — тренировки с вложенными упражнениями, подходами и плановыми значениями.",
			markup,
		)
	}
	if format != "csv" && format != "json" {
		return c.Send("Выбери CSV или JSON в меню экспорта.")
	}
	content, err := b.deps.ExportTraining(ctx, ownerID, format)
	if err != nil {
		return fmt.Errorf("export training: %w", err)
	}
	// Keep the document within the bot upload limit; the web export remains available.
	if len(content) > 49_000_000 {
		return c.Send("Файл слишком большой для Telegram. Скачай экспорт в веб-разделе «Тренировки».")
	}
	mime := "text/csv"
	if format == "json" {
		mime = "application/json"
	}
	document := &tele.Document{
		File:     tele.FromReader(bytes.NewReader(content)),
		FileName: "fitlog-training-all." + format,
		MIME:     mime,
		Caption:  "Тренировки за всё время · " + strings.ToUpper(format),
	}
	// Exports belong to the requester even when the control card is in a group.
	_, err = b.b.Send(c.Sender(), document)
	return err
}
