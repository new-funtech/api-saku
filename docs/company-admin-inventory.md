# Satu portal untuk admin perusahaan: inventarisasi (Tahap 0) dan audit isolasi tenant (Tahap 1)

Dibuat 2026-11-14. Semua angka dan temuan di sini berasal dari pembacaan kode dan dari tes otomatis `tests/tenantiso/`, bukan perkiraan.

## 1. Model role dan tenant di API

| Role | Cakupan | Catatan |
|---|---|---|
| `super_admin`, `admin` | Pusat: lintas tenant | `utils.IsPusat` |
| `company_admin` | Satu BUJP | admin perusahaan (target portal tunggal) |
| `supervisor` | Satu BUJP | operasional, tanpa data master |
| `guard` | Datanya sendiri (`personnelID`) | |
| `bprks` | Pemeriksa pinjaman lintas perusahaan | hanya alur pinjaman |

**Tenant ditentukan di server.** `AuthRequired` memverifikasi JWT (user_id, email, role). `LoadCallerScope` memuat `bujpID` dan `personnelID` dari tabel `users`/`personnels` (cache 60 dtk) dan menaruhnya di `c.Locals`. Tidak ada tenant dari body, query, atau path. Mulai tahap ini `LoadCallerScope` juga memakai **role dan status dari database**, bukan dari token (akun nonaktif atau dihapus: 401; role yang diturunkan berlaku seketika).

Helper otorisasi: `pkg/utils/scope.go` (`ApplyBujpScope`, `CanAccessBujp`, `CanReadBujp`, `CanAccessPersonnel`, ...) dan `internal/handlers/authz.go` (pin perusahaan, kepemilikan record per orang, kepemilikan file, aturan role yang boleh diberikan).

## 2. Portal admin (admin-abujapi) dan padanannya di client

Status: **ada** = sudah ada padanan yang dipakai admin perusahaan; **sebagian** = ada tetapi belum setara; **belum** = belum ada; **platform** = lintas tenant, tetap di portal admin.
"Padanan" di sini baru dibandingkan dari daftar halaman dan layanan; kesetaraan fungsi per halaman diperiksa di Tahap 3/4. Ini asumsi, bukan hasil uji.

| Fitur portal admin | Endpoint | client-web | Mobile | Catatan |
|---|---|---|---|---|
| Dashboard | `/dashboard/summary` | ada (`AdminHomePage`, statistik) | sebagian | |
| BUJP / perusahaan | `/bujps` | - | - | **platform**: hanya Pusat membuat/mengubah/menghapus; company admin hanya membaca perusahaannya |
| Lokasi | `/locations` | belum | belum | |
| Shift | `/shifts` | belum | belum | |
| Personnel | `/personnels` (+ bulk import, template) | sebagian (`AdminPersonnelPage`, daftar) | belum | CRUD belum ada di client |
| Assignment | `/assignments` | ada (`AdminPatrolAssignmentsPage`) | belum | |
| Absensi (semua personel) | `/attendances` | sebagian (`AdminAttendancePage`) | belum | nama, filter, dan pencarian perlu dilengkapi |
| Koreksi absensi | `/attendance-corrections` + approve/reject | ada (`ApprovalPage`) | ada (`approval_koreksi`) | |
| Cuti dan izin | `/leaves` + approve/reject | ada | ada (`approval_perizinan`) | |
| Lembur | `/overtimes` + approve/reject | ada | ada (`approval_lembur`) | |
| Patroli (validasi) | `/patrols` | belum | belum | staf memakai halaman patroli sendiri |
| Komponen gaji | `/salary-components` | belum | belum | |
| Payroll (generate, status bayar) | `/payrolls` | ada (`AdminPayrollPage`) | belum | |
| Laporan bulanan | `/monthly-reports` | belum | belum | |
| Produk pinjaman | `/loan-products` | belum | belum | punya Pusat (global) dan per perusahaan |
| Persetujuan, kelola, riwayat pinjaman, cicilan | `/loans`, `/loan-approvals`, `/loan-installments` | ada | ada | |
| User | `/users` | belum | belum | |
| Profil, notifikasi | `/users/profile`, `/notifications` | ada | ada | |
| Panduan | statis | - | - | |

## 3. Temuan keamanan (Tahap 1)

Diukur dengan tes `tests/tenantiso/`: dua perusahaan dibuat dari nol, lalu admin, supervisor, dan guard milik A mencoba membaca, mengubah, menghapus, dan membuat data milik B lewat HTTP. Hasil sebelum perbaikan: **120 permintaan lintas tenant dibolehkan, 66 baris perusahaan lain berubah atau terhapus, 18 error 500, dan 14 daftar membocorkan data perusahaan lain** (`tests/tenantiso/BASELINE_BEFORE_FIX.txt`).

| # | Temuan | Lokasi | Risiko | Perbaikan |
|---|---|---|---|---|
| 1 | Semua user login (termasuk guard) bisa membuat user dengan role `super_admin`/`admin` | `user_handler.go` CreateUser, route `/users` | **Kritis** | write user hanya `super_admin`/`admin`/`company_admin`; role yang boleh diberikan dibatasi per role pemanggil (`assignableRoles`); company admin hanya membuat guard dan supervisor di perusahaannya; tidak bisa menaikkan role sendiri atau mengubah/menghapus admin lain atau dirinya sendiri |
| 2 | Semua user bisa membuat, mengubah, menghapus perusahaan (BUJP) dan melihat semua perusahaan | `bujp_handler.go`, route `/bujps` | **Kritis** | tulis hanya Pusat; baca hanya perusahaan sendiri |
| 3 | Membaca, mengubah, menghapus data perusahaan lain lewat id: lokasi, shift, koreksi absen, cuti, patroli, laporan bulanan, absensi | handler masing-masing (tanpa cek kepemilikan) | Tinggi | cek kepemilikan sebelum membaca atau menulis (403 di luar tenant, 404 id tidak ada) |
| 4 | Role non-admin (guard, supervisor) bisa membuat data master (lokasi, shift, personel, komponen gaji, produk pinjaman, assignment) | route tanpa `RequireRoles` | Tinggi | write master data hanya admin; perusahaan dikunci ke perusahaan sesi |
| 5 | Payroll dibuat untuk personel perusahaan lain | `payroll_handler.go` Create | Tinggi | cek perusahaan dan kepemilikan personel |
| 6 | Link unduhan, hapus, pindah, dan unggah file S3 untuk key mana pun (KTP, foto, dokumen pinjaman) | `upload_handler.go`, personnel move | Tinggi | setiap id di path key harus dalam cakupan pemanggil; guard hanya file miliknya |
| 7 | Daftar produk pinjaman aktif dan antrean persetujuan pinjaman menampilkan data perusahaan lain; guard bisa membuka antrean | `loan_product_handler.go` GetActive, `loan_approval_handler.go` | Sedang | perusahaan dari sesi; antrean hanya untuk reviewer |
| 8 | Riwayat pinjaman per id personel milik perusahaan lain | `loan_handler.go` History | Sedang | cek kepemilikan personel |
| 9 | Akun yang dinonaktifkan atau dihapus, dan role yang diturunkan, tetap berlaku hingga token kedaluwarsa (24 jam) | `middleware/scope.go` | Sedang | role dan status dari database tiap request (cache 60 dtk) |
| 10 | PUT/DELETE/GET absensi per id tanpa cek role dan tenant | `attendance_handler.go` | Tinggi | diperbaiki di commit sebelumnya |
| 11 | Aksi pada notifikasi orang lain dibalas 200 tanpa efek | `notification_handler.go` | Rendah | tidak bisa dieksploitasi (query per penerima); dibiarkan, tes memastikan baris tidak berubah |
| 12 | Endpoint demo `products` (baca publik, tulis oleh user mana pun) | `product_handler.go` | Info | bukan data tenant; tidak diubah |

Permintaan yang menyebut perusahaan lain di body (misalnya membuat lokasi dengan `bujp_id` perusahaan B): sebagian handler memaksa perusahaan sesi diam-diam (sesuai aturan "tenant dari token"), sebagian menolak dengan 403. Keduanya diterima oleh tes selama tidak ada data yang masuk ke perusahaan B.

## 4. Cara menjalankan tes isolasi
```bash
tests/tenantiso/run.sh            # container Postgres sekali pakai + API baru + semua tes
TENANTISO_KEEP=1 tests/tenantiso/run.sh -run TestSanity   # biarkan API dan database hidup untuk debugging
```
Hanya untuk lokal (loopback). Tes mencakup: akses silang CRUD (company_admin, supervisor, guard dari kedua sisi), aksi keputusan (approve/reject/validate/disburse), pembuatan data di tenant lain dan eskalasi role, seluruh endpoint daftar/lookup/dashboard, endpoint platform, eskalasi role di dalam satu perusahaan, file S3, dan akun nonaktif atau role berubah.

## 5. Yang tidak tercakup (asumsi)
- Tes S3 hanya memeriksa pembuatan link bertanda tangan (tidak ada MinIO); unduhan nyata tidak diuji.
- Role `bprks` (pemeriksa lintas perusahaan) tidak diuji pada endpoint non-pinjaman.
- Ekspor laporan (PDF/Excel) belum ada di API; akan dibuat di Tahap 2/3 dengan filter tenant yang sama.
- Cache scope per instance (60 dtk): perubahan role/status/perusahaan terlihat paling lama setelah TTL pada instance lain. Atur `SCOPE_CACHE_TTL_SECONDS=0` bila ingin seketika.

## 6. Status setelah Tahap 2-4

| Fitur | Endpoint (Tahap 2) | client-web (Tahap 3) | Mobile (Tahap 4) | Portal admin |
|---|---|---|---|---|
| Lokasi, Shift, Komponen Gaji | ada, kini terisolasi per tenant | `/company/locations`, `/company/shifts`, `/company/salary-components` | Kelola > Lokasi, Shift, Komponen Gaji | bisa dimatikan |
| Personel (tambah, ubah, nonaktifkan) | ada | `/company/personnel` | Kelola > Personel | bisa dimatikan; **impor massal Excel** (`/personnels/bulk-import`) belum ada di client |
| Pengguna (tambah, ubah, nonaktifkan) | ada, dengan batas role | `/company/users` | Kelola > Pengguna | bisa dimatikan |
| Riwayat absen seluruh personel | filter dan pencarian baru | `/admin/attendance` (halaman baru) | Riwayat Absensi | bisa dimatikan |
| Laporan bulanan | `GET /monthly-reports/recap` + `/export` (baru) | `/company/reports`, Excel dan cetak PDF | Laporan Bulanan, bagikan PDF | bisa dimatikan |
| Generate payroll, status bayar | ada | `/payroll` (sudah ada) | Penggajian (baru) | bisa dimatikan |
| Approval cuti, koreksi, lembur, kredit | ada | `/approval` (sudah ada) | inbox approval + rute Approval Lembur | bisa dimatikan |
| Penugasan | ada | `/admin/patrol-assignments` (sudah ada) | **belum** | tetap sampai ada di mobile |
| Validasi patroli | ada | **belum** | **belum** | tetap |
| Produk pinjaman | ada | **belum** (tidak diminta) | **belum** | tetap |
| Perusahaan (BUJP) | hanya Pusat | tidak ada | tidak ada | **platform: tetap** |
| Pengguna dan data lintas tenant, role Pusat | ada | tidak ada | tidak ada | **platform: tetap** |

Penyederhanaan di mobile (disengaja, bukan terlewat):
- **Ekspor Excel** tidak ada di mobile: berkas Excel tidak bisa dikelola di ponsel tanpa aplikasi lain. Gantinya **PDF yang bisa dibagikan**; Excel tetap di web.
- **Filter personel** di laporan bulanan diganti **pencarian nama/NIK/NIP** (daftar dropdown ratusan personel tidak praktis di layar kecil); filter lokasi tetap ada.
- Formulir personel di mobile memuat kolom inti (nama, NIK, NIP, kontak, tanggal bergabung, gaji pokok, rekening, status). **Unggah foto dan dokumen KTP/NPWP** serta **impor massal** tetap di web atau portal admin.

Asumsi untuk dikonfirmasi:
1. Admin perusahaan hanya mengelola `guard` dan `supervisor`; membuat atau mengubah sesama admin perusahaan tetap lewat Pusat.
2. Personel tidak bisa dihapus lewat client (hanya dinonaktifkan) agar riwayat absensi dan gaji tetap utuh.
3. Peran Pusat (`super_admin`, `admin`) tetap memakai portal admin; halaman Kelola tidak muncul untuk mereka.
4. Produk pinjaman, validasi patroli, dan impor massal belum dipindahkan: tidak diminta di tugas ini.
5. Portal admin **belum dihapus**. Sebelum dimatikan: uji tiap halaman baru di browser dan ponsel, dan putuskan nasib impor massal, validasi patroli, dan produk pinjaman.
