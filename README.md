# fitlog

Personal Telegram bot that pulls fitness data from **Whoop** and nutrition data from **FatSecret** on demand. The normal UI is a persistent two-button keyboard; slash commands provide a dated report and a 30-day summary. Access is restricted by Telegram ID. There are no schedulers or background API polls.

## Stack

- Go 1.25.7+
- PostgreSQL 16 (`pgx/v5`, `goose` for migrations)
- `chi/v5` (only for the Whoop OAuth callback)
- `telebot.v3` (long-polling)
- `golang.org/x/oauth2` for Whoop; handwritten HMAC-SHA1 signing for FatSecret
- `slog`, `caarlos0/env/v11`, `joho/godotenv`

## Quick start

```bash
cp .env.example .env
# Fill in WHOOP_*, FATSECRET_*, TELEGRAM_*, FITLOG_TOKEN_ENCRYPTION_KEY (32 random bytes, base64)
make docker-up
make migrate-up
make run    # or: docker compose logs -f app
```

In Telegram, send any text to reveal the keyboard. Press **Здоровье🫀**; when Whoop is not connected yet, the bot responds with an OAuth authorization button. FatSecret tokens are read from `.env` directly.

## Configuration

| Variable                       | Required | Notes                                                                |
| ------------------------------ | -------- | -------------------------------------------------------------------- |
| `DATABASE_URL`                 | yes      | `postgres://user:pass@host:5432/db?sslmode=disable`                  |
| `FITLOG_TOKEN_ENCRYPTION_KEY`  | yes      | Base64-encoded 32-byte key for AES-GCM (Whoop tokens at rest)        |
| `WHOOP_CLIENT_ID` / `_SECRET`  | yes      | From Whoop developer console                                         |
| `WHOOP_REDIRECT_URI`           | yes      | Must match the registered callback (`/oauth/whoop/callback`)         |
| `FATSECRET_CONSUMER_KEY/SECRET`| yes      | OAuth 1.0 app credentials                                            |
| `FATSECRET_ACCESS_TOKEN/SECRET`| yes      | OAuth 1.0 user access tokens (never expire)                          |
| `TELEGRAM_BOT_TOKEN`           | yes      | From @BotFather                                                      |
| `TELEGRAM_ALLOWED_USER_IDS`    | yes      | Comma-separated int64 Telegram user IDs                              |
| `HTTP_ADDR`                    | no       | Default `:8080`. Serves Whoop OAuth callback + DB-aware `/healthz`.  |
| `TZ_LOCATION`                  | no       | Default `Europe/Moscow`. Used for "today"/"yesterday" boundaries.    |
| `LOG_LEVEL`                    | no       | `debug` / `info` / `warn` / `error`. Default `info`.                 |

## Telegram UI

| Action | Result |
| ------ | ------ |
| `Здоровье🫀` | Yesterday's completed Whoop sleep, recovery, strain, and workouts |
| `Питание 🥑` | Today's FatSecret meal groups and macros |
| `/health_summary` | Whoop and FatSecret summary for the previous 30 completed days |
| `/info YYYY-MM-DD` | Whoop and FatSecret report for a selected calendar day |

The two provider modules expose the same application pipeline: `Fetch → Transform → Format`. Telegram handlers only select a request and deliver the formatted result.

## Troubleshooting

- **Whoop `invalid_client`** — client credentials must go in the request body, not the `Authorization` header. The code uses `oauth2.AuthStyleInParams`; do not change this.
- **Whoop refresh token rotates on every refresh.** It is persisted by `OAuthProvider` → `persistingTokenSource` → `oauth_tokens`. If you copy a refresh token by hand, the next refresh invalidates the old one.
- **FatSecret 401 / signature errors** — all OAuth params must be sent in the form body (not header, not query), and base-string encoding must use full RFC 3986 percent-encoding (`%20` for space, etc.), not `url.QueryEscape`.
- **FatSecret region-locked** — the platform endpoint blocks requests from some regions; you may need a VPN on the host running the bot.
- **`dghubble/oauth1` does not work with FatSecret.** Don't reintroduce it.

## Layout

```
cmd/fitlog/             entrypoint
internal/domain/        plain DTOs (Sleep, Recovery, Cycle, Workout, MealEntry, ...)
internal/whoop/         OAuth2 + REST client + Fetch/Transform/Format reports
internal/fatsecret/     OAuth1 signer + REST client + Fetch/Transform/Format reports
internal/auth/          AES-GCM crypto + token store
internal/storage/       pgx pool + repositories
internal/reportfmt/     shared MarkdownV2 presentation helpers
internal/bot/           thin Telegram delivery adapter and two-button menu
internal/server/        Whoop OAuth callback HTTP server
internal/config/        env loading
internal/observability/ slog setup
migrations/             goose SQL migrations
deployments/            Dockerfile + docker-compose.yml
```
