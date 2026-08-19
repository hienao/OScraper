#!/bin/sh
set -eu

mkdir -p /data/db /data/work/jobs /cache/logs/nginx /cache/logs/app /cache/tmp/client /cache/tmp/proxy /cache/tmp/fastcgi /cache/tmp/uwsgi /cache/tmp/scgi /run/nginx
nginx
exec /app/oscraper
