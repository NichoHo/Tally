// Server-side API client. Server components call these; the browser only ever
// talks to /api/* (proxied to the gateway by next.config.mjs rewrites).

const API_URL = process.env.API_URL || "http://localhost:8080";

export type Account = {
  id: number;
  name: string;
  currency: string;
  balance_minor: number;
  allow_negative: boolean;
  created_at: string;
  updated_at: string;
};

export type LedgerEntry = {
  id: number;
  transfer_id: number;
  account_id: number;
  direction: "debit" | "credit";
  amount_minor: number;
  created_at: string;
};

export type FraudScore = {
  transfer_id: number;
  score: string;
  decision: "allow" | "review" | "block";
  model_version: string;
  created_at: string;
};

export type Transfer = {
  id: number;
  source_account_id: number;
  dest_account_id: number;
  amount_minor: number;
  currency: string;
  status: string;
  created_at: string;
  entries?: LedgerEntry[];
  fraud_score?: FraudScore;
};

export type Stats = {
  total_accounts: number;
  transfer_count_today: number;
  transfer_volume_today_minor: number;
  flagged_count: number;
  daily_volume: { date: string; volume_minor: number; count: number }[];
};

async function get<T>(path: string): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, { cache: "no-store" });
  if (!res.ok) {
    throw new Error(`GET ${path} failed: ${res.status}`);
  }
  return res.json() as Promise<T>;
}

export const api = {
  stats: () => get<Stats>("/v1/stats"),
  accounts: () => get<Account[]>("/v1/accounts"),
  account: (id: string) => get<Account>(`/v1/accounts/${id}`),
  accountEntries: (id: string) => get<LedgerEntry[]>(`/v1/accounts/${id}/entries`),
  transfers: (params?: { limit?: number; before_id?: number }) => {
    const q = new URLSearchParams();
    if (params?.limit) q.set("limit", String(params.limit));
    if (params?.before_id) q.set("before_id", String(params.before_id));
    const qs = q.toString();
    return get<Transfer[]>(`/v1/transfers${qs ? `?${qs}` : ""}`);
  },
  transfer: (id: string) => get<Transfer>(`/v1/transfers/${id}`),
  fraudFlags: () => get<Transfer[]>("/v1/fraud/flags"),
};
