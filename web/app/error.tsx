"use client";

// Shown when a server component's API call throws. On the hosted demo the usual
// cause is a cold backend: Render spins free services down after about 15
// minutes idle, and Vercel caps a server render at 60 seconds, so the first
// load after a quiet spell can give up before the gateway finishes waking.
// Retrying a few seconds later works, because the wake is already under way.
export default function Error({ reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return (
    <div className="mx-auto max-w-md rounded-card border border-line bg-card p-6 text-center">
      <h2 className="text-base font-medium">Could not reach the API</h2>
      <p className="mt-2 text-sm text-muted">
        The demo backend spins down when idle and takes up to a minute to wake up.
        Give it a few seconds, then try again.
      </p>
      <button
        onClick={reset}
        className="mt-4 rounded-control bg-accent px-4 py-2 text-sm font-medium text-white hover:bg-accent-dark"
      >
        Try again
      </button>
    </div>
  );
}
