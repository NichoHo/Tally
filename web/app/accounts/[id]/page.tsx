import Link from "next/link";
import { api } from "@/lib/api";
import { Money, formatMoney } from "@/components/money";
import { IdTag } from "@/components/id-tag";
import { Table, THead, Row, Cell, EmptyState } from "@/components/table";
import { formatDateTime } from "@/lib/dates";

export default async function AccountDetailPage({ params }: { params: { id: string } }) {
  const [account, entries] = await Promise.all([
    api.account(params.id),
    api.accountEntries(params.id),
  ]);

  // Entries arrive newest first. The newest entry leaves the account at its
  // current balance; walking down the list, the balance after each older entry
  // is the newer running balance minus the newer entry's signed amount.
  const running: number[] = [];
  let bal = account.balance_minor;
  for (const e of entries) {
    running.push(bal);
    bal -= e.direction === "credit" ? e.amount_minor : -e.amount_minor;
  }

  return (
    <div className="space-y-6">
      <div>
        <Link href="/accounts" className="text-sm text-accent hover:underline">
          Accounts
        </Link>
        <h1 className="mt-1 text-xl font-medium">{account.name}</h1>
        <div className="mt-1 flex items-center gap-2 text-sm text-muted">
          <IdTag id={account.id} prefix="acct" />
          <span>{account.currency}</span>
          {account.allow_negative ? <span>may go negative</span> : null}
        </div>
      </div>

      <div className="rounded-card border border-line bg-card p-6">
        <div className="text-sm text-muted">Current balance</div>
        <div className="mt-1 text-4xl font-medium tabular-nums">
          {formatMoney(account.balance_minor, account.currency)}
        </div>
      </div>

      <section className="space-y-3">
        <h2 className="text-sm font-medium text-muted">Ledger entries</h2>
        {entries.length === 0 ? (
          <EmptyState message="No ledger entries yet. This account has not moved money." />
        ) : (
          <Table>
            <THead
              cols={[
                { label: "Date" },
                { label: "Transfer" },
                { label: "Direction" },
                { label: "Amount", align: "right" },
                { label: "Balance", align: "right" },
              ]}
            />
            <tbody>
              {entries.map((e, i) => (
                <Row key={e.id}>
                  <Cell>
                    <span className="text-muted">{formatDateTime(e.created_at)}</span>
                  </Cell>
                  <Cell>
                    <Link href={`/transfers/${e.transfer_id}`} className="text-accent hover:underline">
                      <IdTag id={e.transfer_id} />
                    </Link>
                  </Cell>
                  <Cell>
                    <span className={e.direction === "credit" ? "text-ok-dark" : "text-ink"}>
                      {e.direction}
                    </span>
                  </Cell>
                  <Cell align="right">
                    <span className={e.direction === "credit" ? "text-ok-dark" : ""}>
                      {e.direction === "credit" ? "+" : "-"}
                      <Money minor={e.amount_minor} currency={account.currency} />
                    </span>
                  </Cell>
                  <Cell align="right">
                    <Money minor={running[i]} currency={account.currency} />
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
