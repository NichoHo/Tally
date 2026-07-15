import Link from "next/link";
import { api } from "@/lib/api";
import { Money } from "@/components/money";
import { IdTag } from "@/components/id-tag";
import { CreateAccountModal } from "@/components/create-account-modal";
import { Table, THead, Row, Cell, EmptyState } from "@/components/table";
import { formatDate } from "@/lib/dates";

export default async function AccountsPage() {
  const accounts = await api.accounts();

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-medium">Accounts</h1>
        <CreateAccountModal />
      </div>

      {accounts.length === 0 ? (
        <EmptyState message="No accounts yet. Create one to get started." />
      ) : (
        <Table>
          <THead
            cols={[
              { label: "Account" },
              { label: "Name" },
              { label: "Currency" },
              { label: "Balance", align: "right" },
              { label: "Created" },
            ]}
          />
          <tbody>
            {accounts.map((a) => (
              <Row key={a.id}>
                <Cell>
                  <Link href={`/accounts/${a.id}`} className="text-accent hover:underline">
                    <IdTag id={a.id} prefix="acct" />
                  </Link>
                </Cell>
                <Cell>{a.name}</Cell>
                <Cell>{a.currency}</Cell>
                <Cell align="right">
                  <Money minor={a.balance_minor} currency={a.currency} />
                </Cell>
                <Cell>
                  <span className="text-muted">{formatDate(a.created_at)}</span>
                </Cell>
              </Row>
            ))}
          </tbody>
        </Table>
      )}
    </div>
  );
}
