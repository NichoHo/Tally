// IDs render in monospace inside a subtle gray pill (CLAUDE.md section 12).
export function IdTag({ id, prefix }: { id: number | string; prefix?: string }) {
  return (
    <span className="inline-block rounded bg-page px-1.5 py-0.5 font-mono text-xs text-muted border border-line">
      {prefix ? `${prefix}-${id}` : `#${id}`}
    </span>
  );
}
