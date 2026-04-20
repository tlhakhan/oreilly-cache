import { useEffect, useRef } from 'react';
import type { Virtualizer } from '@tanstack/react-virtual';
import { db } from '../db/db';

interface Options {
  viewKey: string;
  virtualizer: Virtualizer<Window, Element>;
  scrollMargin: number;
  ready: boolean;
}

export function useScrollRestoration({ viewKey, virtualizer, scrollMargin, ready }: Options) {
  const restored = useRef(false);
  const activeKey = useRef(viewKey);

  // Reset when viewKey changes
  if (activeKey.current !== viewKey) {
    activeKey.current = viewKey;
    restored.current = false;
  }

  // Restore on mount once data is ready
  useEffect(() => {
    if (!ready || restored.current) return;
    restored.current = true;

    db.scrollPos.get(viewKey).then((pos) => {
      if (!pos) return;
      requestAnimationFrame(() => {
        requestAnimationFrame(() => {
          virtualizer.scrollToIndex(pos.scroll_index, { align: 'start', behavior: 'instant' });
          window.scrollBy(0, pos.scroll_offset);
        });
      });
    });
  }, [ready, viewKey, virtualizer]);

  // Save on unmount
  useEffect(() => {
    const key = viewKey;
    return () => {
      const items = virtualizer.getVirtualItems();
      if (items.length === 0) return;
      const first = items[0];
      const scrollOffset = window.scrollY - scrollMargin - first.start;
      db.scrollPos.put({
        view_key: key,
        scroll_index: first.index,
        scroll_offset: scrollOffset,
        updated_at: Date.now(),
      });
    };
  }, [viewKey, virtualizer, scrollMargin]);
}
