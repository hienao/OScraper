# OpenlistScraper

A focused web application for scraping media directories stored in OpenList. The application uses a Go/Gin API and a React/TypeScript web interface, following the architecture and interaction conventions of Seshat.

The current development slice includes:

- one-time administrator bootstrap and JWT session revocation;
- encrypted OpenList token storage;
- OpenList connection CRUD and live `/api/me` validation;
- scrape target CRUD with account-root and target-root path boundaries;
- lazy OpenList directory browsing via `/api/fs/list`;
- read-only recursive media scans for movie, TV, and anime targets;
- ostrm-compatible title/year, season/episode, anime absolute-episode, and TMDB ID parsing;
- persistent scan runs, media candidates, directory fingerprints, and scan audit events;
- encrypted TMDB configuration with live connectivity testing;
- ostrm-compatible movie/TV search, exact-year preference, and precise TMDB ID lookup;
- immutable 24-hour scrape previews containing the scan fingerprint, match snapshot, complete rename plan, and metadata artifacts;
- fresh OpenList conflict/staleness checks for movie, season, episode, subtitle, image, and NFO paths;
- Kodi/Jellyfin/Emby-compatible movie and TV NFO XML previews with escaped TMDB metadata and available image download sources;
- API, application, and administrator audit logs;
- a bilingual, responsive light/dark web interface;
- SQLite by default with optional PostgreSQL support;
- a single Docker image served on port `3113`.

OpenList metadata upload, per-episode TMDB metadata, and recoverable scrape jobs are the next implementation milestones. See the [complete design](docs/design.md).

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

## Docker

```bash
cp .env.example .env
# Replace JWT_SECRET and CREDENTIAL_ENCRYPTION_KEY before starting.
docker compose up -d --build
```

Open <http://localhost:3113>. Persistent application data is mounted at `./runtime/data`; logs are mounted at `./runtime/cache`.

## Verification

```bash
(cd backend && go test ./...)
(cd frontend && npm test && npm run build)
docker compose --env-file .env.example config
```
