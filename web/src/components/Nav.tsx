import { Link, useLocation } from '@tanstack/react-router';

const TYPES = ['book', 'video', 'audiobook', 'article'] as const;

export function Nav() {
  const location = useLocation();
  const params = new URLSearchParams(location.search);
  const activeType = params.get('type') ?? 'book';
  const onBrowse = location.pathname === '/browse';

  return (
    <nav className="sticky top-0 z-50 flex h-14 items-center gap-4 border-b border-gray-800 bg-gray-950 px-4">
      <Link to="/" className="shrink-0 text-sm font-semibold text-white">
        oreilly-cache
      </Link>

      <div className="flex flex-1 items-center gap-1">
        {TYPES.map((type) => (
          <Link
            key={type}
            to="/browse"
            search={{ type, sort: 'date' }}
            className={[
              'rounded px-3 py-1 text-xs font-medium capitalize transition-colors',
              onBrowse && activeType === type
                ? 'bg-gray-700 text-white'
                : 'text-gray-400 hover:text-white',
            ].join(' ')}
          >
            {type}
          </Link>
        ))}
      </div>

      {/* Search — non-functional stub, wired in a later step */}
      <input
        type="search"
        placeholder="Search…"
        className="w-48 shrink-0 rounded bg-gray-800 px-3 py-1.5 text-xs text-gray-300 placeholder-gray-600 outline-none"
      />
    </nav>
  );
}
