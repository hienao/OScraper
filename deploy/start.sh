#!/bin/sh
set -eu

PUID="${PUID:-1000}"
PGID="${PGID:-1000}"
UMASK="${UMASK:-002}"

validate_id() {
  name="$1"
  value="$2"
  case "$value" in
    ''|*[!0-9]*)
      echo "$name must be a positive numeric ID" >&2
      exit 1
      ;;
  esac
  if [ "$value" -le 0 ]; then
    echo "$name must be greater than zero; running OScraper as root is not supported" >&2
    exit 1
  fi
}

validate_id PUID "$PUID"
validate_id PGID "$PGID"
case "$UMASK" in
  [0-7][0-7][0-7]|0[0-7][0-7][0-7]) ;;
  *)
    echo "UMASK must be three octal digits, optionally prefixed by 0 (for example 002 or 0022)" >&2
    exit 1
    ;;
esac
umask "$UMASK"

if [ "$(id -g app)" != "$PGID" ]; then
  groupmod -o -g "$PGID" app
fi
usermod -o -u "$PUID" -g "$PGID" app

prepare_mount() {
  root="$1"
  shift
  ownership_marker="$root/.oscraper-owner"
  mkdir -p "$root" "$@"
  if [ ! -f "$ownership_marker" ] || [ "$(cat "$ownership_marker")" != "$PUID:$PGID" ]; then
    echo "Initializing ownership for $root as $PUID:$PGID"
    chown -R app:app "$root"
    printf '%s\n' "$PUID:$PGID" > "$ownership_marker"
    chown app:app "$ownership_marker"
  else
    chown app:app "$root" "$@"
  fi
}

prepare_mount /data /data/db /data/work /data/work/jobs
prepare_mount /cache /cache/logs /cache/logs/nginx /cache/logs/app \
  /cache/tmp /cache/tmp/client /cache/tmp/proxy /cache/tmp/fastcgi \
  /cache/tmp/uwsgi /cache/tmp/scgi

mkdir -p /run/nginx /var/lib/nginx/logs
chown -R app:app /run/nginx /var/lib/nginx

if ! su-exec app:app test -r /media; then
  echo "Warning: /media is not readable by $PUID:$PGID; OpenList targets remain available" >&2
elif ! su-exec app:app test -w /media; then
  echo "Notice: /media is read-only for $PUID:$PGID; local scans work but rename and metadata writes are disabled" >&2
fi

echo "Starting OScraper as $PUID:$PGID with umask $UMASK"
su-exec app:app nginx -e /cache/logs/nginx/error.log
exec su-exec app:app /app/oscraper
