import { api } from "@/lib/api";
import { Money, formatMoney } from "@/components/money";
import { IdTag } from "@/components/id-tag";
import { StatCard } from "@/components/stat-card";
import { Table, THead, Row, Cell, EmptyState } from "@/components/table";

// Reconciliation recomputes every account's balance from its ledger entries and
// compares it to the cached balance, and checks the whole system sums to zero.
// It turns the ledger's core correctness property into something visible in a
// couple of seconds rather than only provable in a test run.
export default async function ReconcilePage() {
  const r = await api.reconcile();

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-medium">Reconciliation</h1>

      <div className={`rounded-card border border-line p-6 ${r.ok ? "bg-ok-tint" : "bg-danger-tint"}`}>
        <div className={`text-lg font-medium ${r.ok ? "text-ok-dark" : "text-danger-dark"}`}>
          {r.ok
            ? `All ${r.account_count} accounts reconcile, system sums to zero`
            : `${r.mismatch_count} of ${r.account_count} accounts do not reconcile`}
        </div>
        <div className="mt-1 text-sm text-muted">
          Cached balances match the balances recomputed from ledger entries, and every entry
          across the system sums to zero (money is conserved).
        </div>
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <StatCard label="Accounts checked" value={String(r.account_count)} />
        <StatCard label="Mismatches" value={String(r.mismatch_count)} />
        <StatCard
          label="System ledger sum"
          value={formatMoney(r.system_sum_minor, "USD")}
          hint="must be zero"
        />
      </div>

      {r.accounts.length === 0 ? (
        <EmptyState message="No accounts yet. Create one to get started." />
      ) : (
        <Table>
          <THead
            cols={[
              { label: "Account" },
              { label: "Name" },
              { label: "Cached balance", align: "right" },
              { label: "Recomputed", align: "right" },
              { label: "Status" },
            ]}
          />
          <tbody>
            {r.accounts.map((a) => (
              <Row key={a.account_id}>
                <Cell>
                  <IdTag id={a.account_id} prefix="acct" />
                </Cell>
                <Cell>{a.name}</Cell>
                <Cell align="right">
                  <Money minor={a.cached_minor} currency={a.currency} />
                </Cell>
                <Cell align="right">
                  <Money minor={a.computed_minor} currency={a.currency} />
                </Cell>
                <Cell>
                  <span
                    className={`inline-block rounded-full px-2.5 py-0.5 text-xs font-medium ${
                      a.ok ? "bg-ok-tint text-ok-dark" : "bg-danger-tint text-danger-dark"
                    }`}
                  >
                    {a.ok ? "reconciled" : "mismatch"}
                  </span>
                </Cell>
              </Row>
            ))}
          </tbody>
        </Table>
      )}
    </div>
  );
}
