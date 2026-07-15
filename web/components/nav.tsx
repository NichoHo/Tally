"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

const LINKS = [
  { href: "/", label: "Dashboard" },
  { href: "/accounts", label: "Accounts" },
  { href: "/transfers", label: "Transfers" },
  { href: "/fraud", label: "Fraud" },
];

export function Nav({ horizontal = false }: { horizontal?: boolean }) {
  const pathname = usePathname();
  const isActive = (href: string) =>
    href === "/" ? pathname === "/" : pathname.startsWith(href);

  return (
    <nav className={horizontal ? "flex gap-1" : "flex flex-col gap-1"}>
      {LINKS.map((l) => (
        <Link
          key={l.href}
          href={l.href}
          className={`rounded-control px-3 py-2 text-sm ${
            isActive(l.href)
              ? "bg-accent text-white"
              : "text-muted hover:bg-page hover:text-ink"
          }`}
        >
          {l.label}
        </Link>
      ))}
    </nav>
  );
}
