package bot

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	tele "gopkg.in/telebot.v3"
)

func TestTrainingExportSendsDocumentToRequester(t *testing.T) {
	for _, format := range []string{"csv", "json"} {
		t.Run(format, func(t *testing.T) {
			var filename, recipient, uploaded string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/bottest/answerCallbackQuery" {
					_, _ = io.WriteString(w, `{"ok":true,"result":true}`)
					return
				}
				if r.URL.Path != "/bottest/sendDocument" {
					t.Errorf("unexpected Telegram method: %s", r.URL.Path)
					http.Error(w, "unexpected method", http.StatusBadRequest)
					return
				}
				if err := r.ParseMultipartForm(1 << 20); err != nil {
					t.Error(err)
					return
				}
				defer r.MultipartForm.RemoveAll()
				file, header, err := r.FormFile("document")
				if err != nil {
					t.Error(err)
					return
				}
				defer file.Close()
				content, err := io.ReadAll(file)
				if err != nil {
					t.Error(err)
					return
				}
				filename, recipient, uploaded = header.Filename, r.FormValue("chat_id"), string(content)
				_, _ = io.WriteString(w, `{"ok":true,"result":{"message_id":456,"chat":{"id":42,"type":"private"}}}`)
			}))
			defer server.Close()
			telegram, err := tele.NewBot(tele.Settings{URL: server.URL, Token: "test", Offline: true, Client: server.Client()})
			if err != nil {
				t.Fatal(err)
			}
			bot := &Bot{b: telegram, deps: Deps{
				ExportTraining: func(ctx context.Context, owner int64, requested string) ([]byte, error) {
					if owner != 42 || requested != format {
						return nil, fmt.Errorf("unexpected export scope: %d %s", owner, requested)
					}
					if _, ok := ctx.Deadline(); !ok {
						return nil, fmt.Errorf("missing export deadline")
					}
					return []byte("тренировка " + format), nil
				},
			}}
			update := tele.Update{Callback: &tele.Callback{
				ID: "export", Sender: &tele.User{ID: 42}, Data: format,
				Message: &tele.Message{ID: 10, Chat: &tele.Chat{ID: -100123, Type: tele.ChatGroup}},
			}}
			if err := bot.handleTrainingExport(telegram.NewContext(update)); err != nil {
				t.Fatal(err)
			}
			if filename != "fitlog-training-all."+format || recipient != "42" || uploaded != "тренировка "+format {
				t.Fatalf("incorrect upload: filename=%s recipient=%s content=%s", filename, recipient, uploaded)
			}
		})
	}
}
