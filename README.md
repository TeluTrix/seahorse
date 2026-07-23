<p align="center">
  <img src="frontend/public/logo.svg" width="130" alt="seahorse logo" />
</p>

<h1 align="center">seahorse</h1>
<p align="center">A self-hosted, single-binary Jellyfin alternative that never transcodes video.</p>

---

seahorse indexes a movies/TV library against TMDB, serves direct-play video straight off disk, and fixes the one thing that actually breaks browser playback for ripped media: audio codecs. It's a Go backend with an embedded Vue frontend, compiled into one executable — no database server, no reverse proxy, no separate media-server daemon required to get started.

## Why it exists

Most self-hosted media servers assume you'll transcode. seahorse assumes you won't: video streams are served as-is via HTTP range requests (`http.ServeContent`), so playback is instant and CPU-free regardless of resolution — the tradeoff is that whatever container/video codec your files use has to be something the browser's `<video>` element already supports (H.264/H.265 + a browser-native audio codec). The one thing browsers can't recover from — incompatible audio (AC3/DTS/E-AC3/TrueHD, common in Blu-ray/DVD rips) — gets a narrow, targeted fix: on scan, seahorse remuxes just the audio track to AAC into a cached sibling file, leaving the (large) video stream byte-for-byte untouched. `ffmpeg` is used nowhere else — it's a soft dependency purely for that audio fix, subtitle extraction/conversion, and cover-image WebP conversion.

## Features

- **Library scanning** — matches `movies/Title (Year)/` and `tvshows/Show/Season NN/...SxxEyy...` folder conventions against TMDB, with lenient fallback (title-only search) when a folder doesn't have a year. Incremental by default; a Full Rescan wipes and re-fetches everything.
- **Direct-play streaming** — no transcoding, ever. Range/seek support via standard HTTP.
- **Automatic audio-compatibility fix** — probes each file's audio codec with `ffprobe`; incompatible tracks are queued into a backlog and remuxed to AAC only *after* the whole library's metadata has been imported, so heavy remux I/O never blocks you from browsing a freshly-scanned library. Live per-file progress and per-item "audio fix pending/in progress" indicators are surfaced in the UI.
- **Metadata** — poster/backdrop art (cached locally, converted to WebP when `ffmpeg` is available), overview, genres, vote average, runtime, director/creators, and top-billed cast with headshots.
- **Subtitles** — both embedded (extracted via `ffmpeg`) and external subtitle files, served as WebVTT.
- **Watch progress** — per-user resume position, "Continue Watching" auto-advances to the next unwatched episode on a show's page, watched/unwatched badges throughout.
- **Search & browse** — combined movie/TV search with year/genre filters, paginated overview pages, sortable by title or release date.
- **Admin dashboard** — trigger scans, watch live status over SSE (including the remux backlog's progress), manage users.
- **JWT auth** — argon2id password hashing, stateless bearer tokens (also accepted as a query param for `<video>`/`<img>`/`EventSource`, which can't set custom headers).
- **Single binary** — the built frontend is embedded via `go:embed`; deploying is "copy one file, set some env vars, run it."
- **Configurable, not hardcoded** — timeouts, concurrency, pagination, player seek amount, and more are environment-driven (see [Configuration](#configuration)) rather than baked into the binary.

## Tech stack

| Layer | Choice |
|---|---|
| Backend | Go 1.26, [gorilla/mux](https://github.com/gorilla/mux) |
| Database | SQLite via [GORM](https://gorm.io) (`mattn/go-sqlite3`, cgo) |
| Auth | [golang-jwt/v5](https://github.com/golang-jwt/jwt), [argon2id](https://github.com/alexedwards/argon2id) |
| Metadata | [TMDB API v3](https://developer.themoviedb.org/docs) |
| Frontend | Vue 3 (`<script setup>`), Pinia, Vue Router, TypeScript, Vite |
| Styling | Plain CSS, no UI framework |
| Media tooling | `ffmpeg`/`ffprobe` (optional; audio remux, subtitle extraction, cover WebP conversion only) |

No message queue, no cache layer, no microservices — a single process, a single file-based database, a single static frontend bundle. See [Architecture notes](#architecture-notes) for the tradeoffs that come with that.

## Getting started

**Prerequisites:** Go 1.26+, Node 18+, a [TMDB API key](https://www.themoviedb.org/settings/api) (free), and optionally `ffmpeg`/`ffprobe` on `PATH` for audio-fix/subtitle/cover features.

```bash
git clone <this repo>
cd seahorse
cp .env.example .env   # then fill in the values below
make build
./bin/seahorse
```

`make build` builds the frontend (`npm install` + `vite build`) and embeds it into the Go binary via `go:embed`. There's no separate frontend server or static file host to run in production.

Point `SEAHORSE_LIBRARY_PATH` at a directory shaped like:

```
library/
├── movies/
│   └── Inception (2010)/
│       └── Inception.2010.1080p.mkv
└── tvshows/
    └── Breaking Bad/
        └── Season 01/
            └── Breaking.Bad.S01E01.mkv
```

Then open the app, register (the **first account created is automatically an admin**), and trigger a scan from Admin → Library.

### Local development

```bash
make dev
```

Runs the Go backend on `:8585` and the Vite dev server side-by-side (with hot reload and API proxying), stopped together with Ctrl-C.

### Cross-compiling for a Linux server

```bash
make build-linux-amd64
```

Because the SQLite driver needs cgo, a naive `GOOS=linux go build` from macOS silently produces a broken binary (cgo gets disabled with no build-time error). This target instead builds inside a `golang:1.26-alpine` Docker container against musl, producing a fully static binary that runs on any x86_64 Linux distro regardless of its glibc version. Requires Docker.

## Configuration

All configuration is environment variables (or a `.env` file, loaded automatically).

| Variable | Default | Required | Purpose |
|---|---|---|---|
| `SEAHORSE_LIBRARY_PATH` | — | ✅ | Path to your `movies/`/`tvshows/` library root |
| `SEAHORSE_JWT_SECRET` | — | ✅ | Signing secret for auth tokens |
| `SEAHORSE_TMDB_API_KEY` | — | ⚠️ scanning fails without it | TMDB v3 API key |
| `SEAHORSE_PORT` | `8585` | | Listen port |
| `SEAHORSE_LISTEN_ON` | *(all interfaces)* | | Listen address |
| `SEAHORSE_DB` | `sqlite.db` | | SQLite file path |
| `SEAHORSE_REMUX_CONCURRENCY` | `1` | | Parallel audio-remux jobs during a scan |
| `SEAHORSE_DISABLE_REGISTRATION` | `false` | | Close `/register` once an account exists (see below) |
| `SEAHORSE_JWT_TTL_HOURS` | `24` | | Session length before re-login |
| `SEAHORSE_TMDB_TIMEOUT_SECONDS` | `10` | | HTTP timeout for TMDB calls |
| `SEAHORSE_AUDIO_PROBE_TIMEOUT_SECONDS` | `30` | | `ffprobe` timeout (codec/duration checks) |
| `SEAHORSE_AUDIO_REMUX_TIMEOUT_MINUTES` | `60` | | `ffmpeg` timeout per remux job |
| `SEAHORSE_AUDIO_BITRATE` | `192k` | | AAC bitrate for remuxed audio |
| `SEAHORSE_CAST_LIMIT` | `15` | | Max cast members fetched/stored per title |
| `SEAHORSE_MAX_PAGE_SIZE` | `200` | | Hard server-side cap on `?page_size=` |
| `SEAHORSE_DEFAULT_PAGE_SIZE` | `48` | | Default grid page size |
| `SEAHORSE_PLAYER_SEEK_SECONDS` | `15` | | Arrow-key seek amount in the player |
| `SEAHORSE_RESUME_THRESHOLD_SECONDS` | `5` | | Minimum progress before "Resume" is offered |
| `SEAHORSE_PROGRESS_REPORT_INTERVAL_SECONDS` | `10` | | How often playback position is saved |

The four player/pagination-facing values (plus whether registration is currently open) are also served over `GET /api/config`, since the frontend is a prebuilt static bundle and can't read server env vars directly — it fetches them once at load and falls back to the same defaults if that call fails.

`SEAHORSE_DISABLE_REGISTRATION` takes effect only *after* the first account exists — bootstrapping the initial admin (see [Getting started](#getting-started)) always works regardless of this setting, since otherwise a fresh install with it set would have no way to ever create a user. Once that first account is created, `/register` (both the API endpoint and the frontend route/link) is closed to everyone else. Admins can still create accounts manually at any time from Admin → Users, regardless of this setting — it only gates public self-service signup.

## Architecture notes

```
cmd/seahorse/      entrypoint: wires config, DB, auth, TMDB client, scanner, HTTP routes
internal/api/       HTTP handlers + DTOs
internal/auth/      JWT issuing/validation, auth middleware
internal/config/    env var loading
internal/db/        GORM/SQLite connection + migrations
internal/ffmpeg/    shared ffmpeg/ffprobe availability check
internal/models/    GORM models (User, Movie, TVShow, Season, Episode, WatchProgress)
internal/progress/  watch-progress persistence
internal/scanner/    library scanning, TMDB matching, remux backlog, SSE status broadcasting
internal/subtitles/ subtitle discovery/extraction/conversion
internal/tmdb/       TMDB API v3 client
internal/transcode/ ffmpeg-based audio remux + cover WebP conversion
internal/user/       user creation/auth
internal/web/        go:embed wrapper serving the built frontend
frontend/            Vue 3 + TypeScript SPA (built into internal/web/dist)
```

This is a monolith on purpose, and it's worth being explicit about what that means: SQLite is a single-writer embedded file and scan/SSE state lives in the running process's memory, so it doesn't horizontally scale or fail over — there's exactly one instance, and if it restarts mid-scan, in-flight scan/remux state is simply lost (the next scan re-detects and re-queues anything still needed, since the source of truth is always "does the file exist on disk", not any persisted job queue). For a single-user or small-household media server, that's the right tradeoff: one process, one file-based database, nothing to operate. It is **not** designed for multi-replica or high-availability deployment.

## Security notes

- Passwords are hashed with argon2id (via `alexedwards/argon2id`'s defaults).
- The first registered user becomes an admin automatically; every subsequent registration is a regular user. There's no invite/approval flow — if you don't want open registration after that first account, set `SEAHORSE_DISABLE_REGISTRATION=true`.
- Auth is a bearer JWT, also accepted as a `?token=` query param on endpoints consumed by native `<video>`/`<img>`/`EventSource` elements (which can't set an `Authorization` header). Keep `SEAHORSE_JWT_SECRET` private and don't log request URLs containing it.

## License

Apache GPL 2.0 License (Have a look at the LICENSE file.)
