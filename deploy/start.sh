#!/bin/sh
set -eu

PUID="${PUID:-1000}"
PGID="${PGID:-1000}"
UMASK="${UMASK:-002}"
MEDIA_GID="${MEDIA_GID:-}"

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
if [ -n "$MEDIA_GID" ]; then
  validate_id MEDIA_GID "$MEDIA_GID"
fi
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

if [ -n "$MEDIA_GID" ] && [ "$MEDIA_GID" != "$PGID" ]; then
  media_group="$(getent group "$MEDIA_GID" | cut -d: -f1 || true)"
  if [ -z "$media_group" ]; then
    media_group="oscraper-media"
    groupadd -o -g "$MEDIA_GID" "$media_group"
  fi
  usermod -a -G "$media_group" app
fi

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

run_groups="$(su-exec app:app id -G)"
if ! su-exec app:app test -r /media; then
  echo "Warning: /media is not readable by UID $PUID with groups $run_groups; set PUID/PGID or MEDIA_GID to match the host permissions" >&2
elif ! su-exec app:app test -w /media; then
  echo "Notice: /media is read-only for UID $PUID with groups $run_groups; local scans work but rename and metadata writes are disabled" >&2
fi

echo "Starting OScraper as UID $PUID with groups $run_groups and umask $UMASK"
su-exec app:app nginx -e /cache/logs/nginx/error.log
exec su-exec app:app /app/oscraper
