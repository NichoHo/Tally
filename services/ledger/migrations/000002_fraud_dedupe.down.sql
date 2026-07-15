CREATE INDEX idx_fraud_transfer ON fraud_scores(transfer_id);
DROP INDEX IF EXISTS idx_fraud_transfer_unique;
