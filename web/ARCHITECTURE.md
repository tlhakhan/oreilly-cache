# Web UI Architecture

This document explains how the `web/` SPA is wired end-to-end, from route bootstrapping to API fetch + cache + render.

---

## 1) Stack and responsibilities

The web UI is a **Vite + React + TypeScript** single-page app built around these roles:

- **TanStack Router**: file-based client routing and typed search params.
- **TanStack Query**: request lifecycle (`loading/error/data`) and in-memory query caching.
- **Dexie (IndexedDB)**: durable local persistence for server data + local UI state.
- **TanStack Virtual**: virtual scrolling for the large browse grid.
- **Tailwind CSS**: utility styling.

The app expects the backend API to be reachable at the same origin (`/items/...`, `/publishers/...`, `/covers/...`).

---

## 2) Startup path

### Entry point (`src/main.tsx`)

1. Builds a router from generated `routeTree`.
2. Registers the router type for TanStack Router TS inference.
3. Mounts `<RouterProvider router={router} />` under React `StrictMode`.

### App shell (`src/routes/__root.tsx`)

The root route wraps every page with:

- a shared `QueryClientProvider` (global query cache), and
- the top navigation (`<Nav />`), followed by nested route content (`<Outlet />`).

This means all routes share one query client and one nav shell.

---

## 3) Route model

Routes are file-based through `createFileRoute`:

- `/` in `src/routes/index.tsx`: simple title screen.
- `/browse` in `src/routes/browse.tsx`: primary catalog experience.

`/browse` validates URL search params with Zod:

- `type`: `book | video | audiobook | article` (default `book`)
- `sort`: `date | popular` (default `date`)
- `publisher`: optional string

Invalid params are coerced back to defaults via `.catch(...)`, so malformed URLs remain usable.

---

## 4) Data-fetch path (network + local cache)

`src/api/fetcher.ts` defines a reusable `fetchCached<T>()` pipeline.

### `fetchCached` behavior

For a given endpoint helper:

1. Reads endpoint `last_modified` from Dexie table `syncMeta`.
2. Adds `If-Modified-Since` when value exists.
3. Sends `fetch(url, { headers })`.
4. If request fails (network exception), returns DB rows if available.
5. If server returns `304`, serves cached DB rows.
6. If non-OK status, also falls back to DB rows when possible.
7. On success, validates JSON with Zod schema and writes rows to DB.
8. Stores response `Last-Modified` back in `syncMeta`.

### Endpoint helpers

- `fetchPublishers()` → `/publishers`
- `fetchItemsByType(type)` → `/items/by-type/{type}`
- `fetchPublisherItems(uuid)` → `/publishers/{uuid}/items`

Each helper supplies:

- its own Zod validator,
- table read/write behavior,
- and endpoint-specific `syncMeta` keying.

---

## 5) IndexedDB schema (Dexie)

`src/db/db.ts` creates one DB: `oreilly-cache`.

Tables:

- `items`: server items cache
- `publishers`: server publishers cache
- `likes`: local user likes
- `scrollPos`: per-view virtual scroll restoration
- `syncMeta`: conditional request metadata (`last_modified` by endpoint)

This design separates **server-derived data** from **local UX state** while keeping both offline-resident.

---

## 6) Browse screen internals (`src/routes/browse.tsx`)

The browse page composes several stages:

1. Read validated search params from router (`type`, `sort`, `publisher`).
2. Run `useQuery` keyed by `['items', type]` with `fetchItemsByType(type)`.
3. Locally sort the result set (`popular` or publication date).
4. Optionally filter by `publisher_uuid`.
5. Compute responsive column count from viewport width.
6. Chunk flat items into rows (`Item[][]`) to match the current column count.
7. Virtualize rows with `useWindowVirtualizer` (not every card in DOM).
8. Render each visible row as a CSS grid of `<ItemCard />` cells.

Why row virtualization (instead of item virtualization)?

- Grid layout is easier to reason about with fixed column counts.
- Each virtual row maps naturally to one horizontal grid strip.

---

## 7) Scroll restoration

`src/hooks/useScrollRestoration.ts` persists virtualized list position in `scrollPos`:

- `viewKey` is derived from browse params (type/sort/publisher).
- On mount (after data is ready), it restores index + pixel offset.
- On unmount, it saves first visible virtual row index and offset.

Result: each logical browse view can resume where the user left it.

---

## 8) Component responsibilities

- `Nav.tsx`: top navigation + type links (updates browse search params).
- `ItemCard.tsx`: item metadata UI + like button.
- `CoverImage.tsx`: cover URL rendering with deterministic SVG fallback placeholder.

The search box in `Nav` is currently a UI stub (not wired to filtering yet).

---

## 9) Request-to-paint sequence (quick trace)

For `/browse?type=book&sort=date`:

1. Router validates params.
2. Browse route runs query for `/items/by-type/book`.
3. Fetcher negotiates conditional request + fallback cache rules.
4. Data arrives from network or Dexie.
5. Browse computes rows + virtualizer range.
6. Visible rows render `ItemCard` + `CoverImage`.
7. Likes read/write from Dexie immediately (local-only).
8. Scroll position is restored/saved via `scrollPos`.

---

## 10) Extension points

Common places to extend behavior safely:

- **Add a new browse filter**:
  - extend `/browse` search schema,
  - include value in `viewKey`,
  - update local `rows` transform.
- **Add a new API endpoint cache**:
  - create a new fetch helper using `fetchCached`,
  - define Zod schema in `src/api/types.ts`,
  - choose appropriate Dexie table/indexes.
- **Wire search input**:
  - promote search term into route search params,
  - apply filter before `chunk(...)`,
  - include in `viewKey` for restoration isolation.

---

## 11) Generated file note

`src/routeTree.gen.ts` is generated router metadata consumed by `main.tsx`; edit route source files, not the generated file directly.
