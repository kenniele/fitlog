# FitLog Control Center

Next.js App Router frontend for the private FitLog dashboard. The application is dark-first, responsive, and uses the existing Go Control Center API without fixture metrics or simulated integrations.

## Local development

Use Node.js 22 and npm. Start the FitLog API on port `8080`, then run:

```bash
cd web
npm ci
npm run dev
```

Open `http://localhost:3000/dashboard/login`. The login value is the static access secret configured by the operator as `FITLOG_DASHBOARD_TOKEN`; it is not a one-time Telegram token.

In development, Next rewrites `/api/:path*` to the backend origin from `FITLOG_API_INTERNAL_URL`. The default is `http://localhost:8080`, so `/api/v1/...` reaches `http://localhost:8080/api/v1/...`.

```bash
FITLOG_API_INTERNAL_URL=http://127.0.0.1:8080 npm run dev
```

Production requests remain same-origin. Route `/api/*` to the Go API at the reverse proxy, or keep the Next rewrite pointed at an internal API origin. `next.config.ts` enables `output: "standalone"`.

## Verification

```bash
npm run typecheck
npm run lint
npm test
npm run build
npm start
```

The test suite covers missing-value formatting, API error/empty states, date-range URL switching, and Zod form validation.

## Routes

- `/dashboard` — overview and period comparison
- `/dashboard/training` and `/dashboard/training/sessions/[id]`
- `/dashboard/recovery`, `/dashboard/nutrition`, `/dashboard/body`
- `/dashboard/analytics`, `/dashboard/plans`, `/dashboard/imports`, `/dashboard/settings`
- `/dashboard/login`

The shared shell keeps `range`, `from`, `to`, and optional `compare=1` in the URL. Filters are part of TanStack Query keys and API requests. `Cmd/Ctrl+K` opens navigation/actions, and quick-add actions open real API-backed forms.

## API assumptions used by the UI

- Successful JSON responses use `{ "data": ... }`; errors use `{ "error": { "code", "message", "fields"? } }`.
- Lists use `data: { items, total, page, page_size }`; the UI requests no more than the backend maximum of 100 rows.
- Unsafe requests send `X-Fitlog-Request: 1`. Authentication uses `POST`, `GET`, and `DELETE /api/v1/auth/session` and a signed HttpOnly cookie.
- Record edits use `PUT /api/v1/{resource}/{id}`; deletes require an explicit confirmation dialog.
- Overview `today` is a flat `DailyPoint`. `summary` contains training scalars plus recovery/nutrition/body `MetricSummary` objects. `comparison` is expected as `{ range, summary }`.
- Training analytics returns `{ summary, daily, weekly }`; daily volume is `training_volume_kg`, while `estimated_1rm` is meaningful only with an exercise filter.
- Recovery, nutrition, and body analytics return `{ summary, daily }`. Body daily points already carry `weight_7d_average`; sparse data is never extrapolated.
- Import preview and execute are stateless JSON requests with `{ data_type, filename, format, content, mapping, source }`. WHOOP and FatSecret are file adapters only. Stable duplicates are skipped by the backend.
- Source labels display `Connected` only when the API explicitly returns `connected: true`; no sync or disconnect endpoint is assumed.
- Correlations provide `coefficient`, `sample_size`, `period`, `definition`, and `insufficient_sample`. The UI always states that correlation is non-causal.
- Plan exercises send both between-set `rest_seconds` and `rest_after_exercise_seconds`.
- Delete-all uses the exact phrase `DELETE MY DATA`; the delete endpoint clears the auth cookie before the UI navigates to login.

Optional or `null` fields are rendered as an em dash. If the backend adds exercise-history comparison to session detail, it can replace the current honest “not returned by detail API” note.
