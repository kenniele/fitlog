# fitlog

Personal Telegram assistant that pulls fitness data from **Whoop**, nutrition data from **FatSecret**, logs strength workouts, and publishes a random Markdown article from an **Obsidian** folder. The normal UI is a persistent four-button keyboard; slash commands provide a dated health report and a 30-day summary. Access to the bot is restricted by Telegram ID. The server refreshes provider-backed dashboard data at startup and then on a configurable interval.

## Stack

- Go 1.25.7+
- PostgreSQL 16 (`pgx/v5`, `goose` for migrations)
- `chi/v5` for the Whoop OAuth callback, healthcheck, and article pages
- Next.js App Router, TypeScript, Tailwind, TanStack Query/Table, Recharts,
  React Hook Form and Zod for the optional Control Center
- `telebot.v3` (long-polling)
- `golang.org/x/oauth2` for Whoop; handwritten HMAC-SHA1 signing for FatSecret
- `slog`, `caarlos0/env/v11`, `joho/godotenv`

## Quick start

```bash
cp .env.example .env
# Fill in WHOOP_*, FATSECRET_*, TELEGRAM_*, FITLOG_TOKEN_ENCRYPTION_KEY (32 random bytes, base64)
make docker-up
make migrate-up
make run
```

In Telegram, send any text to reveal the keyboard. Press **Здоровье🫀** or **Питание 🥑**; when the provider is not connected yet, the bot responds with an OAuth authorization button. Both providers' delegated tokens are encrypted in PostgreSQL.

## FitLog Control Center

The optional Control Center is a single-user web workspace at `/dashboard` for
training, recovery, sleep, nutrition, body measurements, correlations, plans,
and file imports. It reuses the Telegram workout tables instead of maintaining
a second training history. Recovery, sleep, nutrition, and body records are
persisted separately and can be entered manually or imported from CSV/JSON.
Body records support the main values from a gym InBody sheet: body water,
muscle/fat composition, visceral fat, BMR, score, phase angle, and five
segmental lean/fat measurements. The Body page compares each saved scan with
the previous one and keeps missing values as missing rather than zero.

Set a long random `FITLOG_DASHBOARD_TOKEN` to enable the API. The owner defaults
to the first `TELEGRAM_ALLOWED_USER_IDS` entry; set
`FITLOG_DASHBOARD_OWNER_ID` only when a multi-entry allowlist needs another
owner. Leaving the token empty keeps existing bot-only deployments working and
returns `dashboard_disabled` from the protected API.

For local development:

```bash
make migrate-up
npm --prefix web ci
make run
# In a second terminal:
npm --prefix web run dev
```

Open `http://localhost:3000/dashboard`. The Next development server proxies
`/api/*` to `http://localhost:8080`; override that internal destination with
`FITLOG_API_INTERNAL_URL` when needed. Demo data is never loaded implicitly:

```bash
make demo-seed
```

An operator-only FatSecret history backfill is also available. It reads the
already encrypted delegated OAuth token, fetches monthly daily totals, and
defaults to the latest 100 completed local days:

```bash
go run ./cmd/fitlog fatsecret-backfill --days 100 --dry-run
# Persistent API content requires separate FatSecret storage authorization:
go run ./cmd/fitlog fatsecret-backfill --days 100 --storage-authorized
```

Standard FatSecret API terms only permit most diary nutrients to be cached for
24 hours. Do not use the persistent mode without a separate storage entitlement;
import the user's own FatSecret CSV export through Data Imports instead.

WHOOP recovery and sleep can be backfilled independently from the already
encrypted OAuth token. The range includes the current local calendar day,
fetches paginated Cycle/Recovery/Sleep collections, and atomically upserts the
joined health rows without replacing manual/file imports:

```bash
go run ./cmd/fitlog whoop-backfill --days 250 --dry-run
go run ./cmd/fitlog whoop-backfill --days 250
```

The server also runs an automatic WHOOP refresh immediately at startup and
then every `FITLOG_PROVIDER_SYNC_INTERVAL` (default `1h`). Each cycle refreshes
today plus the previous `FITLOG_PROVIDER_SYNC_LOOKBACK_DAYS - 1` local days
(default: a three-day correction window), so late WHOOP scoring is picked up.
Set the interval to `0s` to disable the worker. Manual backfills remain useful
for the initial long history import.

WHOOP workouts are deliberately not converted into strength-training sessions,
because they do not contain FitLog's exercise/set prescription and actuals.

The same automatic cycle can refresh FatSecret, including the current partial
day, but only when `FATSECRET_STORAGE_AUTHORIZED=true`. It remains disabled by
default because standard FatSecret terms restrict persistent diary storage.
A failure or missing OAuth connection for one provider does not stop the other
provider or the bot. WHOOP report requests and the worker share a lock so a
rotating refresh token cannot be consumed concurrently.

The dashboard uses an HttpOnly signed session cookie. Mutating API calls also
require `X-Fitlog-Request: 1`; the web client adds it automatically.

See [Control Center operations and analytics](docs/control-center.md) for date
semantics, formulas, import mapping, security, production topology, and known
limitations.

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
| `FITLOG_DASHBOARD_TOKEN`       | no       | Long random secret; enables authenticated Control Center API         |
| `FITLOG_DASHBOARD_OWNER_ID`    | no       | Owner override; defaults to first allowed Telegram user              |
| `FITLOG_PROVIDER_SYNC_INTERVAL`| no       | Automatic refresh interval; default `1h`, `0s` disables it           |
| `FITLOG_PROVIDER_SYNC_LOOKBACK_DAYS` | no | Correction window including today; default `3`                      |
| `FATSECRET_STORAGE_AUTHORIZED` | no       | Enables automatic persistent FatSecret sync; default `false`         |
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
| `/import_program` | Import a versioned strength program from YAML text or a file |

The provider modules expose the same application pipeline: `Fetch → Transform → Format`. Telegram handlers only select a request and deliver the formatted result.

## Strength workouts

Press **Тренировка 🏋️** to open a single control message. Inline buttons edit that message in place instead of sending a new card after every action. When the card asks for a set, send one of these forms:

```text
12Р 40КГ
12Р -
```

The first form records external weight; the dash records bodyweight. Letter case, decimal comma/dot, and the common hyphen/dash characters are normalized. The temporary input message is deleted after it is processed. Legacy workouts keep the manual **Подход** flow. A structured YAML workout instead shows one next action: the next warm-up set or the repetition buttons for the next working set, followed by nullable RIR buttons. Reaching the planned final working set advances to the next exercise and finishing the final exercise completes the workout automatically.

Structured sessions snapshot the active program revision, recommendation, warm-up plan, rep range, RIR target, weight step, and rest settings. Editing or re-importing a program cannot rewrite an already-started session. Each completed set stores its planned and actual weight/reps/RIR separately, plus `completed_at` and `rest_until`. The recommendation can be overridden for the current session with `weight;sets;reps;RIR;rest`, for example `60;3;8-12;2;180s`; the template and original recommendation remain unchanged.

Reopening an exercise preserves its existing sets and note. The active card also shows the same exercise's sets from the most recent earlier completed workout. Active and completed workouts can be deleted after an explicit confirmation; deleting a published workout removes its channel message first. If Telegram refuses to remove the post, a separate confirmation can delete only the Fitlog record and leave the channel untouched. Active state and the control-message ID are stored in PostgreSQL, so a workout can resume after an app restart.

### Versioned YAML programs

Use `/import_program`, then send a `.yaml` / `.yml` file, plain YAML, or a fenced YAML code block. The import is fully validated and previewed before it creates a new revision. Stable workout IDs survive display-name changes. The active revision supplies new sessions; historical sessions retain their original revision and snapshot.

```yaml
version: 1

program:
  name: "Test Program"
  description: "Three strength days"
  days_per_week: 3

defaults:
  target_rir: 2
  rest_between_sets: 120s
  rest_between_exercises: 180s

workouts:
  - id: bench_day
    name: "Bench Day"
    exercises:
      - exercise: "Жим штанги лёжа"
        warmup:
          - weight: bar
            reps: 10
          - weight: 40kg
            reps: 6
        sets: 3
        reps: 8-12
        starting_weight: 60kg
        weight_step: 2.5kg
        rest: 180s
        after: 180s
        progression: double
```

Version 1 supports deterministic double progression. The working weight increases only when the latest completed workout has exactly the planned number of working sets, every set reaches the top of the rep range, and no known RIR is below the target. Missing RIR does not block progression; warm-up sets never participate. Every recommendation persists a machine-readable reason code and a Russian explanation.

Legacy programs can still be imported from a UTF-8 TXT file. A blank line separates programs, the first line of each block is the program name, and the remaining lines are ordered exercises:

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

The bot shows a preview before saving. An imported program replaces an existing program with the same name; programs absent from the file remain untouched. Running sessions hold an exercise-name snapshot, so replacing a program never rewrites workout history. A saved program can also be opened and one exercise replaced with a new or existing catalog exercise. That editor asks whether to change only the program or also matching completed workouts and their channel publications. Completed workouts remain clickable in history and can be reopened for correction. When an edited published workout is completed again, Fitlog updates the existing channel post instead of creating another one. If `TELEGRAM_WORKOUT_CHANNEL_IDS` (or the legacy singular setting) is configured, **Publish** opens a channel picker and sends the formatted result to the selected destination. Telegram bots cannot enumerate all channels automatically, so each available channel must be present in configuration and the bot must be allowed to post there.

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
internal/controlcenter/ Control Center API, analytics, imports, and demo seed
internal/config/        env loading
internal/observability/ slog setup
migrations/             goose SQL migrations
deployments/            Dockerfile + docker-compose.yml
web/                    Next.js Control Center
docs/                   Operator and developer documentation
```
