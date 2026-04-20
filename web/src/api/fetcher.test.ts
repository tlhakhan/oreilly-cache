import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

// Prevent Dexie from attempting real IndexedDB access in the test environment
vi.mock('../db/db', () => ({
  db: {
    syncMeta: { get: vi.fn(), put: vi.fn() },
    publishers: { toArray: vi.fn(), bulkPut: vi.fn() },
    items: {
      toArray: vi.fn(),
      bulkPut: vi.fn(),
      where: vi.fn(() => ({ equals: vi.fn(() => ({ toArray: vi.fn() })) })),
    },
  },
}));

import { type FetchConfig, fetchCached } from './fetcher';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type Row = { id: string };
const rows: Row[] = [{ id: 'a' }, { id: 'b' }];

function cfg(overrides: Partial<FetchConfig<Row>> = {}): FetchConfig<Row> {
  return {
    url: '/test',
    endpoint: '/test',
    validate: vi.fn().mockReturnValue(rows),
    getFromDb: vi.fn().mockResolvedValue(rows),
    putToDb: vi.fn().mockResolvedValue(undefined),
    getLastModified: vi.fn().mockResolvedValue(undefined),
    setLastModified: vi.fn().mockResolvedValue(undefined),
    ...overrides,
  };
}

function fakeResponse(
  status: number,
  body: unknown = {},
  lastModified?: string,
): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(body),
    headers: {
      get: (h: string) =>
        h === 'Last-Modified' ? (lastModified ?? null) : null,
    },
  } as unknown as Response;
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(fakeResponse(200)));
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('200 response', () => {
  it('calls validate, writes rows to db, and returns them', async () => {
    const validate = vi.fn().mockReturnValue(rows);
    const putToDb = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(fakeResponse(200, { data: rows })),
    );

    const result = await fetchCached(cfg({ validate, putToDb }));

    expect(validate).toHaveBeenCalledOnce();
    expect(putToDb).toHaveBeenCalledWith(rows);
    expect(result).toEqual(rows);
  });

  it('saves Last-Modified when header present', async () => {
    const setLastModified = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal(
      'fetch',
      vi
        .fn()
        .mockResolvedValue(
          fakeResponse(200, {}, 'Thu, 01 Jan 2026 00:00:00 GMT'),
        ),
    );

    await fetchCached(cfg({ setLastModified }));

    expect(setLastModified).toHaveBeenCalledWith(
      'Thu, 01 Jan 2026 00:00:00 GMT',
    );
  });

  it('does not call setLastModified when header absent', async () => {
    const setLastModified = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(fakeResponse(200)));

    await fetchCached(cfg({ setLastModified }));

    expect(setLastModified).not.toHaveBeenCalled();
  });
});

describe('304 response', () => {
  it('returns db rows without writing', async () => {
    const putToDb = vi.fn();
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(fakeResponse(304)));

    const result = await fetchCached(cfg({ putToDb }));

    expect(result).toEqual(rows);
    expect(putToDb).not.toHaveBeenCalled();
  });
});

describe('If-Modified-Since header', () => {
  it('sends header when last-modified is stored', async () => {
    const mockFetch = vi.fn().mockResolvedValue(fakeResponse(200));
    vi.stubGlobal('fetch', mockFetch);

    await fetchCached(
      cfg({
        getLastModified: vi
          .fn()
          .mockResolvedValue('Mon, 01 Dec 2025 00:00:00 GMT'),
      }),
    );

    expect(mockFetch).toHaveBeenCalledWith('/test', {
      headers: { 'If-Modified-Since': 'Mon, 01 Dec 2025 00:00:00 GMT' },
    });
  });

  it('omits header when no last-modified stored', async () => {
    const mockFetch = vi.fn().mockResolvedValue(fakeResponse(200));
    vi.stubGlobal('fetch', mockFetch);

    await fetchCached(cfg());

    expect(mockFetch).toHaveBeenCalledWith('/test', { headers: {} });
  });
});

describe('network error', () => {
  it('returns cached rows when db has data', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('offline')));

    const result = await fetchCached(cfg());

    expect(result).toEqual(rows);
  });

  it('rethrows when db is empty', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('offline')));

    await expect(
      fetchCached(cfg({ getFromDb: vi.fn().mockResolvedValue([]) })),
    ).rejects.toThrow('offline');
  });
});

describe('server error (5xx)', () => {
  it('returns cached rows when db has data', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(fakeResponse(500)));

    const result = await fetchCached(cfg());

    expect(result).toEqual(rows);
  });

  it('throws when db is empty', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(fakeResponse(500)));

    await expect(
      fetchCached(cfg({ getFromDb: vi.fn().mockResolvedValue([]) })),
    ).rejects.toThrow('HTTP 500: /test');
  });
});
