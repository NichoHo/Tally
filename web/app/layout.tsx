import type { Metadata } from "next";
import { Inter } from "next/font/google";
import { Nav } from "@/components/nav";
import "./globals.css";

const inter = Inter({ subsets: ["latin"], weight: ["400", "500"], variable: "--font-inter" });

export const metadata: Metadata = {
  title: "Tally",
  description: "Payments ledger with double-entry bookkeeping and fraud scoring",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={inter.variable}>
      <body className="font-sans">
        {/* Top bar */}
        <header className="flex items-center gap-3 border-b border-line bg-card px-6 py-3">
          <span className="text-base font-medium">Tally</span>
          <span className="rounded-full border border-line bg-page px-2 py-0.5 text-xs text-muted">
            demo data
          </span>
          {/* On small screens the sidebar collapses into this top menu */}
          <div className="ml-auto md:hidden">
            <Nav horizontal />
          </div>
        </header>
        <div className="flex">
          {/* Sidebar (desktop) */}
          <aside className="hidden min-h-[calc(100vh-53px)] w-48 shrink-0 border-r border-line bg-card p-4 md:block">
            <Nav />
          </aside>
          <main className="min-w-0 flex-1 p-6 md:p-8">{children}</main>
        </div>
      </body>
    </html>
  );
}
