#!/usr/bin/env bash
set -euo pipefail
umask 077

root=$(cd "${1:?usage: release.sh ROOT APP_IMAGE WEB_IMAGE}" && pwd)
app_ref=${2:?set app image}
web_ref=${3:?set web image}
release=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
[[ "$app_ref" =~ ^ghcr\.io/kenniele/fitlog(:[a-f0-9]{40}|@sha256:[a-f0-9]{64})$ ]]
[[ "$web_ref" =~ ^ghcr\.io/kenniele/fitlog-web(:[a-f0-9]{40}|@sha256:[a-f0-9]{64})$ ]]
if [[ "$app_ref" != *@* && "$web_ref" != *@* ]]; then
  if [[ "${app_ref##*:}" != "${web_ref##*:}" ]]; then
    echo 'App and web must use the same commit SHA' >&2
    exit 1
  fi
fi
test -f "$root/.env"
exec 9>"$root/.deploy.lock"
flock -w 120 9

resolve_image() {
  local ref=$1 repository=$2 digest
  docker pull "$ref" >/dev/null
  digest=$(docker image inspect "$ref" --format '{{range .RepoDigests}}{{println .}}{{end}}' | awk -v prefix="$repository@sha256:" 'index($0,prefix)==1 {print; exit}')
  [[ "$digest" =~ @sha256:[a-f0-9]{64}$ ]]
  printf '%s\n' "$digest"
}
export FITLOG_IMAGE FITLOG_WEB_IMAGE FITLOG_ENV_FILE="$root/.env"
FITLOG_IMAGE=$(resolve_image "$app_ref" ghcr.io/kenniele/fitlog)
FITLOG_WEB_IMAGE=$(resolve_image "$web_ref" ghcr.io/kenniele/fitlog-web)
compose=(docker compose --project-name fitlog --project-directory "$root"
  --env-file "$root/.env" -f "$release/docker-compose.shared.yml")
"${compose[@]}" config --quiet
previous=$(readlink -f "$root/deploy/current" 2>/dev/null || true)
replaced=false
rollback() {
  local status=$?
  trap - EXIT
  if (( status != 0 )) && [[ "$replaced" == true && -n "$previous" && -f "$previous/app.digest" && -f "$previous/web.digest" ]]; then
    FITLOG_IMAGE=$(cat "$previous/app.digest")
    FITLOG_WEB_IMAGE=$(cat "$previous/web.digest")
    if [[ "$FITLOG_IMAGE" =~ ^ghcr\.io/kenniele/fitlog@sha256:[a-f0-9]{64}$ && "$FITLOG_WEB_IMAGE" =~ ^ghcr\.io/kenniele/fitlog-web@sha256:[a-f0-9]{64}$ ]]; then
      docker compose --project-name fitlog --project-directory "$root" --env-file "$root/.env" \
        -f "$previous/docker-compose.shared.yml" up -d --no-deps --no-build app web || true
      echo 'Release failed; previous application images requested. Database schema was not rolled back.' >&2
    fi
  fi
  exit "$status"
}
trap rollback EXIT
"${compose[@]}" up -d --wait --wait-timeout 90 postgres
mkdir -p "$root/backups"
"${compose[@]}" exec -T postgres pg_dump -U fitlog -d fitlog -Fc --no-owner > "$root/backups/pre-release-$(date -u +%Y%m%dT%H%M%SZ).dump"
"${compose[@]}" run --rm --no-deps -T app migrate </dev/null
replaced=true
"${compose[@]}" up -d --no-deps --no-build --wait --wait-timeout 120 app web

host_port=$("${compose[@]}" config --format json | python3 -c 'import json,sys;print(json.load(sys.stdin)["services"]["app"]["ports"][0]["published"])')
public_url=$("${compose[@]}" config --format json | python3 -c 'import json,sys;print(json.load(sys.stdin)["services"]["app"]["environment"]["PUBLIC_BASE_URL"])')
host=$(python3 -c 'import sys,urllib.parse;u=urllib.parse.urlparse(sys.argv[1]);assert u.scheme=="https" and u.hostname;print(u.hostname)' "$public_url")
curl -fsS --retry 10 --retry-connrefused --retry-delay 2 --max-time 10 "http://127.0.0.1:$host_port/healthz" >/dev/null
# Resolve to this host's proxy: public DNS alone could still hit the old VPS.
curl -fsS --resolve "$host:443:127.0.0.1" --max-time 15 "$public_url/healthz" >/dev/null
curl -fsS --resolve "$host:443:127.0.0.1" --max-time 15 "$public_url/" >/dev/null
curl -fsS --resolve "$host:443:127.0.0.1" --max-time 15 "$public_url/.well-known/oauth-authorization-server" | python3 -c 'import json,sys;assert json.load(sys.stdin)["issuer"]==sys.argv[1]' "$public_url"
status=$(curl -sS --resolve "$host:443:127.0.0.1" --max-time 15 -o /dev/null -w '%{http_code}' "$public_url/mcp")
[[ "$status" == 401 ]]
curl -fsS --max-time 15 "$public_url/healthz" >/dev/null
printf '%s\n' "$FITLOG_IMAGE" > "$release/app.digest"
printf '%s\n' "$FITLOG_WEB_IMAGE" > "$release/web.digest"
if [[ -n "$previous" && "$previous" != "$release" ]]; then
  ln -sfn "$previous" "$root/deploy/previous"
fi
ln -sfn "$release" "$root/deploy/current"
echo "FitLog release ready: $FITLOG_IMAGE $FITLOG_WEB_IMAGE"
