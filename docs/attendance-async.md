# Absensi: Idempotency-Key dan mode asinkron (RabbitMQ)

## Mode sinkron (default)
`POST /api/v1/attendances` (clock-in) dan `PUT /api/v1/attendances/checkout` (clock-out) berperilaku seperti sebelumnya, dengan perbedaan berikut (berlaku di kedua mode):

| Perubahan | Sebelum | Sekarang |
|---|---|---|
| `date`, `check_in`, `check_out`, `status` dari klien | dipakai | **diabaikan**; server yang menentukan |
| Clock-in dobel di hari yang sama | 409 | **201** dengan data absen pertama, header `Idempotent-Replay: true` |
| Clock-out kedua | 200, menimpa jam keluar | key sama: 200 + `Idempotent-Replay: true`; key berbeda: **409** |
| Di luar radius lokasi | tidak dicek di server | **403**; tanpa koordinat: **400** |
| Header `Idempotency-Key` | tidak ada | opsional, UUID; salah format: 400 |

Tanpa header, server memakai key turunan `(personnel_id, tanggal, aksi)`, sehingga retry biasa tetap dikenali.
Aplikasi mobile yang ada tidak perlu diubah. Kode respons clock-in tetap **201** untuk absen baru maupun duplikat (aplikasi mobile menunggu 201).

## Mode asinkron (`ATTENDANCE_ASYNC=true`)
Default **mati**. Hidupkan hanya bila RabbitMQ dan `attendance-worker` sudah berjalan; tanpa publisher API tetap sinkron.

Alur: API memvalidasi (assignment aktif, geofence, shift), mem-publish dengan **publisher confirm**, lalu menjawab **202**. Worker menyimpan ke database.

**Perubahan kontrak (hanya mode async):**

| | Respons |
|---|---|
| Clock-in / clock-out diterima | `202 Accepted`, header `Idempotency-Key`, `Location: /api/v1/attendances/status/{key}`, body `data: {idempotency_key, state:"pending", received_at, status_url}` |
| Broker gagal mengonfirmasi | `503` (tidak ada yang antre; klien boleh mencoba lagi dengan key yang sama) |
| Foto lebih besar dari `ATTENDANCE_MAX_PHOTO_BYTES` | `400` |
| Cek status | `GET /api/v1/attendances/status/{key}` → `state`: `pending` (antre / belum tersimpan), `recorded` (tersimpan, ada `attendance_id`), `failed` (ada `error`) |

Waktu resmi = waktu API menerima request (`server_received_at`), bukan waktu worker. `client_timestamp` hanya catatan audit.
Aplikasi mobile saat ini mengharapkan 201/200 dengan data absen, jadi **jangan** hidupkan mode async sebelum aplikasinya diperbarui untuk membaca 202 dan menampilkan "sedang diproses".

## Antrean
`attendance.events` (durable, pesan persisten) → worker (manual ack, prefetch `ATTENDANCE_WORKER_PREFETCH`).
Gagal → `attendance.events.retry.N` (TTL = backoff `ATTENDANCE_RETRY_DELAYS`) → kembali ke antrean utama. Setelah `ATTENDANCE_MAX_ATTEMPTS` gagal → `attendance.events.dlq` + baris di `attendance_failures`.
Worker idempoten (at-least-once): clock-in `INSERT ... ON CONFLICT DO NOTHING`, clock-out `UPDATE ... WHERE check_out IS NULL`. Batch maksimal `ATTENDANCE_BATCH_SIZE` pesan atau `ATTENDANCE_BATCH_WAIT`; bila satu batch gagal, diulang per pesan.
Foto diunggah ke S3 oleh worker, bukan di dalam request. (Tidak ada notifikasi atau rekap yang terjadi saat absen, jadi tidak ada yang dipindahkan selain foto.)

## Menjalankan
```bash
docker compose --profile async up -d                 # dev: rabbitmq + attendance-worker
docker compose -f docker-compose.prod.yaml --profile async up -d attendance-worker   # prod
```
Metrik worker: `GET :9101/metrics` (`attendance_queue_depth`, `attendance_retry_depth`, `attendance_dlq_depth`, `attendance_queue_wait_ms`, counters). Log JSON terstruktur per batch.

**Mengolah ulang DLQ** (setelah penyebabnya diperbaiki): `attendance-worker -reprocess-dlq [-limit N]`.

## Migrasi dan rollback
`internal/config/sql/NNNN_*.up.sql` dijalankan otomatis saat API/worker start (idempoten). Rollback manual, **kode dikembalikan dulu**, lalu:
```bash
psql -f internal/config/sql/0002_attendance_async.down.sql
psql -f internal/config/sql/0001_attendance_idempotency.down.sql
```
- 0001: kolom `idempotency_key`, `check_out_idempotency_key`, unique `(personnel_id, idempotency_key)`; menghapus index `(personnel_id, date)` yang dobel (unique `idx_personnel_date` tetap).
- 0002: kolom `client_timestamp`, `server_received_at`, tabel `attendance_failures`.

Mematikan mode async: `ATTENDANCE_ASYNC=false`, restart API. Biarkan worker berjalan sampai antrean kosong (`attendance_queue_depth` = 0).
