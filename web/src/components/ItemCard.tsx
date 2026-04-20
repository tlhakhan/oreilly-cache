import type { Item } from '../api/types';
import { CoverImage } from './CoverImage';

interface Props {
  item: Item;
}

function oreillUrl(ourn: string): string {
  const parts = ourn.split(':');
  const id = parts[parts.length - 1] ?? ourn;
  return `https://www.oreilly.com/library/view/-/${id}/`;
}

export function ItemCard({ item }: Props) {
  return (
    <a
      href={oreillUrl(item.ourn)}
      target="_blank"
      rel="noreferrer"
      className="flex flex-col"
    >
      <div className="relative aspect-[2/3]">
        <CoverImage ourn={item.ourn} title={item.name} />
      </div>
      <div className="mt-2 min-w-0">
        <p className="line-clamp-2 text-sm leading-snug font-medium text-gray-900">
          {item.name}
        </p>
        <p className="mt-0.5 text-xs text-gray-500">
          {item.type} · {item.publication_date.slice(0, 4)}
        </p>
      </div>
    </a>
  );
}
