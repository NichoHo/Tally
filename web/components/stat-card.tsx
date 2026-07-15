export function StatCard({ label, value, hint }: { label: string; value: string; hint?: string }) {
  return (
    <div className="rounded-card border border-line bg-card p-6">
      <div className="text-sm text-muted">{label}</div>
      <div className="mt-1 text-2xl font-medium text-ink tabular-nums">{value}</div>
      {hint ? <div className="mt-1 text-xs text-muted">{hint}</div> : null}
    </div>
  );
}
