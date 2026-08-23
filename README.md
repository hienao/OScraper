# OScraper

English · [简体中文](README.zh-CN.md)

OScraper is a self-hosted media scraping workspace for OpenList and local media directories. It scans movies, TV shows, and anime on demand, matches them with TMDB, shows every planned change before execution, and then safely organizes media files and writes compatible metadata.

OScraper is designed for people who want a controlled, auditable alternative to unattended bulk renaming. Nothing is changed during scanning or preview, and an execution stops instead of overwriting conflicting media.

## Highlights

- **OpenList and local storage** — work with a constrained OpenList directory or a host directory mounted at `/media`.
- **Movies, TV shows, and anime** — parse titles, years, seasons, episodes, anime absolute episode numbers, and `{tmdbid-N}` markers.
- **TMDB-assisted matching** — search by title and year or select a precise TMDB ID before scraping.
- **Preview before execution** — inspect directory creation, rename operations, generated NFO files, posters, backdrops, and episode artwork.
- **Safe writes** — verify fresh directory fingerprints, reject stale previews, and never overwrite existing media paths.
- **Recoverable jobs** — run scraping through persistent bounded workers with per-operation checkpoints and retry support.
- **Media-server metadata** — generate movie, TV show, and episode NFO XML compatible with Kodi, Jellyfin, and Emby.
- **Operational visibility** — search job history, API logs, application logs, and administrator audit logs; export logs as CSV.
- **User-friendly interface** — responsive light/dark UI with English and Simplified Chinese.
- **Simple self-hosting** — one multi-platform image for `linux/amd64` and `linux/arm64`, backed by SQLite.

## How it works

```text
Connect storage → Create a constrained target → Scan candidates
       → Confirm the TMDB match → Review the immutable plan
       → Execute and monitor the persistent job
```

Scanning and preview are read-only. Media renames and metadata writes begin only after you explicitly confirm a valid plan.

## Requirements

- Docker Engine with Docker Compose v2
- A TMDB v3 API key
- One of the following media sources:
  - an OpenList account and token with access to the selected media directory; or
  - a local host directory that Docker can mount into the container

For OpenList scraping, use a dedicated account limited to the media-library root whenever possible. Renaming and metadata writes require list, directory creation, move, rename, and upload permissions.

## Deploy with Docker Compose

OScraper images are published only to GitHub Container Registry:

```text
ghcr.io/hienao/oscraper
```

The example below follows the newest Beta for easy evaluation. For a controlled deployment, replace `beta` with an exact version from the GHCR package so upgrades and rollbacks stay deliberate.

### 1. Prepare the deployment directory

```bash
mkdir -p oscraper/media
cd oscraper
```

Create `compose.yaml`:

```yaml
services:
  oscraper:
    image: ${OSCRAPER_IMAGE:-ghcr.io/hienao/oscraper:beta}
    container_name: oscraper
    ports:
      - "3113:3113"
    environment:
      APP_ENV: production
      JWT_SECRET: ${JWT_SECRET:?JWT_SECRET is required}
      CREDENTIAL_ENCRYPTION_KEY: ${CREDENTIAL_ENCRYPTION_KEY:?CREDENTIAL_ENCRYPTION_KEY is required}
      TZ: ${TZ:-UTC}
      SCRAPE_WORKERS: ${SCRAPE_WORKERS:-2}
      SCAN_WORKERS: ${SCAN_WORKERS:-1}
    volumes:
      - oscraper-data:/data
      - oscraper-cache:/cache
      - ${HOST_MEDIA_DIR:-./media}:/media
    restart: unless-stopped

volumes:
  oscraper-data:
  oscraper-cache:
```

### 2. Create the environment file

Generate two independent secrets:

```bash
openssl rand -hex 32
openssl rand -base64 32
```

Create `.env` and place the first value in `JWT_SECRET` and the second value in `CREDENTIAL_ENCRYPTION_KEY`:

```dotenv
OSCRAPER_IMAGE=ghcr.io/hienao/oscraper:beta
JWT_SECRET=replace-with-the-first-generated-value
CREDENTIAL_ENCRYPTION_KEY=replace-with-the-second-generated-value
TZ=Asia/Shanghai

# Host directory exposed to local scrape targets as /media.
HOST_MEDIA_DIR=./media

# Optional bounded worker counts; valid range: 1-4.
SCRAPE_WORKERS=2
SCAN_WORKERS=1
```

Keep `CREDENTIAL_ENCRYPTION_KEY` safe and unchanged. Existing OpenList tokens and TMDB keys cannot be decrypted if this key is lost or replaced.

If the GHCR package is private, authenticate before pulling it:

```bash
echo "$GHCR_TOKEN" | docker login ghcr.io -u YOUR_GITHUB_USERNAME --password-stdin
```

The token needs `read:packages` permission.

### 3. Start OScraper

```bash
docker compose pull
docker compose up -d
docker compose ps
```

Open <http://localhost:3113>. A healthy deployment also responds at:

```text
http://localhost:3113/api/health/live
http://localhost:3113/api/health/ready
```

The first login uses the one-time credentials `admin/admin`. OScraper immediately requires you to replace both the administrator username and password before continuing.

## First-time setup

1. Open **Settings**, enter your TMDB v3 API key, choose the metadata language and region, save, and run the connection test.
2. For OpenList, open **Connections**, add the server URL and token, define the account root, and verify the connection.
3. Open **Targets** and create a movie, TV, or anime target:
   - choose an OpenList connection and a directory below its allowed root; or
   - choose **Local** and a directory below `/media`.
4. Enable media renaming only if OScraper should organize existing directories and filenames. Metadata can still be generated when renaming is disabled.
5. Run a scan, review the detected candidates, correct the TMDB match if necessary, and inspect the complete preview.
6. Confirm execution only when every rename and generated artifact is correct, then monitor progress under **Jobs**.

For a first real run, use a small, recoverable copy of your library rather than the main media collection.

## Storage and permissions

| Mount | Purpose | Persistence |
| --- | --- | --- |
| `/data` | SQLite database, migrations, and persistent job workspace | Required; stored in `oscraper-data` |
| `/cache` | API and application logs plus temporary cache | Recommended; stored in `oscraper-cache` |
| `/media` | Host media exposed to local scrape targets | Required only for local targets |

The container runs as a non-root user. The directory configured by `HOST_MEDIA_DIR` must be readable for scans and writable for renaming or metadata generation. Symbolic links inside local targets are intentionally ignored or rejected.

OScraper supports a single application instance. Do not attach the same `/data` volume to multiple running containers because the application uses SQLite and local job checkpoints.

## Updating

Review available tags in the [GHCR package](https://github.com/hienao/OScraper/pkgs/container/oscraper), back up the persistent data volume, and change `OSCRAPER_IMAGE` to the desired exact version. Then run:

```bash
docker compose pull
docker compose up -d
docker compose ps
```

The `beta` tag follows the newest Beta whenever you pull again. Stable releases also publish `latest`, but controlled and production deployments should pin an exact version.

## Troubleshooting

View container logs:

```bash
docker compose logs --tail=200 oscraper
```

Common checks:

- **The container does not start** — verify that `JWT_SECRET` has at least 32 characters and that `CREDENTIAL_ENCRYPTION_KEY` is a raw 32-byte value or a Base64-encoded 32-byte value.
- **Local media is unavailable** — verify `HOST_MEDIA_DIR`, Docker file sharing, and host read/write permissions.
- **OpenList testing fails** — verify the server URL, token, account-root path, and required permissions.
- **TMDB testing fails** — verify the API key, metadata region/language, proxy, and outbound network access.
- **A preview becomes stale** — the source directory changed after scanning; scan it again and create a new preview.
- **A job reports a destination conflict** — inspect the existing destination and create a new plan; OScraper will not overwrite it.
- **A job was interrupted** — inspect storage state and retry it from the saved checkpoint on the **Jobs** page.

For backup, recovery, permissions, gray release, and failure handling, see the [operations guide](docs/operations.md).

## Current limitations

- OScraper is an on-demand scraper; scheduled scans are not included in the current release.
- It organizes existing media and writes metadata; it does not generate STRM files.
- It does not automatically refresh Kodi, Jellyfin, or Emby libraries after a job.
- Local targets cannot follow symbolic links or move media across filesystems.
- Multi-instance deployments sharing one database are not supported.

## Documentation

- [Operations and recovery](docs/operations.md)
- [Release channels and image tags](docs/release.md)
- [Architecture and design](docs/design.md)
