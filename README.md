# OScraper

A focused web application for scraping media directories stored in OpenList or mounted locally below `/media`. The application uses a Go/Gin API and a React/TypeScript web interface, following the architecture and interaction conventions of Seshat and porting the safe, manually triggered scraping flow from ostrm.

The current development slice includes:

- one-time administrator bootstrap and JWT session revocation;
- encrypted OpenList token storage;
- OpenList connection CRUD and live `/api/me` validation;
- local media status, safe directory browsing, scanning, rename, and metadata writes below `/media`;
- scrape target CRUD with account-root and target-root path boundaries;
- lazy OpenList directory browsing via `/api/fs/list`;
- persistent asynchronous read-only scans with bounded workers and restart recovery for movie, TV, and anime targets;
- ostrm-compatible title/year, season/episode, anime absolute-episode, and TMDB ID parsing;
- persistent scan runs, media candidates, directory fingerprints, and scan audit events;
- encrypted TMDB configuration with live connectivity testing;
- ostrm-compatible movie/TV search, exact-year preference, and precise TMDB ID lookup;
- immutable 24-hour scrape previews containing the scan fingerprint, match snapshot, complete rename plan, and metadata artifacts;
- fresh OpenList conflict/staleness checks for movie, season, episode, subtitle, image, and NFO paths;
- Kodi/Jellyfin/Emby-compatible movie, TV show, and episode NFO XML with TMDB artwork;
- persistent bounded scrape workers, per-operation checkpoints, idempotent submission, retry, and graceful shutdown recovery;
- overwrite-safe OpenList directory creation, move, rename, metadata upload, and final-path verification;
- searchable scrape history and operation detail, plus searchable and CSV-exportable API/application/audit logs;
- API, application, and administrator audit logs;
- readiness/liveness health reports and scheduled retention cleanup without deleting audit logs;
- a bilingual, responsive light/dark web interface;
- SQLite storage for a simple single-instance deployment;
- a single Docker image served on port `3113`.

The first release workflow is implemented end to end. See the [complete design](docs/design.md) and [operations guide](docs/operations.md).

## Local development

Start the API:

```bash
cd backend
go run .
```

Start the web interface in another terminal:

```bash
cd frontend
npm install
npm run dev
```

Open <http://localhost:5173>. On a new database, sign in with `admin/admin` and immediately replace the one-time administrator credentials.

Development data is written below `backend/runtime`. Override `APP_DATA_DIR`, `APP_CACHE_DIR`, or `SERVER_PORT` when needed.

The UI workflow is: choose OpenList or a local directory → create a constrained scrape target → scan → select/correct the TMDB match → inspect the immutable plan → confirm execution → monitor the persistent job.

## Docker

```bash
cp .env.example .env
# Replace JWT_SECRET and CREDENTIAL_ENCRYPTION_KEY before starting.
docker compose up -d --build
```

Open <http://localhost:3113>. Persistent application data is mounted at `./runtime/data`; logs are mounted at `./runtime/cache`.

Local scrape targets use the host directory configured by `HOST_MEDIA_DIR`, mounted read/write at `/media` in the container. For example:

```env
HOST_MEDIA_DIR=/mnt/nas/media
```

The application user must be able to read the directory for scans and write it for rename or metadata operations. Symbolic links inside local targets are deliberately ignored or rejected.

Before upgrading or testing against real media, follow the backup and small-library gray-release procedure in [docs/operations.md](docs/operations.md).

## Releases

OScraper publishes multi-platform images only for version tags. Beta tags such as `v1.2.0-beta.1` publish the exact version plus `beta`; stable tags such as `v1.2.0` publish the exact version, `1.2`, `1`, and `latest`.

```bash
# Beta
git tag v1.2.0-beta.1
git push origin v1.2.0-beta.1

# Stable
git tag v1.2.0
git push origin v1.2.0
```

Images are available from `ghcr.io/<repository-owner>/oscraper`. See the [release guide](docs/release.md) for validation, approval, rollback, and manual retry rules.

## Verification

```bash
(cd backend && go test ./...)
(cd backend && go test -race ./...)
(cd frontend && npm test && npm run build)
docker compose --env-file .env.example config
```
