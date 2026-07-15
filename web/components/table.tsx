import type { ReactNode } from "react";

// Table primitives with the section 12 look: hairline separators, muted
// headers, comfortable rows, no zebra striping. Money columns pass
// align="right" so digits line up.

export function Table({ children }: { children: ReactNode }) {
  return (
    <div className="overflow-x-auto rounded-card border border-line bg-card">
      <table className="w-full text-sm">{children}</table>
    </div>
  );
}

export function THead({ cols }: { cols: { label: string; align?: "right" }[] }) {
  return (
    <thead>
      <tr className="border-b border-line">
        {cols.map((c) => (
          <th
            key={c.label}
            className={`px-4 py-3 text-xs font-medium text-muted ${
              c.align === "right" ? "text-right" : "text-left"
            }`}
          >
            {c.label}
          </th>
        ))}
      </tr>
    </thead>
  );
}

export function Row({ children }: { children: ReactNode }) {
  return <tr className="h-12 border-b border-line last:border-0 hover:bg-page">{children}</tr>;
}

export function Cell({
  children,
  align,
}: {
  children: ReactNode;
  align?: "right";
}) {
  return (
    <td className={`px-4 py-2 ${align === "right" ? "text-right tabular-nums" : ""}`}>
      {children}
    </td>
  );
}

export function EmptyState({ message }: { message: string }) {
  return (
    <div className="rounded-card border border-line bg-card p-10 text-center text-sm text-muted">
      {message}
    </div>
  );
}
