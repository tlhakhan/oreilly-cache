import { useLiveQuery } from 'dexie-react-hooks';
import { Heart } from 'lucide-react';
import type { MouseEvent } from 'react';
import type { Item } from '../api/types';
import { db } from '../db/db';
import { CoverImage } from './CoverImage';

interface Props {
  item: Item;
}

function LikeButton({ ourn }: { ourn: string }) {
  const record = useLiveQuery(() => db.likes.get(ourn), [ourn]);
  const liked = record != null;

  function toggle(e: MouseEvent) {
    e.stopPropagation();
    if (liked) {
      db.likes.delete(ourn);
    } else {
      db.likes.put({ ourn, liked_at: Date.now() });
    }
  }

  return (
    <button
      type="button"
      onClick={toggle}
      className="absolute right-1.5 top-1.5 rounded-full bg-black/40 p-1.5 backdrop-blur-sm transition-colors hover:bg-black/60 focus:outline-none focus:ring-2 focus:ring-white/50"
      aria-label={liked ? 'Remove from likes' : 'Add to likes'}
    >
      <Heart
        className={`h-3.5 w-3.5 ${liked ? 'fill-red-500 stroke-red-500' : 'fill-none stroke-white'}`}
      />
    </button>
  );
}

export function ItemCard({ item }: Props) {
  return (
    <div className="flex flex-col">
      <div className="relative aspect-[2/3] overflow-hidden rounded bg-gray-800">
        <CoverImage ourn={item.ourn} title={item.name} />
        <LikeButton ourn={item.ourn} />
      </div>
      <div className="mt-2 min-w-0">
        <p className="line-clamp-2 text-sm leading-snug font-medium text-white">
          {item.name}
        </p>
        {/* TODO: show publisher name once publisher_uuid is reliably present on Item
            and a publisher lookup (Dexie or context) is wired up — see step 4 */}
        <p className="mt-0.5 text-xs text-gray-400">
          {item.type} · {item.publication_date.slice(0, 4)}
        </p>
      </div>
    </div>
  );
}
