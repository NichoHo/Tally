"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import type { Account } from "@/lib/api";

// The idempotency key is generated client-side when the form data changes and
// reused across retries of the same submission, so a double click or a network
// retry can never move money twice. A successful submit rotates the key.
export function NewTransferForm({ accounts }: { accounts: Account[] }) {
  const router = useRouter();
  const [source, setSource] = useState("");
  const [dest, setDest] = useState("");
  const [amount, setAmount] = useState("");
  const [idemKey, setIdemKey] = useState(() => crypto.randomUUID());
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const currency = accounts.find((a) => String(a.id) === source)?.currency ?? "USD";

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    setSuccess(null);
    try {
      const amountMinor = Math.round(parseFloat(amount) * 100);
      if (!Number.isFinite(amountMinor) || amountMinor <= 0) {
        throw new Error("enter a positive amount");
      }
      const res = await fetch("/api/v1/transfers", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Idempotency-Key": idemKey,
        },
        body: JSON.stringify({
          source_account_id: Number(source),
          dest_account_id: Number(dest),
          amount_minor: amountMinor,
          currency,
        }),
      });
      const body = (await res.json()) as { id?: number; error?: string };
      if (!res.ok) {
        throw new Error(body.error ?? `request failed (${res.status})`);
      }
      setSuccess(`Transfer #${body.id} completed.`);
      setAmount("");
      setIdemKey(crypto.randomUUID()); // next submission is a new request
      router.refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "something went wrong");
    } finally {
      setBusy(false);
    }
  }

  const options = accounts.map((a) => (
    <option key={a.id} value={a.id}>
      {a.name} ({a.currency})
    </option>
  ));

  return (
    <form onSubmit={submit} className="rounded-card border border-line bg-card p-6">
      <h2 className="mb-4 text-base font-medium">New transfer</h2>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-4">
        <label className="block text-sm">
          <span className="text-muted">From</span>
          <select
            value={source}
            onChange={(e) => setSource(e.target.value)}
            required
            className="mt-1 w-full rounded-control border border-line bg-card px-3 py-2 focus:border-accent focus:outline-none"
          >
            <option value="" disabled>
              Select account
            </option>
            {options}
          </select>
        </label>
        <label className="block text-sm">
          <span className="text-muted">To</span>
          <select
            value={dest}
            onChange={(e) => setDest(e.target.value)}
            required
            className="mt-1 w-full rounded-control border border-line bg-card px-3 py-2 focus:border-accent focus:outline-none"
          >
            <option value="" disabled>
              Select account
            </option>
            {options}
          </select>
        </label>
        <label className="block text-sm">
          <span className="text-muted">Amount ({currency})</span>
          <input
            value={amount}
            onChange={(e) => setAmount(e.target.value)}
            required
            inputMode="decimal"
            placeholder="0.00"
            className="mt-1 w-full rounded-control border border-line px-3 py-2 text-right tabular-nums focus:border-accent focus:outline-none"
          />
        </label>
        <div className="flex items-end">
          <button
            type="submit"
            disabled={busy}
            className="w-full rounded-control bg-accent px-4 py-2 text-sm font-medium text-white hover:bg-accent-dark disabled:opacity-50"
          >
            {busy ? "Sending..." : "Send"}
          </button>
        </div>
      </div>
      <div className="mt-3 flex items-center gap-2 text-xs text-muted">
        <span>Idempotency key</span>
        <span className="rounded bg-page px-1.5 py-0.5 font-mono border border-line">{idemKey}</span>
      </div>
      {error ? <p className="mt-3 text-sm text-danger">{error}</p> : null}
      {success ? <p className="mt-3 text-sm text-ok-dark">{success}</p> : null}
    </form>
  );
}
