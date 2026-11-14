-- Rollback of 0001. Roll the API code back first (the model still maps these columns), then run this.
CREATE UNIQUE INDEX IF NOT EXISTS idx_attendances_personnel_date ON attendances (personnel_id, date);
DROP INDEX IF EXISTS idx_attendances_personnel_idempotency;
ALTER TABLE attendances DROP COLUMN IF EXISTS check_out_idempotency_key;
ALTER TABLE attendances DROP COLUMN IF EXISTS idempotency_key;
