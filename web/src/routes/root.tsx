import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createRootRoute, Outlet } from '@tanstack/react-router';
import { Nav } from '../components/Nav';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { staleTime: 5 * 60 * 1000 },
  },
});

export const rootRoute = createRootRoute({
  component: () => (
    <QueryClientProvider client={queryClient}>
      <Nav />
      <Outlet />
    </QueryClientProvider>
  ),
});
