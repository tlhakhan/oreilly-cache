import { createRoute, createRouter, redirect } from '@tanstack/react-router';
import { browseRoute } from './routes/browse';
import { publisherRoute } from './routes/publisher';
import { publishersRoute } from './routes/publishers';
import { rootRoute } from './routes/root';

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  beforeLoad: () => {
    throw redirect({ to: '/publishers' });
  },
});

const routeTree = rootRoute.addChildren([
  indexRoute,
  publishersRoute,
  publisherRoute,
  browseRoute,
]);

export const router = createRouter({ routeTree });

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}
