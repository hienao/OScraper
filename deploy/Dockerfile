FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci --registry=https://registry.npmjs.org --fetch-retries=5 --fetch-retry-maxtimeout=120000
COPY frontend/ ./
RUN npm run build

FROM golang:1.25-alpine AS backend-builder
WORKDIR /app/backend
RUN apk add --no-cache gcc musl-dev
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags="-s -w" -o oscraper .

FROM alpine:3.22
ENV APP_ENV=production TZ=UTC APP_DATA_DIR=/data APP_CACHE_DIR=/cache SERVER_PORT=8080 \
    PUID=1000 PGID=1000 UMASK=002
RUN apk add --no-cache ca-certificates nginx shadow su-exec tzdata && \
    addgroup -S -g 1000 app && adduser -S -D -H -u 1000 -G app app && \
    mkdir -p /data/db /data/work/jobs /cache/logs/nginx /cache/logs/app /cache/tmp/client /cache/tmp/proxy /cache/tmp/fastcgi /cache/tmp/uwsgi /cache/tmp/scgi /media /run/nginx /var/lib/nginx/logs && \
    chown -R app:app /data /cache /media /run/nginx /var/lib/nginx
WORKDIR /app
COPY --from=backend-builder /app/backend/oscraper ./oscraper
COPY --from=frontend-builder /app/frontend/dist /usr/share/nginx/html
COPY deploy/nginx.conf /etc/nginx/nginx.conf
COPY deploy/start.sh /app/start.sh
RUN chmod +x /app/start.sh && chown -R app:app /app /usr/share/nginx/html
USER root
EXPOSE 3113
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD wget -qO- http://127.0.0.1:3113/api/health >/dev/null || exit 1
CMD ["/app/start.sh"]
