# API untuk admin perusahaan (company_admin)

Semua endpoint di bawah berlaku untuk `company_admin` (dan Pusat: `super_admin`, `admin`). Perusahaan (BUJP) selalu diambil dari sesi, tidak dari body, query, atau path. Pusat harus menyebut `bujp_id` bila endpoint membutuhkan perusahaan. Kontrak endpoint yang sudah dipakai klien lama tidak diubah; tambahan hanya parameter dan endpoint baru.

| Kebutuhan | Endpoint | Role |
|---|---|---|
| Lokasi: daftar, tambah, ubah, hapus | `GET/POST /locations`, `GET/PUT/DELETE /locations/:id` | baca: semua di perusahaan; tulis: admin |
| Shift | `/shifts`, `/shifts/:id` | sama |
| Komponen gaji | `/salary-components`, `/salary-components/:id` | sama |
| Produk pinjaman | `/loan-products`, `/loan-products/active`, `/loan-products/:id` | sama |
| Personel (tambah, ubah, nonaktifkan: `PUT` dengan `status: "inactive"`) | `/personnels`, `/personnels/:id`, `/personnels/bulk-import`, `/personnels/template/download` | tulis: admin |
| User (tambah, ubah, nonaktifkan: `PUT` dengan `status: "inactive"`) | `/users`, `/users/:id` | tulis: admin; company admin hanya membuat/mengubah `guard` dan `supervisor` di perusahaannya |
| Penugasan | `/assignments`, `/assignments/:id` | tulis: admin |
| Approval | `POST /leaves/:id/approve|reject`, `/attendance-corrections/:id/approve|reject`, `/overtimes/:id/approve|reject`, `/loan-approvals/...` | admin / reviewer |
| Generate payroll, ubah status bayar | `POST /payrolls/generate`, `POST /payrolls/:id/update-payment-status` | admin |
| Laporan bulanan (ringkasan lama) | `/monthly-reports`, `POST /monthly-reports/generate` | admin |
| **Rekap absensi bulanan (baru)** | `GET /monthly-reports/recap?period=YYYY-MM[&location_id][&personnel_id][&search][&bujp_id]` | admin |
| **Ekspor Excel rekap (baru)** | `GET /monthly-reports/recap/export?...` (parameter sama; berkas `.xlsx`) | admin |
| **Riwayat absen seluruh personel** | `GET /attendances` dengan `date_from`/`date_to` (atau `start_date`/`end_date`), `personnel_id`, `location_id`, `status`, `search` (nama, NIK, NIP), `page`, `per_page` (maks 200) | admin / supervisor (perusahaannya); guard hanya miliknya |

## Rekap absensi bulanan
Satu baris per personel perusahaan: `hadir` (present + late), `telat` (sudah termasuk hadir), `tidak_hadir` (absent), `izin` (permission + sick + leave), `lembur_jam` (lembur yang disetujui di bulan itu), `jam_kerja_menit`. `totals` menjumlahkan semua baris. Filter lokasi menyertakan personel yang punya penugasan atau absensi di lokasi itu.
Asumsi: angka berasal dari status baris absensi (`present`, `late`, `absent`, `permission`, `sick`, `leave`), sama dengan ringkasan dashboard; hari tanpa baris absensi tidak dihitung sebagai tidak hadir.

## Riwayat absen
Setiap baris memuat `personnel` (nama, NIK, NIP), `location`, `assignment.shift` (nama dan jam), `check_in`, `check_out`, `status`. Sebelumnya parameter `per_page`, `date_from`, dan `date_to` yang dikirim klien web diabaikan oleh API (hanya `limit`, `start_date`, `end_date` yang dibaca), sehingga daftar admin hanya berisi 10 baris tanpa filter tanggal; sekarang keduanya diterima.

## Status HTTP
`401` token tidak valid atau akun nonaktif/dihapus; `403` role tidak boleh atau data milik perusahaan lain; `404` id tidak ada; `400` parameter salah (misalnya `period` bukan `YYYY-MM`).
