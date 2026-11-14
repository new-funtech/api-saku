-- Idempotency for clock-in/clock-out.
-- idempotency_key: the clock-in's key; unique per employee, so a repeat cannot create a second row.
-- check_out_idempotency_key: the key that closed the attendance, to tell a retry from a second clock-out.
ALTER TABLE attendances ADD COLUMN IF NOT EXISTS idempotency_key varchar(64);
ALTER TABLE attendances ADD COLUMN IF NOT EXISTS check_out_idempotency_key varchar(64);
CREATE UNIQUE INDEX IF NOT EXISTS idx_attendances_personnel_idempotency ON attendances (personnel_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
-- identical to idx_personnel_date (personnel_id, date), which stays as the one-row-per-day guarantee
DROP INDEX IF EXISTS idx_attendances_personnel_date;
