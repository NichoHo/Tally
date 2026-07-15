// Status pills: colored tint background with a darker text shade of the same
// color, never plain black (CLAUDE.md section 12).

const STYLES: Record<string, string> = {
  completed: "bg-ok-tint text-ok-dark",
  allow: "bg-ok-tint text-ok-dark",
  review: "bg-warn-tint text-warn-dark",
  pending: "bg-warn-tint text-warn-dark",
  block: "bg-danger-tint text-danger-dark",
  failed: "bg-danger-tint text-danger-dark",
};

export function StatusPill({ status }: { status: string }) {
  const style = STYLES[status] ?? "bg-line text-muted";
  return (
    <span className={`inline-block rounded-full px-2.5 py-0.5 text-xs font-medium ${style}`}>
      {status}
    </span>
  );
}
