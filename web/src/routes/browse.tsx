import { useQuery } from '@tanstack/react-query';
import { createFileRoute } from '@tanstack/react-router';
import { useWindowVirtualizer } from '@tanstack/react-virtual';
import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { z } from 'zod';
import { fetchItemsByType } from '../api/fetcher';
import type { Item } from '../api/types';
import { ItemCard } from '../components/ItemCard';
import { useScrollRestoration } from '../hooks/useScrollRestoration';

// ---------------------------------------------------------------------------
// Search params
// ---------------------------------------------------------------------------

const searchSchema = z.object({
  type: z
    .enum(['book', 'video', 'audiobook', 'article'])
    .default('book')
    .catch('book'),
  sort: z.enum(['date', 'popular']).default('date').catch('date'),
  publisher: z.string().optional(),
});

export const Route = createFileRoute('/browse')({
  validateSearch: searchSchema,
  component: Browse,
});

// ---------------------------------------------------------------------------
// Hooks
// ---------------------------------------------------------------------------

function getColumns(): number {
  const w = window.innerWidth;
  if (w >= 1280) return 6;
  if (w >= 1024) return 4;
  if (w >= 768) return 3;
  return 2;
}

function useColumns(): number {
  const [cols, setCols] = useState(getColumns);
  useEffect(() => {
    const h = () => setCols(getColumns());
    window.addEventListener('resize', h);
    return () => window.removeEventListener('resize', h);
  }, []);
  return cols;
}

// Estimate row height from current column count and viewport width
function useRowHeight(cols: number): number {
  const [height, setHeight] = useState(380);
  useEffect(() => {
    const gap = 16;
    const padding = 32;
    const cardWidth = (window.innerWidth - padding - (cols - 1) * gap) / cols;
    setHeight(Math.ceil(cardWidth * 1.5 + 80 + gap));
  }, [cols]);
  return height;
}

// ---------------------------------------------------------------------------
// Utilities
// ---------------------------------------------------------------------------

function chunk<T>(arr: T[], size: number): T[][] {
  const out: T[][] = [];
  for (let i = 0; i < arr.length; i += size) out.push(arr.slice(i, i + size));
  return out;
}

function sortItems(items: Item[], sort: 'date' | 'popular'): Item[] {
  return [...items].sort((a, b) =>
    sort === 'popular'
      ? b.popularity - a.popularity
      : b.publication_date.localeCompare(a.publication_date),
  );
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

function Browse() {
  const { type, sort, publisher } = Route.useSearch();
  const cols = useColumns();
  const rowHeight = useRowHeight(cols);
  const listRef = useRef<HTMLDivElement>(null);
  const [scrollMargin, setScrollMargin] = useState(0);

  useLayoutEffect(() => {
    setScrollMargin(listRef.current?.offsetTop ?? 0);
  }, []);

  const { data: items = [], isLoading } = useQuery({
    queryKey: ['items', type],
    queryFn: () => fetchItemsByType(type),
  });

  const rows = useMemo(() => {
    let list = sortItems(items, sort);
    if (publisher) list = list.filter((it) => it.publisher_uuid === publisher);
    return chunk(list, cols);
  }, [items, sort, publisher, cols]);

  const virtualizer = useWindowVirtualizer({
    count: rows.length,
    estimateSize: () => rowHeight,
    overscan: 3,
    scrollMargin,
  });

  const viewKey = `browse:${type}:${sort}:${publisher ?? ''}`;
  useScrollRestoration({ viewKey, virtualizer, scrollMargin, ready: !isLoading && rows.length > 0 });

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-gray-950 text-gray-400">
        Loading…
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-950 text-white">
      <div ref={listRef} className="mx-auto max-w-screen-2xl px-4 py-6">
        <div
          className="relative"
          style={{ height: `${virtualizer.getTotalSize()}px` }}
        >
          {virtualizer.getVirtualItems().map((vRow) => (
            <div
              key={vRow.key}
              className="absolute inset-x-0 grid gap-4 px-4"
              style={{
                top: 0,
                transform: `translateY(${vRow.start - scrollMargin}px)`,
                gridTemplateColumns: `repeat(${cols}, 1fr)`,
              }}
            >
              {(rows[vRow.index] ?? []).map((item) => (
                <ItemCard key={item.ourn} item={item} />
              ))}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
