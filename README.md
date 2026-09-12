# Multi-Window Media Sequencer — Backend (Go)

Golang HTTP API backing the Multi-Window Media Sequencer assignment.
PostgreSQL-backed, supports dynamic playlist management, sync broadcast via
Server-Sent Events, and media upload.

Frontend repo: see the companion React project (deployed separately) —
[ https://multi-window-media-sequencer-fronte.vercel.app/].

## Postgres setup

The backend expects a database that **already exists** — it creates tables,
not the database itself.

```
DATABASE_URL=postgresql://postgres:%23ma@127.0.0.1:5432/eva_bharat?sslmode=disable
```

`%23` is a URL-encoded `#` (required since the password contains one).
`sslmode=disable` is for local Postgres only — a managed/deployed Postgres
(Render, Railway, etc.) usually needs `sslmode=require` instead; use whatever
connection string your hosting provider gives you.

Create the database once, before first run:

```bash
psql -U postgres -h 127.0.0.1 -c "CREATE DATABASE eva_bharat;"
```

## Local setup

```bash
go mod tidy      # downloads github.com/lib/pq and github.com/google/uuid
go run .
```

Reads `.env` automatically (a small built-in loader, no external dependency).
On first run it seeds 3 example windows with a short mixed playlist each.

Env vars (see `.env`):

- `DATABASE_URL` — Postgres connection string (required)
- `PORT` — server port (default `8080`; hosting platforms usually set this for you)
- `UPLOAD_DIR` — where uploaded media files are stored on disk (default `uploads`)
- `CYCLE_SECONDS` — the "5 hour" cycle length in seconds (default `18000` = 5h).
  **Set this to something small while demoing**, e.g. `CYCLE_SECONDS=30 go run .`,
  so the loop-and-restart behavior is visible without waiting hours.

## How sync behavior works

Each window's playback position is computed **client-side** (by the
frontend) from `(now - window.created_at_unix) mod totalPlaylistDuration` —
this backend never tracks "what's currently playing." When `POST /sync`
fires, the backend broadcasts a `SyncState` event over SSE (`GET /events`)
to every connected browser; the frontend overrides its display for the sync
duration without touching its own elapsed-time clock, so playback resumes
exactly where it should be once sync ends.

## API documentation

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/windows` | List every window with its full playlist |
| `GET` | `/windows/{id}` | Get a single window |
| `POST` | `/windows/{id}/items` | Add a media item: `{type, url, duration_seconds}` |
| `PUT` | `/items/{id}` | Edit an existing item's type, url, or duration |
| `DELETE` | `/items/{id}` | Remove an item from its window's playlist |
| `POST` | `/upload` | Upload a file (`multipart/form-data`, field `file`) → returns `{"url": "/uploads/<name>"}` |
| `GET` | `/uploads/{filename}` | Serves an uploaded file (static) |
| `POST` | `/sync` | Trigger sync: `{item_id, duration_seconds}` |
| `GET` | `/sync/status` | Current sync state (for polling instead of SSE) |
| `GET` | `/events` | SSE stream of sync state changes |

`type` is one of `image`, `video`, or `blank` (blank items omit `url`).

### Paste-URL-or-upload

Both `POST /windows/{id}/items` and `PUT /items/{id}` take a plain `url`
string. Either paste an external URL directly, or `POST /upload` a file
first and use the returned `/uploads/<uuid>.jpg` path as that `url` — the
rest of the system treats both identically.

## Assumptions

- **"5-hour cycle" is treated as an infinite loop of the same playlist** —
  restarting an identical list every 5 hours is observably identical to
  looping it continuously.
- **Blank is only shown when explicitly added as a playlist item.**
- **Sync uses a fixed duration passed by the caller** (default 10s), not
  tied to the synced item's own natural duration.
- **Deleting/editing an item does not shift `order_index`** of other items.
- **Uploaded files are stored on local disk**, not object storage — fine
  for this assignment, but a real limitation on ephemeral hosting (most
  free tiers wipe local disk on redeploy; a production version would use S3
  or similar).
- **Migrations**: schema is created via `CREATE TABLE IF NOT EXISTS` at
  startup, which handles first-run bootstrap but is not a versioned
  migration system — a later column change wouldn't be applied
  automatically. Acceptable for this assignment's fixed schema.

## Deployment (Render)
 url:-https://multi-window-media-sequencer-backend.onrender.com
1. Connect this repo as a Web Service.
2. Use a **managed Postgres add-on** from the same host — set `DATABASE_URL`
   to that instance's connection string as an environment variable (don't
   commit `.env` — it's git-ignored).
3. Build command: `go build -o app .` — Start command: `./app`
4. Set `CYCLE_SECONDS` and `UPLOAD_DIR` as environment variables too.
5. **Known limitation**: free-tier hosts typically have an ephemeral
   filesystem, so uploaded files won't survive a redeploy.
