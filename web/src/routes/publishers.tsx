import { useQuery } from '@tanstack/react-query';
import { createRoute, Link } from '@tanstack/react-router';
import { fetchPublishers } from '../api/fetcher';
import { rootRoute } from './root';

export const publishersRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/publishers',
  component: Publishers,
});

function Publishers() {
  const { data: publishers = [], isLoading } = useQuery({
    queryKey: ['publishers'],
    queryFn: fetchPublishers,
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
      <div className="mx-auto max-w-screen-2xl px-4 py-6">
        <div className="grid grid-cols-2 gap-x-6 gap-y-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
          {publishers
            .filter((pub) => (pub.item_count ?? 0) > 0)
            .sort((a, b) => a.name.localeCompare(b.name))
            .map((pub) => (
              <Link
                key={pub.uuid}
                to="/publishers/$uuid"
                params={{ uuid: pub.uuid }}
                className="truncate text-sm text-gray-700 hover:text-gray-900 hover:underline visited:text-purple-600"
              >
                {pub.name}
                {pub.item_count != null && (
                  <span className="ml-1 text-gray-400">({pub.item_count})</span>
                )}
              </Link>
            ))}
        </div>
      </div>
    </div>
  );
}
