# oreilly-cache

A Go service that caches and transforms data from the O'Reilly Learning API. Acts as a faster, shape-controlled frontend over slow upstream endpoints, with periodic background scraping and on-disk storage. Optionally serves a React SPA for browsing the cached content.

## How it works

On startup the scraper performs a full sync of all active, whitelisted publishers and their items, then repeats on a configurable interval. Only publishers with an `is_active: true`, `is_white_listed: true`, and a non-empty `url` field are synced. Subsequent scrapes are cheap: items are sorted by publication date descending, so paging stops as soon as a known item is encountered.

Publishers whose item endpoint returns a 400 are skipped on all future scrapes. A zero-byte `.skip` sentinel is written to disk on first failure and checked before each scrape attempt.

All upstream responses are stored byte-for-byte as `.raw.json` sidecars alongside transformed `.json` files. The HTTP server serves only transformed data; raw files are insurance against schema changes.

Cover images are fetched lazily on first request, written to disk, and served from cache thereafter. Concurrent requests for the same uncached cover are deduplicated. Upstream 404s are negative-cached so the upstream is never hit twice for a missing cover.

After each full scrape cycle, items are grouped by type and written to per-type index files under `items/by-type/`. Delta scrapes merge newly discovered items into the existing index.

## Frontend

The `web/` directory contains a React SPA built with Vite 8, TanStack Router, TanStack Query, TanStack Virtual, and Dexie (IndexedDB). It fetches data from the Go service's JSON API, caches responses locally, and supports browsing by type, sorting, and liking items.

Run both together with:

```sh
make run-with-web
```

This builds the frontend and starts the Go service with `-static-dir ./web/dist`. The SPA is served at `/`; all `/publishers`, `/items/`, and `/covers/` routes continue to serve JSON as before.

## API

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/publishers` | List of all active publishers |
| `GET` | `/publishers/{uuid}` | Single publisher |
| `GET` | `/publishers/{uuid}/items` | All items for a publisher |
| `GET` | `/items/{ourn}` | Single item |
| `GET` | `/items/by-type/{type}` | All items of a given type (e.g. `book`, `video`, `audiobook`, `article`) |
| `GET` | `/covers/{identifier}/{size}` | Cover image (lazy-cached) |
| `GET` | `/healthz` | Liveness check + last scrape summary |

All JSON endpoints support conditional GETs via `If-Modified-Since` / `ETag`.

## Configuration

| Flag | Default | Description |
|------|---------|-------------|
| `-cache-dir` | `./cache` | Root directory for on-disk cache |
| `-listen` | `:8080` | HTTP listen address |
| `-upstream` | `https://learning.oreilly.com` | Upstream base URL |
| `-scrape-interval` | `1h` | How often to re-scrape upstream |
| `-workers` | `5` | Max concurrent publisher item scrapes |
| `-page-size` | `100` | Items per upstream page request |
| `-http-timeout` | `30s` | Per-request upstream HTTP timeout |
| `-shutdown-timeout` | `10s` | Graceful shutdown deadline |
| `-static-dir` | *(empty)* | Serve SPA static files from this directory; if unset, API-only mode |

## Development

```sh
# Go service
make build          # compile linux/amd64 and linux/arm64 binaries
make test           # run all Go tests
make run            # run with defaults (API only)
make clean          # remove compiled binaries

# Frontend
make web-install    # npm install
make web-dev        # start Vite dev server (proxies API to :8080)
make web-build      # production build → web/dist/
make web-check      # biome lint + tsc + vitest

# Combined
make build-all      # build Go binaries + frontend
make run-with-web   # build frontend then run service with -static-dir ./web/dist
```

## On-disk layout

```
cache/
  publishers/
    index.json                 # transformed publisher list
    by-uuid/
      {uuid}.json              # transformed publisher
      {uuid}.raw.json          # raw upstream response
      {uuid}-items.json        # transformed item list for publisher
      {uuid}-items.raw.json    # raw upstream response
      {uuid}-items.skip        # 400 sentinel — publisher skipped on future scrapes
  items/
    by-ourn/
      {ourn}.json              # transformed item
      {ourn}.raw.json          # raw upstream response
    by-type/
      {type}.json              # all items of that type (book, video, audiobook, …)
  covers/
    {identifier}/
      {size}.jpg               # cached cover image
      {size}.404               # negative-cache sentinel
  meta/
    last-scrape.json           # timestamps, counts, errors
```
