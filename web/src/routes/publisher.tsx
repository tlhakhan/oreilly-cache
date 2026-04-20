import { useQuery } from '@tanstack/react-query';
import { createRoute } from '@tanstack/react-router';
import { useWindowVirtualizer } from '@tanstack/react-virtual';
import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { fetchPublisherItems, fetchPublishers } from '../api/fetcher';
import { ItemCard } from '../components/ItemCard';
import { rootRoute } from './root';

export const publisherRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/publishers/$uuid',
  component: Publisher,
});

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

function chunk<T>(arr: T[], size: number): T[][] {
  const out: T[][] = [];
  for (let i = 0; i < arr.length; i += size) out.push(arr.slice(i, i + size));
  return out;
}

function Publisher() {
  const { uuid } = publisherRoute.useParams();
  const cols = useColumns();
  const rowHeight = useRowHeight(cols);
  const listRef = useRef<HTMLDivElement>(null);
  const [scrollMargin, setScrollMargin] = useState(0);

  useLayoutEffect(() => {
    setScrollMargin(listRef.current?.offsetTop ?? 0);
  }, []);

  const { data: publishers = [] } = useQuery({
    queryKey: ['publishers'],
    queryFn: fetchPublishers,
  });
  const publisher = publishers.find((p) => p.uuid === uuid);

  const { data: items = [], isLoading } = useQuery({
    queryKey: ['publisher-items', uuid],
    queryFn: () => fetchPublisherItems(uuid),
  });

  const rows = useMemo(() => chunk(items, cols), [items, cols]);

  const virtualizer = useWindowVirtualizer({
    count: rows.length,
    estimateSize: () => rowHeight,
    overscan: 3,
    scrollMargin,
  });

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-gray-50 text-gray-400">
        Loading…
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50">
      <div ref={listRef} className="mx-auto max-w-screen-2xl px-4 py-6">
        <h1 className="mb-6 text-xl font-semibold text-gray-900">
          {publisher?.name ?? uuid}
        </h1>
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
