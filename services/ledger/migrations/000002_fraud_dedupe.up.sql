-- One fraud score per transfer. This is what makes event consumption idempotent:
-- the fraud service inserts with ON CONFLICT (transfer_id) DO NOTHING, so
-- processing the same transfers.completed event twice cannot create two rows.
CREATE UNIQUE INDEX idx_fraud_transfer_unique ON fraud_scores(transfer_id);

-- The old non-unique index is redundant now.
DROP INDEX IF EXISTS idx_fraud_transfer;
