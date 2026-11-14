-- Rollback of 0002. Roll the API/worker code back first, then run this.
DROP TABLE IF EXISTS attendance_failures;
ALTER TABLE attendances DROP COLUMN IF EXISTS server_received_at;
ALTER TABLE attendances DROP COLUMN IF EXISTS client_timestamp;
