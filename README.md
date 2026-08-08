# fitlog

Personal Telegram assistant that pulls fitness data from **Whoop**, nutrition data from **FatSecret**, logs strength workouts, and publishes a random Markdown article from an **Obsidian** folder. The normal UI is a persistent four-button keyboard; slash commands provide a dated health report and a 30-day summary. Access to the bot is restricted by Telegram ID. There are no schedulers or background API polls.

## Stack

- Go 1.25.7+
- PostgreSQL 16 (`pgx/v5`, `goose` for migrations)
- `chi/v5` for the Whoop OAuth callback, healthcheck, and article pages
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

In Telegram, send any text to reveal the keyboard. Press **Здоровье🫀** or **Питание 🥑**; when the provider is not connected yet, the bot responds with an OAuth authorization button. Both providers' delegated tokens are encrypted in PostgreSQL.

## Configuration

| Variable                       | Required | Notes                                                                |
| ------------------------------ | -------- | -------------------------------------------------------------------- |
| `DATABASE_URL`                 | yes      | `postgres://user:pass@host:5432/db?sslmode=disable`                  |
| `FITLOG_TOKEN_ENCRYPTION_KEY`  | yes      | Base64-encoded 32-byte key for AES-GCM OAuth tokens at rest          |
| `WHOOP_CLIENT_ID` / `_SECRET`  | yes      | From Whoop developer console                                         |
| `WHOOP_REDIRECT_URI`           | yes      | Must match the registered callback (`/oauth/whoop/callback`)         |
| `FATSECRET_CONSUMER_KEY/SECRET`| yes      | OAuth 1.0 app credentials                                            |
| `FATSECRET_ACCESS_TOKEN/SECRET`| no       | Legacy fallback; leave blank to connect through Telegram            |
| `NUTRITION_ESTIMATED_TDEE`     | no       | Maintenance kcal/day used by the 14-day deficit analysis             |
| `TELEGRAM_BOT_TOKEN`           | yes      | From @BotFather                                                      |
| `TELEGRAM_ALLOWED_USER_IDS`    | yes      | Comma-separated int64 Telegram user IDs                              |
| `TELEGRAM_WORKOUT_CHANNEL_IDS` | no       | Comma-separated publish-channel IDs; bot needs permission to post    |
| `TELEGRAM_WORKOUT_CHANNEL_ID`  | no       | Legacy single channel ID; combined with the plural setting           |
| `HTTP_ADDR`                    | no       | Default `:8080`. Serves OAuth callbacks + DB-aware `/healthz`.       |
| `TZ_LOCATION`                  | no       | Default `Europe/Moscow`. Used for "today"/"yesterday" boundaries.    |
| `LOG_LEVEL`                    | no       | `debug` / `info` / `warn` / `error`. Default `info`.                 |
| `OBSIDIAN_ARTICLES_PATH`       | no       | Folder containing publishable `.md` files; read recursively          |
| `PUBLIC_BASE_URL`              | no       | Public origin for article links; falls back to Whoop redirect origin |

## Telegram UI

| Action | Result |
| ------ | ------ |
| `Здоровье🫀` | Yesterday's completed Whoop sleep, recovery, strain, and workouts |
| `Питание 🥑` | Yesterday's completed FatSecret meal groups and macros |
| `Статья 📖` | A random Obsidian article as a styled HTML page |
| `Тренировка 🏋️` | One-message workout card, program import, and workout history |
| `/health_summary` | Whoop and FatSecret summary for the previous 30 completed days |
| `/nutrition_analysis` | Average intake, deficit, protein, and calculated weekly weight change for 14 completed days |
| `/info YYYY-MM-DD` | Whoop and FatSecret report for a selected calendar day |
| `/connect_fatsecret` | Authorize or replace the connected FatSecret account |

The provider modules expose the same application pipeline: `Fetch → Transform → Format`. Telegram handlers only select a request and deliver the formatted result.

## Strength workouts

Press **Тренировка 🏋️** to open a single control message. Inline buttons edit that message in place instead of sending a new card after every action. When the card asks for a set, send one of these forms:

```text
12Р 40КГ
12Р -
```

The first form records external weight; the dash records bodyweight. Letter case, decimal comma/dot, and the common hyphen/dash characters are normalized. The temporary input message is deleted after it is processed. Each exercise has **Подход**, **Конец упражнения**, **Заметка**, and **Исправить** actions. Reopening an exercise preserves its existing sets and note, and finishing it continues with the next exercise that is still incomplete. The active card also shows the same exercise's sets from the most recent earlier completed workout. Active state and the control-message ID are stored in PostgreSQL, so a workout can resume after an app restart.

Programs can be imported from a UTF-8 TXT file. A blank line separates programs, the first line of each block is the program name, and the remaining lines are ordered exercises:

```text
Понедельник
Тяга вертикального блока
Жим гантелей лёжа

Вторник
Подтягивания
Отжимания
```

CSV imports accept comma or semicolon delimiters and optional English or Russian headers:

```csv
program,exercise
Понедельник,Тяга вертикального блока
Понедельник,Жим гантелей лёжа
Вторник,Подтягивания
```

The bot shows a preview before saving. An imported program replaces an existing program with the same name; programs absent from the file remain untouched. Running sessions hold an exercise-name snapshot, so replacing a program never rewrites workout history. Completed workouts remain clickable in history, so an unpublished workout can be reopened for correction or publication later. If `TELEGRAM_WORKOUT_CHANNEL_IDS` (or the legacy singular setting) is configured, **Publish** opens a channel picker and sends the formatted result to the selected destination. Telegram bots cannot enumerate all channels automatically, so each available channel must be present in configuration and the bot must be allowed to post there.

## Obsidian articles

Point `OBSIDIAN_ARTICLES_PATH` directly at the folder whose Markdown notes may be public. Nested folders are supported; hidden directories, symlinks, and non-Markdown files are ignored. A note participates in random selection only when its YAML frontmatter explicitly contains `publish: true`; `publish: false`, a missing field, or an invalid value keeps it private. This makes publication opt-in and lets the vault own the filtering policy without a fitlog redeploy. YAML frontmatter is removed, `title` and the first H1 are recognised, and common Markdown/Obsidian constructs receive lightweight HTML formatting.

An `obsidian-git` vault can live next to fitlog and sync independently. For example:

```env
OBSIDIAN_ARTICLES_PATH=/home/fitlog-user/my-vault/Public
PUBLIC_BASE_URL=https://fitlog.example.com
```

Docker Compose mounts this host path at `/vault` read-only. The Caddy route already forwarding the fitlog origin to `app:8080` also serves `/articles/...`; no Telegraph account or additional service is required.

Article URLs contain an opaque AES-GCM token with a fresh random nonce and a protected expiry timestamp, rather than a filename or an encoded vault path. There is no separate setting for this: fitlog derives an isolated article-link key from `FITLOG_TOKEN_ENCRYPTION_KEY`. Tokens cannot be enumerated or modified in practice, expire exactly seven days after issue, and remain usable by anyone who receives the complete URL until then. Rotating `FITLOG_TOKEN_ENCRYPTION_KEY` immediately invalidates all issued article links.

## Troubleshooting

- **Whoop `invalid_client`** — client credentials must go in the request body, not the `Authorization` header. The code uses `oauth2.AuthStyleInParams`; do not change this.
- **Whoop refresh token rotates on every refresh.** It is persisted by `OAuthProvider` → `persistingTokenSource` → `oauth_tokens`. If you copy a refresh token by hand, the next refresh invalidates the old one.
- **FatSecret 401 / signature errors** — all OAuth params must be sent in the form body (not header, not query), and base-string encoding must use full RFC 3986 percent-encoding (`%20` for space, etc.), not `url.QueryEscape`.
- **FatSecret credentials changed or became invalid** — use `/connect_fatsecret`; a successful authorization stored in DB overrides the optional legacy credentials.
- **FatSecret region-locked** — the platform endpoint blocks requests from some regions; you may need a VPN on the host running the bot.
- **`dghubble/oauth1` does not work with FatSecret.** Don't reintroduce it.

## Layout

```
cmd/fitlog/             entrypoint
internal/domain/        provider DTOs (Sleep, Recovery, Cycle, Workout, MealEntry, ...)
internal/training/      manual workout parsing, state, use case, and formatting
internal/whoop/         OAuth2 + REST client + Fetch/Transform/Format reports
internal/fatsecret/     OAuth1 signer + REST client + Fetch/Transform/Format reports
internal/obsidian/      read-only vault scanner + Markdown-to-HTML article publisher
internal/auth/          AES-GCM crypto + token store
internal/storage/       pgx pool + repositories
internal/reportfmt/     shared MarkdownV2 presentation helpers
internal/bot/           thin Telegram delivery adapter and three-button menu
internal/server/        HTTP router for OAuth, healthcheck, and articles
internal/config/        env loading
internal/observability/ slog setup
migrations/             goose SQL migrations
deployments/            Dockerfile + docker-compose.yml
```
