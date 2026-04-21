import { Link, useLocation } from '@tanstack/react-router';
import { useLiveQuery } from 'dexie-react-hooks';
import { useRef, useState } from 'react';
import { db } from '../db/db';
import type { ItemType } from '../api/types';

const TYPES = [
  { value: 'book', label: 'Books' },
  { value: 'learning-plan', label: 'Learning Plans' },
  { value: 'article', label: 'Articles' },
  { value: 'video', label: 'Videos' },
  { value: 'audiobook', label: 'Audiobooks' },
  { value: 'live-event-series', label: 'Live Event Series' },
  { value: 'scenario', label: 'Scenarios' },
  { value: 'certs-practice-exam', label: 'Certs Practice Exams' },
] as const;

export function Nav() {
  const location = useLocation();
  const params = new URLSearchParams(location.search);
  const activeType = params.get('type') ?? 'book';
  const onBrowse = location.pathname === '/browse';
  const onPublishers = location.pathname.startsWith('/publishers');
  const [browseOpen, setBrowseOpen] = useState(false);
  const closeTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const typeCounts = useLiveQuery(
    () => Promise.all(
      TYPES.map(({ value }) =>
        db.items.where('type').equals(value).count().then((n) => [value, n] as [ItemType, number])
      )
    ).then((pairs) => Object.fromEntries(pairs) as Record<ItemType, number>),
    []
  );

  function openMenu() {
    if (closeTimer.current) clearTimeout(closeTimer.current);
    setBrowseOpen(true);
  }

  function scheduleClose() {
    closeTimer.current = setTimeout(() => setBrowseOpen(false), 500);
  }

  return (
    <nav className="sticky top-0 z-50 flex h-14 items-center gap-4 border-b border-gray-200 bg-white px-4">
      <Link
        to="/publishers"
        className="shrink-0 font-semibold uppercase tracking-wide text-[#D30000]"
        style={{ fontStretch: 'condensed' }}
      >
        O'Reilly Cache
      </Link>

      <div className="flex flex-1 items-center gap-1">
        <Link
          to="/publishers"
          className={[
            'rounded px-3 py-1 text-xs font-medium transition-colors',
            onPublishers
              ? 'bg-gray-200 text-gray-900'
              : 'text-gray-500 hover:text-gray-900',
          ].join(' ')}
        >
          Publishers
        </Link>

        <div
          role="menu"
          className="relative"
          onMouseEnter={openMenu}
          onMouseLeave={scheduleClose}
        >
          <button
            type="button"
            className={[
              'rounded px-3 py-1 text-xs font-medium transition-colors',
              onBrowse
                ? 'bg-gray-200 text-gray-900'
                : 'text-gray-500 hover:text-gray-900',
            ].join(' ')}
          >
            Browse
          </button>

          {browseOpen && (
            <div className="absolute left-0 top-full w-48 rounded-md border border-gray-200 bg-white py-1 shadow-lg">
              {TYPES.map(({ value, label }) => (
                <Link
                  key={value}
                  to="/browse"
                  search={{ type: value, sort: 'date' }}
                  onClick={() => setBrowseOpen(false)}
                  className={[
                    'block px-4 py-1.5 text-xs transition-colors',
                    onBrowse && activeType === value
                      ? 'bg-gray-100 font-medium text-gray-900'
                      : 'text-gray-600 hover:bg-gray-50 hover:text-gray-900',
                  ].join(' ')}
                >
                  {label}
                  {typeCounts?.[value] != null && (
                    <span className="ml-1 text-gray-400">({typeCounts[value].toLocaleString()})</span>
                  )}
                </Link>
              ))}
            </div>
          )}
        </div>
      </div>

      <span className="shrink-0 text-xs text-gray-400">v{__APP_VERSION__}</span>
    </nav>
  );
}
