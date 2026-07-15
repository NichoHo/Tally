import Link from "next/link";
import { api } from "@/lib/api";
import { Money } from "@/components/money";
import { IdTag } from "@/components/id-tag";
import { StatusPill } from "@/components/status-pill";
import { Table, THead, Row, Cell, EmptyState } from "@/components/table";
import { formatDateTime } from "@/lib/dates";

export default async function FraudPage() {
  const flagged = await api.fraudFlags();

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-medium">Fraud flags</h1>
        <p className="mt-1 text-sm text-muted">
          Transfers the fraud model marked for review or blocked. Scores come from
          an IsolationForest trained on synthetic data and are illustrative.
        </p>
      </div>

      {flagged.length === 0 ? (
        <EmptyState message="No flagged transfers. Everything the model has seen looks ordinary." />
      ) : (
        <Table>
          <THead
            cols={[
              { label: "Transfer" },
              { label: "Date" },
              { label: "Amount", align: "right" },
              { label: "Score", align: "right" },
              { label: "Decision" },
              { label: "Model" },
            ]}
          />
          <tbody>
            {flagged.map((t) => (
              <Row key={t.id}>
                <Cell>
                  <Link href={`/transfers/${t.id}`} className="text-accent hover:underline">
                    <IdTag id={t.id} />
                  </Link>
                </Cell>
                <Cell>
                  <span className="text-muted">{formatDateTime(t.created_at)}</span>
                </Cell>
                <Cell align="right">
                  <Money minor={t.amount_minor} currency={t.currency} />
                </Cell>
                <Cell align="right">{t.fraud_score?.score ?? ""}</Cell>
                <Cell>
                  {t.fraud_score ? <StatusPill status={t.fraud_score.decision} /> : null}
                </Cell>
                <Cell>
                  <span className="text-xs text-muted">{t.fraud_score?.model_version}</span>
                </Cell>
              </Row>
            ))}
          </tbody>
        </Table>
      )}
    </div>
  );
}
