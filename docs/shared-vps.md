# FitLog на общем сервере Pulse

Production-каталог: `/home/deploy/fitlog`, SSH `deploy@148.135.208.115`.
Домен остаётся `https://fitlog.chickenkiller.com`.

Стек `fitlog` использует `deployments/docker-compose.shared.yml`: отдельную
PostgreSQL 16, volume `fitlog_postgres`, backend и frontend. Caddy управляется
отдельно в `/home/deploy/pulse-proxy`; release приложения его не пересоздаёт.
Backend публикует только `127.0.0.1:18082`. В сети `proxy` используются aliases
`fitlog-backend` и `fitlog-web`; БД остаётся в закрытой сети.

Операторские команды выполняются в Bash:

```bash
root=/home/deploy/fitlog
release=$(readlink -f "$root/deploy/current")
export FITLOG_ENV_FILE="$root/.env"
export FITLOG_IMAGE=$(cat "$release/app.digest")
export FITLOG_WEB_IMAGE=$(cat "$release/web.digest")
compose=(docker compose --project-name fitlog --project-directory "$root"
  --env-file "$root/.env" -f "$release/docker-compose.shared.yml")
"${compose[@]}" ps
curl -fsS http://127.0.0.1:18082/healthz
```

GitHub Actions передаёт bundle в `deploy/releases/<SHA>`. `release.sh` разрешает
app/web одного commit SHA в registry digests, выполняет резервную копию БД,
миграции и проверку приложения через локальный Caddy, затем меняет `current`.
`previous` сохраняет предшествующий успешный bundle. При ошибке после замены
приложения скрипт пытается вернуть его предыдущие образы; схему БД не откатывает.
Несовместимая миграция требует отдельного решения о восстановлении.

CI требует secrets `DEPLOY_HOST`, `DEPLOY_USER`, `DEPLOY_PORT`, `DEPLOY_SSH_KEY`,
`DEPLOY_KNOWN_HOSTS` и variable `DEPLOY_PATH=/home/deploy/fitlog`.
Runtime-секреты хранятся в `.env` с правами `0600`. `FITLOG_POSTGRES_IMAGE`
позволяет закрепить PostgreSQL по digest. Пароль БД — `POSTGRES_PASSWORD`;
URL подключения формирует Compose. Не печатайте `compose config` без `--quiet`.

Obsidian vault расположен в `/home/deploy/obsidian-vault`. Приложение получает
только `Знания/02 Темы` как `/vault:ro`; Git pull запускается отдельно раз в
10 минут. SSH alias `github-obsidian` использует отдельный read-only deploy key.

Для переноса между серверами сначала остановите старый FitLog: он запускает
Telegram polling и WHOOP/FatSecret sync вместе с HTTP. После первого запуска
на новом сервере откат должен сохранить новые записи и обновлённые OAuth tokens.
Два экземпляра с независимыми БД одновременно не запускаются.

Локальные `backups/pre-release-*.dump` содержат личные данные и не заменяют
зашифрованную копию вне VPS. Проверка backup включает restore в отдельную БД.
Старые standalone compose-файлы не используются для общего сервера.
