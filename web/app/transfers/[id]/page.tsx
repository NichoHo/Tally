import Link from "next/link";
import { api } from "@/lib/api";
import { formatMoney } from "@/components/money";
import { IdTag } from "@/components/id-tag";
import { StatusPill } from "@/components/status-pill";
import { formatDateTimeLong } from "@/lib/dates";

export default async function TransferDetailPage({ params }: { params: { id: string } }) {
  const t = await api.transfer(params.id);
  const debit = t.entries?.find((e) => e.direction === "debit");
  const credit = t.entries?.find((e) => e.direction === "credit");

  return (
    <div className="space-y-6">
      <div>
        <Link href="/transfers" className="text-sm text-accent hover:underline">
          Transfers
        </Link>
        <div className="mt-1 flex items-center gap-3">
          <h1 className="text-xl font-medium">Transfer</h1>
          <IdTag id={t.id} />
          <StatusPill status={t.status} />
        </div>
      </div>

      <div className="rounded-card border border-line bg-card p-6">
        <div className="text-sm text-muted">Amount</div>
        <div className="mt-1 text-4xl font-medium tabular-nums">
          {formatMoney(t.amount_minor, t.currency)}
        </div>
        <div className="mt-2 text-sm text-muted">{formatDateTimeLong(t.created_at)}</div>
      </div>

      {/* The two sides of the double entry, shown side by side so the idea is
          visually obvious: same amount leaves one account and arrives in the
          other. */}
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <div className="rounded-card border border-line bg-card p-6">
          <div className="mb-2 text-sm font-medium text-muted">Debit (money out)</div>
          {debit ? (
            <>
              <Link href={`/accounts/${debit.account_id}`} className="text-accent hover:underline">
                <IdTag id={debit.account_id} prefix="acct" />
              </Link>
              <div className="mt-2 text-2xl font-medium tabular-nums text-ink">
                -{formatMoney(debit.amount_minor, t.currency)}
              </div>
              <div className="mt-1 text-xs text-muted">
                entry <IdTag id={debit.id} />
              </div>
            </>
          ) : (
            <p className="text-sm text-muted">Missing entry.</p>
          )}
        </div>
        <div className="rounded-card border border-line bg-card p-6">
          <div className="mb-2 text-sm font-medium text-muted">Credit (money in)</div>
          {credit ? (
            <>
              <Link href={`/accounts/${credit.account_id}`} className="text-accent hover:underline">
                <IdTag id={credit.account_id} prefix="acct" />
              </Link>
              <div className="mt-2 text-2xl font-medium tabular-nums text-ok-dark">
                +{formatMoney(credit.amount_minor, t.currency)}
              </div>
              <div className="mt-1 text-xs text-muted">
                entry <IdTag id={credit.id} />
              </div>
            </>
          ) : (
            <p className="text-sm text-muted">Missing entry.</p>
          )}
        </div>
      </div>

      <div className="rounded-card border border-line bg-card p-6">
        <div className="mb-2 text-sm font-medium text-muted">Fraud check</div>
        {t.fraud_score ? (
          <div className="flex items-center gap-4">
            <StatusPill status={t.fraud_score.decision} />
            <span className="text-sm tabular-nums">score {t.fraud_score.score}</span>
            <span className="text-xs text-muted">{t.fraud_score.model_version}</span>
          </div>
        ) : (
          <p className="text-sm text-muted">Not scored yet. The fraud service scores transfers within a few seconds.</p>
        )}
      </div>
    </div>
  );
}
