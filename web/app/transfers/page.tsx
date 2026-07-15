import Link from "next/link";
import { api } from "@/lib/api";
import { Money } from "@/components/money";
import { IdTag } from "@/components/id-tag";
import { StatusPill } from "@/components/status-pill";
import { NewTransferForm } from "@/components/new-transfer-form";
import { Table, THead, Row, Cell, EmptyState } from "@/components/table";
import { formatDateTime } from "@/lib/dates";

const PAGE_SIZE = 20;

export default async function TransfersPage({
  searchParams,
}: {
  searchParams: { before_id?: string };
}) {
  const beforeID = Number(searchParams.before_id) || undefined;
  const [accounts, transfers] = await Promise.all([
    api.accounts(),
    api.transfers({ limit: PAGE_SIZE, before_id: beforeID }),
  ]);

  const lastID = transfers.length > 0 ? transfers[transfers.length - 1].id : 0;
  const hasOlder = transfers.length === PAGE_SIZE && lastID > 1;

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-medium">Transfers</h1>

      <NewTransferForm accounts={accounts} />

      {transfers.length === 0 ? (
        <EmptyState message="No transfers yet. Create one to get started." />
      ) : (
        <div className="space-y-3">
          <Table>
            <THead
              cols={[
                { label: "Transfer" },
                { label: "Date" },
                { label: "From" },
                { label: "To" },
                { label: "Amount", align: "right" },
                { label: "Status" },
              ]}
            />
            <tbody>
              {transfers.map((t) => (
                <Row key={t.id}>
                  <Cell>
                    <Link href={`/transfers/${t.id}`} className="text-accent hover:underline">
                      <IdTag id={t.id} />
                    </Link>
                  </Cell>
                  <Cell>
                    <span className="text-muted">{formatDateTime(t.created_at)}</span>
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
                </Row>
              ))}
            </tbody>
          </Table>
          <div className="flex justify-between text-sm">
            {beforeID ? (
              <Link href="/transfers" className="text-accent hover:underline">
                Newest
              </Link>
            ) : (
              <span />
            )}
            {hasOlder ? (
              <Link href={`/transfers?before_id=${lastID}`} className="text-accent hover:underline">
                Older
              </Link>
            ) : null}
          </div>
        </div>
      )}
    </div>
  );
}
