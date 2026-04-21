import { db } from '../db/db';
import {
  type Item,
  ItemListSchema,
  type Publisher,
  PublisherIndexSchema,
} from './types';

export interface FetchConfig<T> {
  url: string;
  endpoint: string;
  validate: (body: unknown) => T[];
  getFromDb: () => Promise<T[]>;
  putToDb: (rows: T[]) => Promise<void>;
  getLastModified: () => Promise<string | undefined>;
  setLastModified: (value: string) => Promise<void>;
}

export async function fetchCached<T>(config: FetchConfig<T>): Promise<T[]> {
  const {
    url,
    validate,
    getFromDb,
    putToDb,
    getLastModified,
    setLastModified,
  } = config;

  const headers: Record<string, string> = {};
  const lastMod = await getLastModified();
  if (lastMod) {
    headers['If-Modified-Since'] = lastMod;
  }

  let response: Response;
  try {
    response = await fetch(url, { headers });
  } catch (err) {
    const cached = await getFromDb();
    if (cached.length > 0) return cached;
    throw err;
  }

  if (response.status === 304) {
    const cached = await getFromDb();
    if (cached.length > 0) return cached;
    // syncMeta has a Last-Modified but the DB is empty (e.g. storage was
    // cleared or data predates an index change). Re-fetch unconditionally.
    response = await fetch(url);
  }

  if (!response.ok) {
    const cached = await getFromDb();
    if (cached.length > 0) return cached;
    throw new Error(`HTTP ${response.status}: ${url}`);
  }

  const body: unknown = await response.json();
  const rows = validate(body);

  // Storage can fail on mobile Chrome (QuotaExceededError). Keep it non-fatal
  // so the freshly fetched rows are still returned. Skip setLastModified on
  // failure so the next request re-fetches instead of relying on an empty cache.
  try {
    await putToDb(rows);
    const newLastMod = response.headers.get('Last-Modified');
    if (newLastMod) {
      await setLastModified(newLastMod);
    }
  } catch (e) {
    console.warn('fetchCached: failed to write to IndexedDB, skipping cache', e);
  }

  return rows;
}

// URL-encode an ourn for use in URL path segments (orns contain colons)
export function encodeOurn(ourn: string): string {
  return encodeURIComponent(ourn);
}

// --- Wired endpoint helpers ---

export function fetchPublishers(): Promise<Publisher[]> {
  const endpoint = '/api/publishers';
  return fetchCached({
    url: endpoint,
    endpoint,
    validate: (body) => PublisherIndexSchema.parse(body).publishers,
    getFromDb: () => db.publishers.toArray(),
    putToDb: (rows) => db.publishers.bulkPut(rows).then(() => {}),
    getLastModified: () =>
      db.syncMeta.get(endpoint).then((m) => m?.last_modified),
    setLastModified: (v) =>
      db.syncMeta.put({ endpoint, last_modified: v }).then(() => {}),
  });
}

export function fetchItemsByType(type: string): Promise<Item[]> {
  const endpoint = `/api/items/by-type/${type}`;
  return fetchCached({
    url: endpoint,
    endpoint,
    validate: (body) => ItemListSchema.parse(body).items,
    getFromDb: () => db.items.where('type').equals(type).toArray(),
    putToDb: (rows) => db.items.bulkPut(rows).then(() => {}),
    getLastModified: () =>
      db.syncMeta.get(endpoint).then((m) => m?.last_modified),
    setLastModified: (v) =>
      db.syncMeta.put({ endpoint, last_modified: v }).then(() => {}),
  });
}

export function fetchPublisherItems(publisherUuid: string): Promise<Item[]> {
  const endpoint = `/api/publishers/${publisherUuid}/items`;
  return fetchCached({
    url: endpoint,
    endpoint,
    validate: (body) => ItemListSchema.parse(body).items,
    getFromDb: () =>
      db.items.where('publisher_uuid').equals(publisherUuid).toArray(),
    putToDb: (rows) => db.items.bulkPut(rows).then(() => {}),
    getLastModified: () =>
      db.syncMeta.get(endpoint).then((m) => m?.last_modified),
    setLastModified: (v) =>
      db.syncMeta.put({ endpoint, last_modified: v }).then(() => {}),
  });
}
