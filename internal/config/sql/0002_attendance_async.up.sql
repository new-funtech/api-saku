-- Asynchronous attendance (RabbitMQ). Official time = when the API received the request;
-- client_timestamp is only what the device claimed.
ALTER TABLE attendances ADD COLUMN IF NOT EXISTS client_timestamp timestamptz;
ALTER TABLE attendances ADD COLUMN IF NOT EXISTS server_received_at timestamptz;
CREATE TABLE IF NOT EXISTS attendance_failures (
    id uuid PRIMARY KEY,
    personnel_id uuid NOT NULL,
    idempotency_key varchar(64) NOT NULL,
    action varchar(10) NOT NULL,
    error text,
    attempts integer,
    created_at timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_attendance_failures_key ON attendance_failures (personnel_id, idempotency_key, action);
