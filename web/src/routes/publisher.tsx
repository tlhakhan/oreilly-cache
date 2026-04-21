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

const TYPE_LABELS = [
  { value: 'book',                label: 'Books' },
  { value: 'video',               label: 'Videos' },
  { value: 'article',             label: 'Articles' },
  { value: 'audiobook',           label: 'Audiobooks' },
  { value: 'learning-plan',       label: 'Learning Plans' },
  { value: 'live-event-series',   label: 'Live Event Series' },
  { value: 'scenario',            label: 'Scenarios' },
  { value: 'certs-practice-exam', label: 'Certs Practice Exams' },
] as const;

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

  const typeCounts = useMemo(() => {
    const counts: Partial<Record<string, number>> = {};
    for (const item of items) counts[item.type] = (counts[item.type] ?? 0) + 1;
    return counts;
  }, [items]);

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
        <h1 className="text-xl font-semibold text-gray-900">
          {publisher?.name ?? uuid}
          {items.length > 0 && (
            <span className="ml-2 text-base font-normal text-gray-400">
              ({items.length.toLocaleString()})
            </span>
          )}
        </h1>
        {Object.keys(typeCounts).length > 0 && (
          <div className="mt-2 mb-6 flex flex-wrap gap-1.5">
            {TYPE_LABELS.filter(({ value }) => typeCounts[value]).map(({ value }) => (
              <span
                key={value}
                className="rounded-full bg-gray-100 px-2.5 py-0.5 text-xs text-gray-500"
              >
                {value} ({typeCounts[value]!.toLocaleString()})
              </span>
            ))}
          </div>
        )}
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
