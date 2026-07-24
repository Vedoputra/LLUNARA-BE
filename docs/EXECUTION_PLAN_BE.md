# Execution Plan — LLunara Backend (`llunara-api`)

**Dokumen ini adalah panduan eksekusi berurutan dari nol sampai selesai.**
Setiap task punya format yang sama agar dapat dikerjakan oleh developer maupun AI agent tanpa konteks tambahan.

---

## Cara Membaca Dokumen Ini

Setiap task memiliki struktur:

| Field | Arti |
|---|---|
| **ID** | Identitas unik task. Dipakai untuk referensi silang dengan plan Frontend |
| **Tujuan** | Kondisi akhir yang ingin dicapai. Jawaban atas "kenapa task ini ada" |
| **Langkah** | Instruksi teknis yang harus dilakukan |
| **Output** | Artefak konkret yang dihasilkan |
| **Selesai Jika** | Kriteria verifikasi. Task tidak boleh dianggap selesai sebelum ini terpenuhi |
| **Blocking** | Task Frontend yang tidak bisa jalan sebelum task ini selesai |

**Aturan:** kerjakan berurutan. Jangan lompat fase kecuali dependensinya sudah jelas terpenuhi.

---

## Tech Stack Backend

| Komponen | Pilihan | Alasan |
|---|---|---|
| Bahasa | Go (versi stabil terbaru) | Sudah dikuasai developer |
| Router | `github.com/go-chi/chi/v5` | Kompatibel `net/http`, middleware bersih |
| Database driver | `github.com/jackc/pgx/v5` | Driver Postgres paling matang di Go |
| JWT | `github.com/golang-jwt/jwt/v5` | Verifikasi token Supabase |
| Validasi | `github.com/go-playground/validator/v10` | Validasi struct berbasis tag |
| Env loader | `github.com/joho/godotenv` | Hanya untuk development lokal |
| Logging | `log/slog` (stdlib) | Structured logging tanpa dependency |
| PDF | `github.com/johnfercher/maroto/v2` | Generate laporan PDF |
| Migration | File SQL manual + `golang-migrate` | Kontrol penuh atas skema |

**Prinsip dependency:** tambahkan library hanya jika stdlib benar-benar tidak cukup. Setiap dependency baru harus bisa dijelaskan alasannya.

---

# FASE 0 — Bootstrap & Deployment Awal

> **Tujuan fase:** memastikan pipeline "kode lokal → live URL" berfungsi **sebelum** menulis satu baris pun business logic. Ini mencegah kejutan deployment di akhir project.

---

### BE-0.1 — Inisialisasi Repository

**Tujuan:** Repository backend berdiri sendiri dengan konfigurasi dasar yang benar sejak awal.

**Langkah:**
1. Buat repository GitHub baru bernama `llunara-api` (public, agar GitHub Actions gratis)
2. Inisialisasi Go module: `go mod init github.com/<username>/llunara-api`
3. Buat `.gitignore` yang mencakup: `.env`, `bin/`, `tmp/`, `*.log`, `coverage.out`
4. Buat `README.md` awal berisi deskripsi satu paragraf dan link ke `PRD.md`
5. Buat `LICENSE` (MIT)

**Output:** Repository kosong yang siap dikembangkan.

**Selesai jika:** `go mod tidy` berjalan tanpa error dan repository sudah ter-push ke GitHub.

---

### BE-0.2 — Struktur Folder

**Tujuan:** Menetapkan batas antar layer sejak awal, agar tidak terjadi pencampuran tanggung jawab di kemudian hari.

**Langkah:**

Buat struktur folder berikut (gunakan file `.gitkeep` untuk folder kosong):

```
cmd/api/
internal/config/
internal/handler/
internal/service/
internal/repository/
internal/model/
internal/middleware/
internal/pkg/apierror/
internal/pkg/validator/
migrations/
docs/
```

**Aturan arah dependency yang wajib dipatuhi:**

```
handler  →  service  →  repository  →  database
   ↓          ↓             ↓
        model (boleh diakses semua layer)
```

- `handler` **tidak boleh** mengakses `repository` secara langsung
- `repository` **tidak boleh** mengimpor `service` atau `handler`
- `model` **tidak boleh** mengimpor layer manapun

**Output:** Kerangka folder lengkap.

**Selesai jika:** Struktur folder sesuai dan aturan dependency didokumentasikan di `docs/ARCHITECTURE.md`.

---

### BE-0.3 — Config Loader

**Tujuan:** Seluruh konfigurasi dibaca dari environment variable, tidak ada nilai yang di-hardcode. Ini prasyarat agar aplikasi bisa di-deploy.

**Langkah:**
1. Buat `internal/config/config.go` dengan struct `Config` yang memuat:
   - `Port` (default `8080`)
   - `Env` (`development` / `production`)
   - `DatabaseURL`
   - `SupabaseJWTSecret`
   - `SupabaseURL`
   - `AllowedOrigins`
2. Buat fungsi `Load() (*Config, error)` yang membaca dari environment variable
3. **Wajib:** aplikasi gagal start (fatal) jika variable esensial kosong. Jangan pakai fallback diam-diam
4. Buat `.env.example` berisi seluruh key tanpa nilai asli

**Output:** `internal/config/config.go`, `.env.example`

**Selesai jika:** Menjalankan aplikasi tanpa `DATABASE_URL` menghasilkan pesan error yang jelas dan aplikasi berhenti.

---

### BE-0.4 — Router & Health Endpoint

**Tujuan:** Server HTTP dapat menerima request. Ini fondasi seluruh endpoint berikutnya.

**Langkah:**
1. Setup chi router di `cmd/api/main.go`
2. Pasang middleware bawaan chi: `RequestID`, `RealIP`, `Recoverer`
3. Buat route `GET /health` yang mengembalikan:
   ```json
   { "status": "ok", "version": "0.1.0", "timestamp": "..." }
   ```
4. Buat sub-router `/api/v1` sebagai wadah seluruh endpoint bisnis
5. Implementasi graceful shutdown menggunakan `signal.NotifyContext` dan `server.Shutdown` dengan timeout 10 detik

**Output:** Server yang bisa dijalankan lokal.

**Selesai jika:** `curl localhost:8080/health` mengembalikan status 200, dan `Ctrl+C` menghentikan server tanpa memutus request yang sedang berjalan.

---

### BE-0.5 — Dockerfile & Makefile

**Tujuan:** Build reproducible dan perintah development yang konsisten.

**Langkah:**
1. Buat `Dockerfile` multi-stage:
   - Stage 1: base image Go, `go mod download`, lalu compile dengan `CGO_ENABLED=0`
   - Stage 2: base image `alpine` atau `distroless`, salin binary saja
   - Expose port dari environment variable, **bukan** hardcoded
2. Buat `Makefile` dengan target:
   - `run` — jalankan lokal
   - `build` — compile binary
   - `test` — jalankan seluruh test
   - `lint` — jalankan `go vet` dan `gofmt -l`
   - `migrate-up` / `migrate-down`

**Output:** `Dockerfile`, `Makefile`

**Selesai jika:** Image Docker berhasil di-build dan container berjalan, `/health` dapat diakses dari luar container.

---

### BE-0.6 — Deploy ke Vercel

> **Catatan (25 Juli 2026):** Task ini ditulis ulang dua kali. Semula Render (dibatalkan: kini wajib kartu kredit, lihat ADR-004), lalu dicoba Zeabur (dibatalkan: dashboard live-nya ternyata mengharuskan beli/bawa server, lihat ADR-005). Sekarang memakai Vercel Hobby plan dengan Go Framework Preset — lihat ADR-006 di `PRD.md` Bagian 10.

**Tujuan:** Membuktikan pipeline deployment berfungsi sejak awal, bukan di akhir project.

**Langkah:**
1. Daftar/login di [vercel.com](https://vercel.com) menggunakan GitHub. Tidak perlu kartu kredit untuk Hobby plan — kalau di titik manapun diminta, berhenti dan konfirmasi dulu sebelum lanjut
2. Pastikan `vercel.json` ada di root repo berisi `{"framework": "go"}` (sudah disiapkan)
3. Di dashboard Vercel, klik **Add New → Project** → import repository `LLUNARA-BE` dari GitHub
4. Vercel akan mendeteksi Go Framework Preset lewat `go.mod` + entrypoint `cmd/api/main.go` — biarkan default, jangan ubah build command
5. Set environment variable di tab **Environment Variables** project (nilai database akan diisi setelah BE-1.1): `DATABASE_URL`, `SUPABASE_URL`, `SUPABASE_JWT_SECRET`, `ENV=production`
6. Aplikasi sudah listen pada `os.Getenv("PORT")` dengan default `8080` — sesuai syarat Go Framework Preset, tidak perlu perubahan kode
7. Klik **Deploy**. Auto-deploy dari branch `main` aktif otomatis untuk setiap push berikutnya, tanpa setup tambahan
8. Domain publik gratis otomatis dibuat, format `https://<nama-project>.vercel.app`

**Output:** URL publik, format `https://<nama-project>.vercel.app`

**Selesai jika:**
- `curl https://<nama-project>.vercel.app/health` mengembalikan 200
- Push ke `main` memicu deploy otomatis
- Tidak ada metode pembayaran terdaftar di akun

**Blocking:** FE-0.7

---

# FASE 1 — Database & Skema

> **Tujuan fase:** struktur data final dan aman. Perubahan skema setelah fase ini akan mahal, jadi kerjakan dengan teliti.

---

### BE-1.1 — Setup Supabase Project

**Tujuan:** Database Postgres siap pakai dengan kredensial yang aman.

**Langkah:**
1. Buat project Supabase baru. Pilih region terdekat (Singapore untuk Indonesia)
2. Simpan kredensial berikut di tempat aman:
   - `DATABASE_URL` (connection string, gunakan **connection pooler**, bukan direct connection — hosting backend free tier punya keterbatasan koneksi)
   - `SUPABASE_URL`
   - `SUPABASE_PUBLISHABLE_KEY` (nama baru Supabase untuk apa yang dulu disebut `anon key`) → nanti dipakai Frontend
   - `SUPABASE_SECRET_KEY` (nama baru Supabase untuk `service_role key`) → **hanya** untuk backend
   - `SUPABASE_JWT_SECRET` → untuk verifikasi token. **Catatan (25 Juli 2026):** project Supabase baru default ke JWT signing key asimetris, bukan shared secret. Cari & aktifkan **"Legacy JWT Secret"** di Project Settings → API → JWT Settings untuk tetap dapat nilai ini — dengan begitu BE-2.2 tidak perlu diubah ke verifikasi berbasis JWKS
3. Masukkan nilai-nilai ini ke environment variable Vercel
4. Masukkan ke `.env` lokal (pastikan file ini ada di `.gitignore`)

**Output:** Database aktif dan kredensial tersimpan.

**Selesai jika:** Koneksi ke database berhasil dari lokal, dan tidak ada satu pun key yang ter-commit ke repository.

> **Peringatan keamanan:** `SUPABASE_SECRET_KEY` mem-bypass seluruh Row Level Security. Key ini tidak boleh muncul di kode frontend, log, pesan error, maupun screenshot dokumentasi.

---

### BE-1.2 — Migration 001: Skema Inti

**Tujuan:** Seluruh tabel terbentuk sesuai desain di PRD Bagian 6.

**Langkah:**

Buat `migrations/001_init_schema.sql` yang membuat tabel berikut, mengikuti definisi kolom di PRD Bagian 6.2:

1. `profiles`
2. `cycles`
3. `symptoms`
4. `daily_logs`
5. `daily_log_symptoms`
6. `wellness_logs`
7. `reminders`
8. `sharing_permissions`

**Ketentuan wajib:**
- Buat tipe enum lebih dulu: `flow_intensity`, `reminder_type`, `sharing_status`
- Setiap tabel punya `created_at timestamptz default now()`
- Tabel yang bisa diubah juga punya `updated_at`
- Constraint unik:
  - `cycles`: `UNIQUE(user_id, start_date)`
  - `daily_logs`: `UNIQUE(user_id, date)`
  - `wellness_logs`: `UNIQUE(user_id, date)`
- Foreign key ke `auth.users(id)` dengan `ON DELETE CASCADE`
- Index pada kolom yang sering di-query: `(user_id, date)` dan `(user_id, start_date)`
- Trigger untuk memperbarui `updated_at` secara otomatis

**Output:** `migrations/001_init_schema.sql`

**Selesai jika:** Migration berjalan tanpa error, seluruh tabel muncul di Supabase Table Editor, dan constraint dapat diverifikasi (mencoba insert duplikat pada `(user_id, date)` menghasilkan error).

---

### BE-1.3 — Migration 002: Row Level Security

**Tujuan:** Tidak ada satu pun user yang bisa membaca atau mengubah data user lain, bahkan jika terjadi bug di aplikasi.

**Langkah:**

Buat `migrations/002_rls_policies.sql`:

1. Aktifkan RLS pada **seluruh** tabel tanpa terkecuali
2. Untuk tiap tabel yang punya `user_id`, buat 4 policy: `SELECT`, `INSERT`, `UPDATE`, `DELETE`, masing-masing dengan kondisi `auth.uid() = user_id`
3. Tabel `symptoms` perlu penanganan khusus: preset milik sistem (`user_id IS NULL`) boleh dibaca semua orang, tapi hanya pemilik yang boleh mengubah tag kustomnya
   ```sql
   create policy "read_presets_and_own_symptoms"
     on symptoms for select
     using (user_id is null or auth.uid() = user_id);
   ```
4. `daily_log_symptoms` tidak punya `user_id` langsung, jadi policy-nya mengecek kepemilikan melalui join ke `daily_logs`

**Output:** `migrations/002_rls_policies.sql`

**Selesai jika:** Diuji dengan dua akun berbeda — akun A tidak dapat melihat data akun B melalui Supabase REST API sama sekali.

> **Verifikasi wajib:** buat dua user uji, isi data pada masing-masing, lalu coba query silang menggunakan anon key. Task ini tidak boleh dilewati.

---

### BE-1.4 — Migration 003: Seed Data Gejala

**Tujuan:** Daftar gejala dan mood preset tersedia sebagai pilihan default.

**Langkah:**
1. Buat `migrations/003_seed_symptoms.sql`
2. Insert gejala preset dengan `user_id = NULL` dan `is_custom = false`, sesuai daftar di PRD Bagian 3.3
3. Beri kategori: `physical` atau `emotional`
4. Gunakan `ON CONFLICT DO NOTHING` agar migration bersifat idempoten

**Output:** `migrations/003_seed_symptoms.sql`

**Selesai jika:** Query `SELECT * FROM symptoms WHERE user_id IS NULL` mengembalikan seluruh gejala preset.

**Blocking:** FE-3.4

---

### BE-1.5 — Koneksi Database & Repository Dasar

**Tujuan:** Layer akses data yang aman, efisien, dan konsisten.

**Langkah:**
1. Buat `internal/repository/postgres.go`
2. Inisialisasi `pgxpool.Pool` dengan konfigurasi hemat resource (hosting free tier punya RAM terbatas):
   - `MaxConns`: 5
   - `MinConns`: 1
   - `MaxConnIdleTime`: 5 menit
3. Buat health check koneksi yang dipanggil saat startup
4. Buat interface `Repository` sebagai kontrak, agar service tidak bergantung pada implementasi konkret
5. **Wajib:** seluruh query menggunakan parameterized statement. Tidak ada string concatenation dalam pembuatan SQL

**Output:** `internal/repository/postgres.go`

**Selesai jika:** Aplikasi berhasil connect saat startup, dan gagal start dengan pesan jelas jika database tidak dapat dijangkau.

---

### BE-1.6 — GitHub Action Keep-Alive

**Tujuan:** Mencegah Supabase project ter-pause karena tidak aktif selama 7 hari.

**Langkah:**
1. Buat `.github/workflows/keep-alive.yml`
2. Jadwalkan cron setiap 3 hari
3. Isi job: melakukan HTTP request sederhana ke endpoint `/health` backend, yang di dalamnya melakukan satu query ringan ke database (`SELECT 1`)
4. Tambahkan `workflow_dispatch` agar bisa dijalankan manual

**Output:** `.github/workflows/keep-alive.yml`

**Selesai jika:** Workflow berhasil dijalankan manual dan mengembalikan status sukses.

> **Catatan:** `/health` harus benar-benar menyentuh database, bukan sekadar mengembalikan JSON statis. Kalau tidak, Supabase tetap menganggap project idle.

---

# FASE 2 — Autentikasi & Fondasi Keamanan

> **Tujuan fase:** setiap request terverifikasi identitasnya. Tidak ada endpoint bisnis yang boleh dibangun sebelum fase ini selesai.

---

### BE-2.1 — Package Error Terstandardisasi

**Tujuan:** Seluruh error API punya bentuk yang konsisten, sehingga frontend bisa menanganinya secara seragam.

**Langkah:**
1. Buat `internal/pkg/apierror/apierror.go`
2. Definisikan struct:
   ```go
   type APIError struct {
       Code       string         `json:"code"`
       Message    string         `json:"message"`
       Details    map[string]any `json:"details,omitempty"`
       HTTPStatus int            `json:"-"`
   }
   ```
3. Buat konstruktor untuk tiap kode error di PRD Bagian 7.3
4. Buat helper `WriteError(w http.ResponseWriter, err error)` yang menulis response JSON
5. **Wajib:** pesan error untuk klien tidak boleh membocorkan detail internal. Error asli dicatat di log; klien hanya menerima pesan yang aman

**Output:** `internal/pkg/apierror/apierror.go`

**Selesai jika:** Seluruh error response mengikuti format `{ "error": { "code", "message", "details" } }`.

---

### BE-2.2 — Middleware Verifikasi JWT

**Tujuan:** Backend hanya melayani request dari user yang identitasnya terbukti secara kriptografis.

**Langkah:**
1. Buat `internal/middleware/auth.go`
2. Ambil token dari header `Authorization: Bearer <token>`
3. Verifikasi signature menggunakan `SUPABASE_JWT_SECRET` (algoritma HS256)
4. Validasi claim: `exp` (belum kedaluwarsa), `aud` (bernilai `authenticated`)
5. Ekstrak `sub` sebagai `user_id`
6. Simpan `user_id` ke request context menggunakan **typed key**, bukan string biasa:
   ```go
   type contextKey string
   const UserIDKey contextKey = "user_id"
   ```
7. Buat helper `GetUserID(ctx context.Context) (uuid.UUID, error)`
8. Tolak dengan 401 `UNAUTHORIZED` jika token tidak ada, tidak valid, atau kedaluwarsa

**Output:** `internal/middleware/auth.go`

**Selesai jika:**
- Request tanpa token → 401
- Request dengan token asal-asalan → 401
- Request dengan token valid → lolos, dan `user_id` tersedia di context

> **Aturan keamanan absolut:** `user_id` **hanya** boleh berasal dari JWT yang sudah diverifikasi. Backend tidak boleh menerima `user_id` dari body request, query parameter, maupun header kustom — dalam kondisi apapun.

**Blocking:** FE-1.5

---

### BE-2.3 — Endpoint Uji Terproteksi

**Tujuan:** Membuktikan alur autentikasi end-to-end berfungsi sebelum membangun fitur.

**Langkah:**
1. Buat `GET /api/v1/me` yang mengembalikan `user_id` dari context
2. Pasang middleware auth pada sub-router `/api/v1`
3. Uji menggunakan token asli hasil login dari Supabase

**Output:** Endpoint `/api/v1/me`

**Selesai jika:** Endpoint mengembalikan `user_id` yang sesuai dengan akun yang login.

**Blocking:** FE-1.4

---

### BE-2.4 — Layer Validasi Input

**Tujuan:** Data yang tidak valid ditolak di gerbang terluar, sebelum menyentuh business logic.

**Langkah:**
1. Buat `internal/pkg/validator/validator.go` sebagai pembungkus `go-playground/validator`
2. Buat helper `DecodeAndValidate(r *http.Request, dst any) error` yang melakukan decode JSON sekaligus validasi
3. Terjemahkan error validasi menjadi `VALIDATION_ERROR` (HTTP 422) dengan detail per field
4. Batasi ukuran body request (maksimal 1 MB) menggunakan `http.MaxBytesReader`

**Output:** `internal/pkg/validator/validator.go`

**Selesai jika:** Request dengan field wajib yang kosong menghasilkan 422 beserta nama field yang bermasalah.

---

# FASE 3 — Core Tracking API

> **Tujuan fase:** seluruh operasi pencatatan berfungsi. Setelah fase ini, aplikasi sudah punya nilai guna nyata.

---

### BE-3.1 — Domain Model

**Tujuan:** Representasi data yang jelas dan terpisah dari struktur tabel maupun bentuk JSON.

**Langkah:**
1. Buat file di `internal/model/`: `cycle.go`, `daily_log.go`, `symptom.go`, `wellness.go`, `insight.go`
2. Untuk setiap entitas, definisikan tiga bentuk terpisah:
   - **Domain model** — struktur murni, tanpa tag JSON
   - **Request DTO** — dengan tag validasi
   - **Response DTO** — dengan tag JSON
3. Buat fungsi konversi antar bentuk tersebut

**Alasan pemisahan ini:** perubahan pada struktur tabel tidak otomatis merusak kontrak API, dan sebaliknya.

**Output:** File-file di `internal/model/`

**Selesai jika:** Seluruh entitas terdefinisi dan kompilasi berhasil.

---

### BE-3.2 — Cycle Repository

**Tujuan:** Operasi database untuk siklus, terisolasi dari business logic.

**Langkah:**

Implementasi method berikut di `internal/repository/cycle_repository.go`:

| Method | Kegunaan |
|---|---|
| `Create(ctx, cycle) (*Cycle, error)` | Simpan siklus baru |
| `GetByID(ctx, userID, cycleID)` | Ambil satu siklus |
| `ListByUser(ctx, userID, limit)` | Ambil riwayat, urut `start_date` menurun |
| `GetLatest(ctx, userID)` | Ambil siklus terbaru |
| `FindOverlapping(ctx, userID, startDate)` | Deteksi tumpang tindih |
| `Update(ctx, cycle)` | Perbarui siklus |
| `Delete(ctx, userID, cycleID)` | Hapus siklus |

**Ketentuan wajib:**
- **Setiap** query menyertakan filter `user_id`. Karena backend memakai service role key yang mem-bypass RLS, filter ini adalah satu-satunya lapisan pengaman
- `ListByUser` dibatasi maksimal 100 baris untuk mencegah query berat

**Output:** `internal/repository/cycle_repository.go`

**Selesai jika:** Seluruh method punya unit test, dan tidak ada satu pun query tanpa filter `user_id`.

---

### BE-3.3 — Cycle Service

**Tujuan:** Aturan bisnis pencatatan siklus. Ini bagian yang tidak bisa digantikan oleh Supabase auto-API — alasan utama backend ini ada.

**Langkah:**

Implementasi di `internal/service/cycle_service.go`:

1. **`StartCycle(ctx, userID, startDate)`**
   - Tolak jika `startDate` di masa depan → `VALIDATION_ERROR`
   - Cek tumpang tindih dengan siklus yang sudah ada → `CYCLE_OVERLAP` (409)
   - Jika ada siklus sebelumnya yang masih terbuka, tutup siklus tersebut dan hitung `cycle_length` = selisih hari antara `start_date` lama dan baru
   - Tandai `is_outlier = true` jika `cycle_length` < 21 atau > 45
   - Simpan siklus baru
   - Panggil ulang kalkulasi prediksi

2. **`EndCycle(ctx, userID, cycleID, endDate)`**
   - Validasi `endDate` tidak mendahului `startDate`
   - Validasi durasi wajar (1–14 hari); di luar itu beri peringatan tapi tetap izinkan
   - Hitung dan simpan `period_length`

3. **`DeleteCycle(ctx, userID, cycleID)`**
   - Hapus siklus, lalu hitung ulang `cycle_length` siklus di sekitarnya

**Output:** `internal/service/cycle_service.go`

**Selesai jika:** Seluruh aturan di atas punya unit test, termasuk kasus batas (siklus pertama, siklus tumpang tindih, penghapusan siklus di tengah riwayat).

---

### BE-3.4 — Cycle Handler

**Tujuan:** Mengekspos service siklus melalui HTTP.

**Langkah:**

Implementasi endpoint berikut di `internal/handler/cycle_handler.go`:

| Method | Path | Fungsi |
|---|---|---|
| `POST` | `/api/v1/cycles` | Catat awal menstruasi |
| `PATCH` | `/api/v1/cycles/{id}` | Perbarui siklus |
| `DELETE` | `/api/v1/cycles/{id}` | Hapus siklus |
| `GET` | `/api/v1/cycles` | Riwayat siklus |

**Ketentuan:**
- Ambil `user_id` dari context, tidak pernah dari body
- Validasi format UUID pada path parameter
- Response 201 untuk create, 200 untuk update, 204 untuk delete

**Output:** `internal/handler/cycle_handler.go`

**Selesai jika:** Seluruh endpoint dapat diuji manual dan mengembalikan status code yang tepat.

**Blocking:** FE-3.2

---

### BE-3.5 — Daily Log (Repository, Service, Handler)

**Tujuan:** Pencatatan kondisi harian, termasuk relasi many-to-many dengan gejala.

**Langkah:**

1. **Repository** — implementasi:
   - `Upsert(ctx, log)` menggunakan `INSERT ... ON CONFLICT (user_id, date) DO UPDATE`
   - `GetByDate(ctx, userID, date)`
   - `ListByRange(ctx, userID, from, to)`
   - `Delete(ctx, userID, date)`
   - `ReplaceSymptoms(ctx, logID, symptomIDs)` — hapus relasi lama, masukkan yang baru

2. **Service** — implementasi:
   - Tolak log pada tanggal di masa depan
   - Kaitkan log ke `cycle_id` yang sesuai secara otomatis berdasarkan tanggal
   - Validasi seluruh `symptom_id` benar-benar dimiliki user atau merupakan preset sistem
   - Batasi `notes` maksimal 500 karakter

3. **Handler** — implementasi:
   - `POST /api/v1/daily-logs` (upsert)
   - `GET /api/v1/daily-logs?from=&to=`
   - `DELETE /api/v1/daily-logs/{date}`

**Ketentuan penting:** operasi upsert log beserta penggantian gejalanya harus berada dalam **satu transaksi database**. Kalau salah satu gagal, seluruhnya dibatalkan.

**Output:** Tiga file di layer masing-masing.

**Selesai jika:** Menyimpan log dua kali pada tanggal yang sama menghasilkan pembaruan, bukan duplikasi, dan daftar gejala tergantikan dengan benar.

**Blocking:** FE-3.3

---

### BE-3.6 — Symptom Management

**Tujuan:** User dapat menambahkan tag gejala sendiri di luar daftar preset.

**Langkah:**
1. `GET /api/v1/symptoms` — kembalikan preset sistem digabung dengan tag kustom milik user
2. `POST /api/v1/symptoms` — buat tag kustom
   - Tolak jika namanya sudah ada (case-insensitive), baik di preset maupun tag milik user sendiri
   - Batasi maksimal 30 tag kustom per user
3. `DELETE /api/v1/symptoms/{id}` — hapus tag kustom
   - Tolak jika mencoba menghapus preset sistem
   - Relasi di `daily_log_symptoms` ikut terhapus melalui cascade

**Output:** `internal/handler/symptom_handler.go` beserta service dan repository-nya.

**Selesai jika:** Tag kustom dapat dibuat, muncul di daftar, dan preset sistem tidak dapat dihapus.

**Blocking:** FE-3.4

---

# FASE 4 — Prediksi Siklus

> **Tujuan fase:** algoritma inti aplikasi. Bagian ini yang paling layak ditonjolkan di portofolio, jadi kerjakan dengan test coverage yang baik.

---

### BE-4.1 — Algoritma Prediksi

**Tujuan:** Menghasilkan prediksi yang dapat dipertanggungjawabkan dan mudah diuji.

**Langkah:**

Buat `internal/service/prediction_service.go`. **Tulis seluruh kalkulasi sebagai pure function** — menerima slice `[]Cycle`, mengembalikan `Prediction`, tanpa akses database di dalamnya. Ini membuatnya mudah di-unit-test.

Implementasi bertahap:

1. **`calculateAverageCycleLength(cycles []Cycle) int`**
   - Ambil maksimal 6 siklus terakhir yang sudah punya `cycle_length`
   - Kecualikan yang `is_outlier = true`
   - Jika data tersisa kurang dari 2, kembalikan `default_cycle_length` dari profil user
   - Kembalikan rata-rata dibulatkan

2. **`calculateAveragePeriodLength(cycles []Cycle) int`**
   - Logika serupa, memakai `period_length`

3. **`predictNextPeriod(lastCycle Cycle, avgLength int) (start, end time.Time)`**
   - `start` = `lastCycle.StartDate` + `avgLength` hari
   - `end` = `start` + `avgPeriodLength` - 1 hari

4. **`calculateOvulation(nextStart time.Time, avgLength int) time.Time`**
   - Ovulasi = `nextStart` - 14 hari (fase luteal dianggap konstan)

5. **`calculateFertileWindow(ovulation time.Time) (start, end time.Time)`**
   - `start` = ovulasi - 5 hari
   - `end` = ovulasi + 1 hari

6. **`determineCurrentPhase(today time.Time, lastCycle Cycle, avg int) Phase`**
   - `menstrual` — dalam rentang periode berjalan
   - `follicular` — setelah menstruasi sampai sebelum jendela subur
   - `ovulation` — dalam jendela subur
   - `luteal` — setelah ovulasi sampai menstruasi berikutnya

7. **`calculateConfidence(cycles []Cycle) Confidence`**
   - `low` — kurang dari 3 siklus
   - `medium` — 3 sampai 5 siklus
   - `high` — 6 siklus atau lebih **dan** standar deviasi panjang siklus ≤ 5 hari

**Output:** `internal/service/prediction_service.go`

**Selesai jika:** Seluruh fungsi punya unit test dengan minimal skenario berikut:
- User baru tanpa riwayat sama sekali
- Satu siklus tercatat
- Enam siklus teratur
- Enam siklus dengan variasi tinggi
- Riwayat yang mengandung outlier
- Siklus yang melewati pergantian tahun

---

### BE-4.2 — Endpoint Prediksi

**Tujuan:** Frontend dapat mengambil hasil prediksi.

**Langkah:**
1. Buat `GET /api/v1/cycles/prediction`
2. Ambil riwayat siklus user, jalankan kalkulasi, kembalikan sesuai format di PRD Bagian 7.2
3. Jika user belum punya siklus sama sekali, kembalikan 200 dengan `confidence: "low"` dan field prediksi bernilai null — **bukan** error
4. Sertakan `day_of_cycle` dan `current_phase` untuk kebutuhan tampilan dashboard

**Output:** Endpoint `/api/v1/cycles/prediction`

**Selesai jika:** Endpoint mengembalikan data yang benar untuk user berpengalaman maupun user yang benar-benar baru.

**Blocking:** FE-4.1, FE-4.2

---

### BE-4.3 — Kalkulasi Ulang Otomatis

**Tujuan:** Prediksi selalu mencerminkan data terbaru tanpa perlu aksi manual dari user.

**Langkah:**
1. Panggil ulang kalkulasi prediksi setiap kali terjadi operasi tulis pada siklus (create, update, delete)
2. Sertakan objek prediksi terbaru dalam response endpoint tersebut

**Alasan:** frontend perlu menjadwalkan ulang notifikasi lokal segera setelah prediksi berubah. Menyertakan prediksi dalam response menghemat satu round-trip.

**Output:** Modifikasi pada cycle service dan handler.

**Selesai jika:** Response `POST /api/v1/cycles` menyertakan field `prediction` yang sudah diperbarui.

**Blocking:** FE-5.3

---

# FASE 5 — Insight & Analytics

---

### BE-5.1 — Ringkasan Siklus

**Tujuan:** Statistik dasar yang menggambarkan pola siklus user.

**Langkah:**
1. Buat `GET /api/v1/insights/summary`
2. Hitung dan kembalikan:
   - Rata-rata panjang siklus
   - Siklus terpendek dan terpanjang
   - Rata-rata durasi menstruasi
   - Total siklus tercatat
   - Tingkat keteraturan (`regular` jika standar deviasi ≤ 3 hari, `irregular` jika > 7 hari, `moderate` di antaranya)
   - Deret data untuk grafik tren panjang siklus
3. Jika data kurang dari 2 siklus, kembalikan 200 dengan `has_sufficient_data: false` beserta pesan yang menjelaskan berapa data lagi yang dibutuhkan

**Output:** `internal/service/insight_service.go`, `internal/handler/insight_handler.go`

**Selesai jika:** Endpoint berfungsi dan menangani kondisi data minim tanpa error.

**Blocking:** FE-6.2

---

### BE-5.2 — Analisis Gejala

**Tujuan:** Menunjukkan hubungan antara gejala dan fase siklus.

**Langkah:**
1. Buat `GET /api/v1/insights/symptoms?months=6`
2. Kembalikan:
   - Frekuensi tiap gejala, diurutkan dari yang paling sering
   - Distribusi gejala per fase siklus (menstrual, follicular, ovulation, luteal)
   - Hari siklus yang paling sering memunculkan gejala tertentu
3. Lakukan agregasi di level SQL sebisa mungkin, agar hemat memori
4. Sertakan `sample_size` pada tiap hasil, agar frontend bisa menampilkan konteks

**Ketentuan:** hasil bersifat **deskriptif**, bukan prediktif atau preskriptif. Jangan gunakan kata seperti "menyebabkan" atau "disarankan".

**Output:** Endpoint dan service terkait.

**Selesai jika:** Endpoint mengembalikan data yang benar untuk rentang 6 bulan terakhir.

**Blocking:** FE-6.3

---

### BE-5.3 — Pola Mood

**Tujuan:** Distribusi mood berdasarkan fase siklus.

**Langkah:**
1. Buat `GET /api/v1/insights/mood?months=6`
2. Kembalikan distribusi tiap mood per fase siklus, dalam bentuk persentase dan jumlah absolut
3. Sertakan mood yang paling dominan pada tiap fase

**Output:** Endpoint dan service terkait.

**Selesai jika:** Endpoint berfungsi dan menangani kondisi data kosong dengan baik.

**Blocking:** FE-6.3

---

# FASE 6 — Wellness, Export & Manajemen Akun

---

### BE-6.1 — Wellness Endpoints

**Tujuan:** Pencatatan metrik gaya hidup harian.

**Langkah:**
1. `POST /api/v1/wellness` — upsert berdasarkan `(user_id, date)`
2. `GET /api/v1/wellness?from=&to=`
3. Validasi rentang nilai yang wajar:
   - Air minum: 0–30 gelas
   - Tidur: 0–24 jam
   - Berat badan: 20–300 kg
4. Seluruh field bersifat opsional — user boleh mengisi sebagian saja

**Ketentuan:** jangan menetapkan target atau nilai anjuran apapun di sisi server. Server hanya menyimpan angka; interpretasi diserahkan sepenuhnya kepada user.

**Output:** Endpoint wellness beserta service dan repository.

**Selesai jika:** Data wellness dapat disimpan sebagian dan diambil kembali dengan benar.

**Blocking:** FE-7.1

---

### BE-6.2 — Export CSV

**Tujuan:** User dapat mengeluarkan data mentahnya.

**Langkah:**
1. Buat `POST /api/v1/export?format=csv&from=&to=`
2. Gunakan `encoding/csv` dari stdlib
3. Kolom: tanggal, hari siklus, fase, intensitas flow, mood, daftar gejala, catatan, data wellness
4. Kembalikan sebagai file dengan header `Content-Disposition: attachment`
5. Batasi rentang maksimal 2 tahun untuk mencegah beban berlebih

**Output:** `internal/service/export_service.go`

**Selesai jika:** File CSV yang dihasilkan dapat dibuka di spreadsheet dan isinya lengkap.

---

### BE-6.3 — Export PDF

**Tujuan:** Laporan yang layak dibawa ke tenaga medis.

**Langkah:**
1. Tambahkan `format=pdf` pada endpoint export
2. Susun laporan dengan struktur:
   - Header: nama aplikasi, rentang tanggal, tanggal pembuatan
   - Ringkasan: rata-rata siklus, keteraturan, jumlah siklus
   - Tabel riwayat siklus
   - Gejala yang paling sering muncul
   - Footer: disclaimer medis sesuai PRD Bagian 14
3. Jaga agar tetap ringkas — maksimal 3 halaman untuk rentang 6 bulan

**Output:** Fungsi generate PDF di `export_service.go`

**Selesai jika:** PDF terbentuk dengan tata letak rapi dan disclaimer tercantum.

**Blocking:** FE-7.2

---

### BE-6.4 — Penghapusan Akun

**Tujuan:** User punya kendali penuh untuk menghapus seluruh jejak datanya.

**Langkah:**
1. Buat `DELETE /api/v1/account`
2. Hapus seluruh data user secara berurutan dalam satu transaksi
3. Hapus user dari `auth.users` melalui Supabase Admin API
4. Kembalikan 204 setelah seluruhnya berhasil

**Ketentuan:** ini adalah **hard delete**, bukan soft delete. Untuk aplikasi data kesehatan, menyimpan data setelah user meminta penghapusan adalah praktik yang tidak dapat diterima.

**Output:** `internal/handler/account_handler.go`

**Selesai jika:** Setelah penghapusan, tidak ada satu baris pun tersisa di seluruh tabel untuk `user_id` tersebut.

**Blocking:** FE-7.5

---

# FASE 7 — Hardening & Dokumentasi

---

### BE-7.1 — Rate Limiting

**Tujuan:** Perlindungan dasar dari penyalahgunaan dan pemborosan kuota hosting free tier.

**Langkah:**
1. Buat `internal/middleware/rate_limit.go`
2. Gunakan pendekatan token bucket berbasis memori, dengan kunci `user_id`
3. Batas: 100 request per menit per user
4. Kembalikan 429 dengan header `Retry-After` saat batas terlampaui

**Catatan:** rate limiter berbasis memori tetap dipakai sebagai perlindungan dasar meski Vercel (ADR-006) bisa menjalankan lebih dari satu instance saat concurrency scaling — akurasi lintas instance tidak 100% terjamin, tapi ini dampaknya dapat diabaikan untuk skala 1–2 pengguna. Jangan menambahkan Redis — itu berarti layanan berbayar.

**Output:** `internal/middleware/rate_limit.go`

**Selesai jika:** Request ke-101 dalam satu menit menerima status 429.

---

### BE-7.2 — Structured Logging

**Tujuan:** Log yang dapat ditelusuri saat terjadi masalah di production.

**Langkah:**
1. Setup `log/slog` dengan format JSON pada mode production, dan format teks pada mode development
2. Buat middleware logging yang mencatat: method, path, status, durasi, request ID
3. **Wajib:** jangan pernah mencatat token, password, isi catatan harian, maupun detail gejala. Cukup catat `user_id` untuk keperluan penelusuran
4. Catat seluruh error di level `ERROR` beserta stack trace, sementara klien tetap menerima pesan yang aman

**Output:** `internal/middleware/logger.go`

**Selesai jika:** Log terbaca di dashboard Vercel (Runtime Logs) dan tidak mengandung satu pun data sensitif.

---

### BE-7.3 — CORS

**Tujuan:** Mengizinkan akses dari aplikasi mobile secara terkendali.

**Langkah:**
1. Buat `internal/middleware/cors.go`
2. Baca daftar origin yang diizinkan dari environment variable
3. Pada mode development, izinkan origin lokal Expo
4. **Jangan** gunakan wildcard `*` pada mode production

**Output:** `internal/middleware/cors.go`

**Selesai jika:** Request dari origin yang tidak terdaftar ditolak pada mode production.

---

### BE-7.4 — CI Pipeline

**Tujuan:** Kualitas kode terjaga secara otomatis pada setiap perubahan.

**Langkah:**
1. Buat `.github/workflows/ci.yml`
2. Jalankan pada setiap push dan pull request:
   - `gofmt -l .` (gagal jika ada file yang belum terformat)
   - `go vet ./...`
   - `go test ./... -race -coverprofile=coverage.out`
   - `go build ./...`
3. Tampilkan angka coverage pada ringkasan job

**Output:** `.github/workflows/ci.yml`

**Selesai jika:** CI berjalan otomatis dan berstatus hijau pada branch `main`.

---

### BE-7.5 — Dokumentasi

**Tujuan:** Repository dapat dipahami oleh orang lain tanpa penjelasan lisan.

**Langkah:**
1. `README.md` — deskripsi project, tech stack, cara menjalankan lokal, variabel environment, cara deploy
2. `docs/ARCHITECTURE.md` — diagram layer, aturan arah dependency, alasan pemilihan chi, ringkasan ADR
3. `docs/API.md` — seluruh endpoint beserta contoh request dan response
4. Sertakan diagram arsitektur dari PRD Bagian 5.2
5. Cantumkan link ke repository frontend

**Output:** Tiga file dokumentasi.

**Selesai jika:** Orang yang belum pernah melihat project ini bisa menjalankannya secara lokal hanya dengan mengikuti README.

---

## Ringkasan Urutan Eksekusi

```
FASE 0  Bootstrap        BE-0.1 → 0.2 → 0.3 → 0.4 → 0.5 → 0.6
FASE 1  Database         BE-1.1 → 1.2 → 1.3 → 1.4 → 1.5 → 1.6
FASE 2  Auth             BE-2.1 → 2.2 → 2.3 → 2.4
FASE 3  Core Tracking    BE-3.1 → 3.2 → 3.3 → 3.4 → 3.5 → 3.6
FASE 4  Prediksi         BE-4.1 → 4.2 → 4.3
FASE 5  Insight          BE-5.1 → 5.2 → 5.3
FASE 6  Export & Akun    BE-6.1 → 6.2 → 6.3 → 6.4
FASE 7  Hardening        BE-7.1 → 7.2 → 7.3 → 7.4 → 7.5
```

## Titik Sinkronisasi dengan Frontend

| Setelah BE selesai | FE dapat mulai |
|---|---|
| BE-0.6 | FE-0.7 (verifikasi konektivitas) |
| BE-2.2, BE-2.3 | FE-1.4, FE-1.5 (auth flow) |
| BE-1.4 | FE-3.4 (daftar gejala) |
| BE-3.4 | FE-3.2 (pencatatan periode) |
| BE-3.5 | FE-3.3 (log harian) |
| BE-4.2 | FE-4.1, FE-4.2 (tampilan prediksi) |
| BE-4.3 | FE-5.3 (penjadwalan ulang notifikasi) |
| BE-5.1 – 5.3 | FE-6.2, FE-6.3 (halaman insight) |
| BE-6.1 | FE-7.1 (wellness) |
| BE-6.3 | FE-7.2 (export) |
| BE-6.4 | FE-7.5 (hapus akun) |
