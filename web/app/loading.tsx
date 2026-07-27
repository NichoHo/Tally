// Root loading state. Next.js wraps every page in a Suspense boundary with
// this, so the shell (top bar, sidebar) paints immediately while the server
// component waits on the API. That matters on the hosted demo, where the
// gateway spins down when idle and can take a while to answer the first call.
export default function Loading() {
  return (
    <div className="animate-pulse space-y-6" aria-busy="true" aria-label="Loading">
      <div className="h-7 w-44 rounded bg-line" />
      <div className="h-64 rounded-card border border-line bg-card" />
    </div>
  );
}
