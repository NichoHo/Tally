import Link from "next/link";
import { api } from "@/lib/api";
import { Money, formatMoney } from "@/components/money";
import { StatCard } from "@/components/stat-card";
import { StatusPill } from "@/components/status-pill";
import { IdTag } from "@/components/id-tag";
import { VolumeChart } from "@/components/volume-chart";
import { Table, THead, Row, Cell, EmptyState } from "@/components/table";

export default async function DashboardPage() {
  const [stats, recent] = await Promise.all([api.stats(), api.transfers({ limit: 10 })]);

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-medium">Dashboard</h1>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard label="Total accounts" value={String(stats.total_accounts)} />
        <StatCard
          label="Volume today"
          value={formatMoney(stats.transfer_volume_today_minor, "USD")}
        />
        <StatCard label="Transfers today" value={String(stats.transfer_count_today)} />
        <StatCard
          label="Flagged transfers"
          value={String(stats.flagged_count)}
          hint="decision review or block"
        />
      </div>

      <section className="rounded-card border border-line bg-card p-6">
        <h2 className="mb-4 text-sm font-medium text-muted">Transfer volume, last 7 days</h2>
        <VolumeChart data={stats.daily_volume} />
      </section>

      <section className="space-y-3">
        <div className="flex items-baseline justify-between">
          <h2 className="text-sm font-medium text-muted">Recent transfers</h2>
          <Link href="/transfers" className="text-sm text-accent hover:underline">
            View all
          </Link>
        </div>
        {recent.length === 0 ? (
          <EmptyState message="No transfers yet. Create one to get started." />
        ) : (
          <Table>
            <THead
              cols={[
                { label: "Transfer" },
                { label: "From" },
                { label: "To" },
                { label: "Amount", align: "right" },
                { label: "Status" },
                { label: "Fraud" },
              ]}
            />
            <tbody>
              {recent.map((t) => (
                <Row key={t.id}>
                  <Cell>
                    <Link href={`/transfers/${t.id}`} className="text-accent hover:underline">
                      <IdTag id={t.id} />
                    </Link>
                  </Cell>
                  <Cell>
                    <IdTag id={t.source_account_id} prefix="acct" />
                  </Cell>
                  <Cell>
                    <IdTag id={t.dest_account_id} prefix="acct" />
                  </Cell>
                  <Cell align="right">
                    <Money minor={t.amount_minor} currency={t.currency} />
                  </Cell>
                  <Cell>
                    <StatusPill status={t.status} />
                  </Cell>
                  <Cell>
                    {t.fraud_score ? <StatusPill status={t.fraud_score.decision} /> : (
                      <span className="text-xs text-muted">pending</span>
                    )}
                  </Cell>
                </Row>
              ))}
            </tbody>
          </Table>
        )}
      </section>
    </div>
  );
}
