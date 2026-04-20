import { createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/')({
  component: Index,
});

function Index() {
  return (
    <main className="flex min-h-screen items-center justify-center bg-gray-950 text-white">
      <h1 className="text-2xl font-bold">oreilly-cache</h1>
    </main>
  );
}
