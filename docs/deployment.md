# Production deployment FitLog

Production-каталог: `/home/deploy/fitlog`, SSH `deploy@148.135.208.115`.
Домен остаётся `https://fitlog.chickenkiller.com`.

Стек `fitlog` использует `deployments/compose.prod.yml`: отдельную
PostgreSQL 16, volume `fitlog_postgres`, backend и frontend. Caddy управляется
отдельным репозиторием `kenniele/server-infra` в `/home/deploy/server-infra`; release приложения его не пересоздаёт.
Backend публикует только `127.0.0.1:18082`. В выделенной сети `edge-fitlog` используются aliases
`fitlog-backend` и `fitlog-web`; БД остаётся в закрытой сети.

Операторские команды выполняются в Bash:

```bash
root=/home/deploy/fitlog
release=$(readlink -f "$root/deploy/current")
export FITLOG_ENV_FILE="$root/.env"
export FITLOG_IMAGE=$(cat "$release/app.digest")
export FITLOG_WEB_IMAGE=$(cat "$release/web.digest")
compose=(docker compose --project-name fitlog --project-directory "$root"
  --env-file "$root/.env" -f "$release/compose.prod.yml")
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

По решению владельца от 16 октября 2026 Obsidian исключён из серверного
развёртывания. Vault не монтируется, `OBSIDIAN_ARTICLES_PATH` пуст,
Git-синхронизация статей не запускается.

Для переноса между серверами сначала остановите старый FitLog: он запускает
Telegram polling и WHOOP/FatSecret sync вместе с HTTP. После первого запуска
на новом сервере откат должен сохранить новые записи и обновлённые OAuth tokens.
Два экземпляра с независимыми БД одновременно не запускаются.

Локальные `backups/pre-release-*.dump` содержат личные данные и не заменяют
зашифрованную копию вне VPS. Проверка backup включает restore в отдельную БД.
Старые standalone compose-файлы не используются для общего сервера.

Сеть `edge-fitlog` создаёт server-infra; другие приложения к ней не подключаются.
Frontend обращается к `fitlog-api:8080` через собственную сеть `application`;
этот alias не публикуется в ingress-сети. Caddy использует `fitlog-backend`.
`FITLOG_PROXY_NETWORK` выбирает внешнюю сеть; по умолчанию `edge-fitlog`.
Проект не читает конфигурацию Pulse или IMAP и не управляет их сервисами.

## Ежедневные общие резервные копии

С 2026-10-18 production включён в общий backup Pulse + FitLog + IMAP + server-infra.
Таймер `server-backup.timer` пользователя deploy выполняется каждый день в
**04:00 Europe/Moscow** независимо от SSH-сеанса. Архивы age:
`/home/deploy/server-backups/archives`, хранение 14 дней; автоматическая загрузка
на Mac каждый час в `~/Backups/server-services/archives`, хранение 30 дней.
Mac забирает накопившиеся копии после включения/пробуждения, пока они есть на VPS.
Включены custom dump БД, `.env`, текущий release, ключи приложений и Caddy state.
Ключ расшифровки остаётся на Mac: `~/.ssh/server_backups.agekey`, не на VPS/Git.

Состояние: `systemctl --user list-timers server-backup.timer` и
`~/server-backups/last-success.json` / `last-error.json` на VPS;
`~/Backups/server-services/last-success.json` / `last-error.json` на Mac.
Первое восстановление всех трёх БД в отдельный PostgreSQL проверено 2026-10-18.
Исходники, тесты и полная инструкция восстановления — `server-infra/backups/README.md`.
Общий backup работает отдельно от релизов приложения; предыдущие разовые копии
сохраняются. Архив IMAP содержит данные сервиса, а не содержимое почтовых ящиков.
