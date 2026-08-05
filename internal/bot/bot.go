package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"golang.org/x/oauth2"
	tele "gopkg.in/telebot.v3"

	"fitlog/internal/fatsecret"
	"fitlog/internal/obsidian"
	"fitlog/internal/reportfmt"
	"fitlog/internal/whoop"
)

const (
	HealthButton    = "Здоровье🫀"
	NutritionButton = "Питание 🥑"
	ArticleButton   = "Статья 📖"
)

// StateIssuer is the small OAuth-state capability required by the delivery
// layer. The concrete in-memory implementation lives in internal/server.
type StateIssuer interface {
	Issue(chatID int64) (string, error)
}

type FatSecretAuthorizer interface {
	Begin(ctx context.Context, chatID int64) (string, error)
}

type Deps struct {
	Whoop         whoop.ReportUseCase
	FatSecret     fatsecret.ReportUseCase
	Articles      obsidian.ReportUseCase
	PublicBaseURL string
	OAuthConfig   *oauth2.Config
	States        StateIssuer
	FatSecretAuth FatSecretAuthorizer
	Location      *time.Location
	Logger        *slog.Logger
}

// Bot is deliberately a thin delivery adapter. Fetching, transformation, and
// formatting live in the provider-specific use cases.
type Bot struct {
	b    *tele.Bot
	deps Deps
	menu *tele.ReplyMarkup
}

func New(token string, allowlist *Allowlist, deps Deps) (*Bot, error) {
	tb, err := tele.NewBot(tele.Settings{
		Token: token,
		// Whoop refresh tokens rotate. Serial processing prevents two reports
		// from refreshing the same token concurrently.
		Synchronous: true,
		Poller:      &tele.LongPoller{Timeout: 10 * time.Second},
		OnError: func(err error, c tele.Context) {
			deps.Logger.Error("telebot handler error", "err", err)
			if c != nil {
				_ = c.Send("Что-то пошло не так. Загляни в логи.")
			}
		},
	})
	if err != nil {
		return nil, fmt.Errorf("telebot init: %w", err)
	}

	menu := mainMenu()

	bot := &Bot{b: tb, deps: deps, menu: menu}
	tb.Use(recoverMiddleware(deps.Logger))
	tb.Use(allowlist.Middleware())
	bot.registerHandlers()

	// SetCommands replaces the old Telegram command menu with the two report commands.
	if err := tb.SetCommands(botCommands()); err != nil {
		deps.Logger.Warn("set telegram commands", "err", err)
	}
	return bot, nil
}

func mainMenu() *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{ResizeKeyboard: true, IsPersistent: true, Placeholder: "Выбери раздел"}
	menu.Reply(
		menu.Row(menu.Text(HealthButton), menu.Text(NutritionButton)),
		menu.Row(menu.Text(ArticleButton)),
	)
	return menu
}

func botCommands() []tele.Command {
	return []tele.Command{
		{Text: "health_summary", Description: "Саммари здоровья и питания за 30 дней"},
		{Text: "nutrition_analysis", Description: "Анализ дефицита за последние 14 дней"},
		{Text: "info", Description: "Здоровье и питание за выбранную дату"},
		{Text: "connect_fatsecret", Description: "Подключить аккаунт FatSecret"},
	}
}

func (b *Bot) registerHandlers() {
	b.b.Handle(HealthButton, b.handleHealth)
	b.b.Handle(NutritionButton, b.handleNutrition)
	b.b.Handle(ArticleButton, b.handleArticle)
	b.b.Handle("/health_summary", b.handleHealthSummary)
	b.b.Handle("/nutrition_analysis", b.handleNutritionAnalysis)
	b.b.Handle("/info", b.handleInfo)
	b.b.Handle("/connect_fatsecret", b.sendFatSecretConnect)
	// Unknown text, including Telegram's conventional /start, only opens the
	// three-button menu and does not create another bot command.
	b.b.Handle(tele.OnText, b.handleMenu)
}

func (b *Bot) handleNutritionAnalysis(c tele.Context) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	report, err := b.executeFatSecret(ctx, fatsecret.NutritionAnalysis(time.Now(), b.deps.Location))
	if err != nil {
		if errors.Is(err, fatsecret.ErrNotConnected) {
			return b.sendFatSecretConnect(c)
		}
		return b.reply(c, "Не удалось рассчитать дефицит: "+reportfmt.Escape(err.Error()))
	}
	return b.reply(c, report)
}

func (b *Bot) Start(ctx context.Context) {
	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		b.b.Stop()
		close(done)
	}()
	b.b.Start()
	<-done
}

func (b *Bot) Stop() { b.b.Stop() }

func (b *Bot) NotifyOAuthSuccess(_ context.Context, chatID int64) {
	if _, err := b.b.Send(&tele.Chat{ID: chatID}, "✓ Whoop подключён. Нажми «Здоровье🫀».", b.menu); err != nil {
		b.deps.Logger.Warn("notify oauth success", "err", err)
	}
}

func (b *Bot) NotifyOAuthFailure(_ context.Context, chatID int64, reason string) {
	if _, err := b.b.Send(&tele.Chat{ID: chatID}, "✗ "+reason, b.menu); err != nil {
		b.deps.Logger.Warn("notify oauth failure", "err", err)
	}
}

func (b *Bot) NotifyFatSecretOAuthSuccess(_ context.Context, chatID int64) {
	if _, err := b.b.Send(&tele.Chat{ID: chatID}, "✓ FatSecret подключён. Нажми «Питание 🥑».", b.menu); err != nil {
		b.deps.Logger.Warn("notify fatsecret oauth success", "err", err)
	}
}

func (b *Bot) NotifyFatSecretOAuthFailure(_ context.Context, chatID int64, reason string) {
	if _, err := b.b.Send(&tele.Chat{ID: chatID}, "✗ "+reason, b.menu); err != nil {
		b.deps.Logger.Warn("notify fatsecret oauth failure", "err", err)
	}
}

func (b *Bot) handleMenu(c tele.Context) error {
	return c.Send("Выбери раздел:", b.menu)
}

func (b *Bot) handleArticle(c tele.Context) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	article, err := b.deps.Articles.Execute(ctx, obsidian.RandomRequest())
	if err != nil {
		b.deps.Logger.Warn("random obsidian article", "err", err)
		switch {
		case errors.Is(err, obsidian.ErrNotConfigured):
			return b.reply(c, "Папка статей Obsidian ещё не настроена\\.")
		case errors.Is(err, obsidian.ErrNoArticles):
			return b.reply(c, "В папке Obsidian нет Markdown\\-статей\\.")
		default:
			return b.reply(c, "Не удалось выбрать статью: "+reportfmt.Escape(err.Error()))
		}
	}
	link := b.deps.PublicBaseURL + "/articles/" + article.ID
	return c.Send("📖 "+article.Title+"\n"+link, b.menu)
}

func (b *Bot) handleHealth(c tele.Context) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	report, err := b.deps.Whoop.Execute(ctx, whoop.Yesterday(time.Now(), b.deps.Location))
	if err != nil {
		if errors.Is(err, whoop.ErrNotConnected) {
			return b.sendWhoopConnect(c)
		}
		b.deps.Logger.Warn("whoop daily report", "err", err)
		return b.reply(c, "Не удалось получить данные Whoop: "+reportfmt.Escape(err.Error()))
	}
	return b.reply(c, report)
}

func (b *Bot) handleInfo(c tele.Context) error {
	args := c.Args()
	if len(args) != 1 {
		return b.reply(c, "Использование: /info ГГГГ\\-ММ\\-ДД")
	}
	day, err := time.ParseInLocation("2006-01-02", args[0], b.deps.Location)
	if err != nil {
		return b.reply(c, "Не понял дату\\. Использование: /info ГГГГ\\-ММ\\-ДД")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	var output string
	whoopReport, whoopErr := b.deps.Whoop.Execute(ctx, whoop.Day(day, b.deps.Location))
	if whoopErr != nil {
		b.deps.Logger.Warn("whoop dated report", "err", whoopErr, "day", day.Format("2006-01-02"))
		if errors.Is(whoopErr, whoop.ErrNotConnected) {
			output = "🫀 *Whoop* — не подключён"
		} else {
			output = "🫀 *Whoop* — ошибка: " + reportfmt.Escape(whoopErr.Error())
		}
	} else {
		output = whoopReport
	}

	fatSecretReport, fatSecretErr := b.executeFatSecret(ctx, fatsecret.Day(day, b.deps.Location))
	if fatSecretErr != nil {
		output += "\n\n🥑 *FatSecret* — ошибка: " + reportfmt.Escape(fatSecretErr.Error())
	} else {
		output += "\n\n" + fatSecretReport
	}
	return b.reply(c, output)
}

func (b *Bot) handleNutrition(c tele.Context) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	report, err := b.executeFatSecret(ctx, fatsecret.Yesterday(time.Now(), b.deps.Location))
	if err != nil {
		if errors.Is(err, fatsecret.ErrNotConnected) {
			return b.sendFatSecretConnect(c)
		}
		return b.reply(c, "Не удалось получить данные FatSecret: "+reportfmt.Escape(err.Error()))
	}
	return b.reply(c, report)
}

func (b *Bot) handleHealthSummary(c tele.Context) error {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	now := time.Now()
	var output string
	whoopReport, whoopErr := b.deps.Whoop.Execute(ctx, whoop.LastCompletedDays(now, b.deps.Location, 30))
	if whoopErr != nil {
		b.deps.Logger.Warn("whoop monthly report", "err", whoopErr)
		if errors.Is(whoopErr, whoop.ErrNotConnected) {
			output = "🫀 *Whoop* — не подключён"
		} else {
			output = "🫀 *Whoop* — ошибка: " + reportfmt.Escape(whoopErr.Error())
		}
	} else {
		output = whoopReport
	}

	fatSecretReport, fatSecretErr := b.executeFatSecret(ctx, fatsecret.LastCompletedDays(now, b.deps.Location, 30))
	if fatSecretErr != nil {
		output += "\n\n🥑 *FatSecret* — ошибка: " + reportfmt.Escape(fatSecretErr.Error())
	} else {
		output += "\n\n" + fatSecretReport
	}
	return b.reply(c, output)
}

func (b *Bot) executeFatSecret(ctx context.Context, req fatsecret.ReportRequest) (string, error) {
	started := time.Now()
	attrs := []any{
		"mode", fatSecretMode(req.Mode),
		"from", req.From.Format("2006-01-02"),
		"to_exclusive", req.To.Format("2006-01-02"),
	}
	b.deps.Logger.Info("fatsecret request started", attrs...)

	fetched, err := b.deps.FatSecret.Fetch(ctx, req)
	attrs = append(attrs, "duration", time.Since(started))
	if err != nil {
		attrs = append(attrs, "err", err)
		b.deps.Logger.Warn("fatsecret request failed", attrs...)
		return "", err
	}

	transformed := b.deps.FatSecret.Transform(fetched)
	attrs = append(attrs,
		"entries_received", len(fetched.Entries),
		"days_received", len(fetched.Days),
		"days_in_period", transformed.LoggedDays,
		"calories", transformed.Calories,
	)
	b.deps.Logger.Info("fatsecret request completed", attrs...)
	return b.deps.FatSecret.Format(transformed), nil
}

func fatSecretMode(mode fatsecret.ReportMode) string {
	switch mode {
	case fatsecret.DailyReport:
		return "daily"
	case fatsecret.SummaryReport:
		return "summary"
	case fatsecret.AnalysisReport:
		return "analysis"
	default:
		return "unknown"
	}
}

func (b *Bot) sendFatSecretConnect(c tele.Context) error {
	if b.deps.FatSecretAuth == nil {
		return b.reply(c, "FatSecret не подключён\\.")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	link, err := b.deps.FatSecretAuth.Begin(ctx, c.Chat().ID)
	if err != nil {
		b.deps.Logger.Error("begin fatsecret oauth", "err", err)
		return b.reply(c, "Не удалось начать подключение FatSecret\\.")
	}
	markup := &tele.ReplyMarkup{}
	button := markup.URL("Подключить FatSecret", link)
	markup.Inline(markup.Row(button))
	return c.Send("FatSecret ещё не подключён\\.", markup)
}

func (b *Bot) sendWhoopConnect(c tele.Context) error {
	state, err := b.deps.States.Issue(c.Chat().ID)
	if err != nil {
		b.deps.Logger.Error("issue oauth state", "err", err)
		return b.reply(c, "Не удалось начать подключение Whoop\\.")
	}
	markup := &tele.ReplyMarkup{}
	button := markup.URL("Подключить Whoop", b.deps.OAuthConfig.AuthCodeURL(state))
	markup.Inline(markup.Row(button))
	return c.Send("Whoop ещё не подключён\\. Авторизация действует 10 минут\\.",
		&tele.SendOptions{ParseMode: tele.ModeMarkdownV2, DisableWebPagePreview: true}, markup)
}

func (b *Bot) reply(c tele.Context, report string) error {
	for i, chunk := range reportfmt.Split(report) {
		options := &tele.SendOptions{ParseMode: tele.ModeMarkdownV2, DisableWebPagePreview: true}
		if i == 0 {
			if err := c.Send(chunk, options, b.menu); err != nil {
				return err
			}
			continue
		}
		if err := c.Send(chunk, options); err != nil {
			return err
		}
	}
	return nil
}

func recoverMiddleware(logger *slog.Logger) tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) (err error) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.Error("handler panic", "panic", recovered, "stack", string(debug.Stack()))
					err = fmt.Errorf("panic: %v", recovered)
				}
			}()
			return next(c)
		}
	}
}
