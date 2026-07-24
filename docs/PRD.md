# PRD — LLunara App

**Product Requirements Document**

| | |
|---|---|
| **Nama Produk** | LLunara |
| **Versi Dokumen** | 1.0 |
| **Tanggal** | 24 Juli 2026 |
| **Status** | Draft — Pre-Development |
| **Tipe Project** | Portfolio Project (Solo Developer) |

---

## 1. Ringkasan Produk

LLunara adalah aplikasi mobile untuk mencatat dan memantau siklus menstruasi, dilengkapi dengan analitik kesehatan personal, pengingat siklus, dan tracking wellness harian. Aplikasi ini dirancang sebagai *personal health companion*, bukan sekadar kalender menstruasi.

Referensi produk: Flo Health.

### 1.1 Konteks Project

Project ini dibangun sebagai **portfolio project oleh solo developer**, dengan dua tujuan:

1. **Fungsional** — menghasilkan aplikasi yang benar-benar dipakai (personal use, 1–2 pengguna)
2. **Demonstratif** — menunjukkan kemampuan engineering yang mendekati standar industri: arsitektur yang sadar trade-off, keamanan data sensitif, dokumentasi yang layak, dan keputusan teknis yang bisa dipertanggungjawabkan

**Batasan utama:** seluruh stack harus dapat berjalan pada tier gratis, tanpa memerlukan kartu kredit di titik manapun.

### 1.2 Problem Statement

Pencatatan siklus menstruasi umumnya dilakukan secara manual (catatan HP, ingatan) sehingga:

- Sulit melihat pola jangka panjang (keteraturan siklus, korelasi gejala)
- Prediksi siklus berikutnya tidak akurat
- Tidak ada data terstruktur ketika perlu konsultasi ke tenaga medis
- Tidak ada pengingat proaktif menjelang siklus

### 1.3 Target Pengguna

| Persona | Deskripsi | Kebutuhan Utama |
|---|---|---|
| **Primary User** | Wanita usia produktif yang ingin memantau siklus dan kondisi tubuhnya secara mandiri | Pencatatan cepat, prediksi akurat, insight yang mudah dipahami |
| **Partner (opsional)** | Pasangan yang diberi akses terbatas oleh primary user | Melihat ringkasan siklus untuk empati & perencanaan bersama |

---

## 2. Tujuan & Batasan

### 2.1 Goals

| # | Goal | Metrik Keberhasilan |
|---|---|---|
| G1 | Pencatatan harian tidak memakan waktu lama | Log gejala harian selesai dalam ≤ 3 tap |
| G2 | Prediksi siklus lebih akurat daripada rata-rata statis | Prediksi memakai rolling average + deteksi outlier |
| G3 | User memahami pola tubuhnya | Minimal 3 jenis insight tersedia di dashboard |
| G4 | Data dapat dibawa ke tenaga medis | Export laporan PDF/CSV berfungsi |
| G5 | Biaya operasional Rp 0 | Tidak ada layanan berbayar / kartu kredit terdaftar |

### 2.2 Non-Goals (Eksplisit Tidak Dikerjakan)

- Diagnosis medis atau saran pengobatan
- Integrasi dengan wearable device (Apple Health, Google Fit, smartwatch)
- Mode kehamilan & mode TTC (*trying to conceive*)
- Komunitas / forum antar pengguna
- Monetisasi, iklan, in-app purchase
- Publikasi ke Google Play Store / Apple App Store
- Dukungan offline penuh (lihat ADR-002)

### 2.3 Constraints

| Constraint | Implikasi |
|---|---|
| Zero budget, tanpa kartu kredit | Semua layanan harus punya free tier tanpa payment method |
| Solo developer | Scope harus realistis; hindari fitur yang butuh maintenance tinggi |
| Data kesehatan reproduksi = data sensitif | Keamanan & privasi bukan opsional, harus jadi requirement kelas satu |
| Supabase free tier auto-pause setelah 7 hari idle | Perlu penanganan UI & mitigasi keep-alive |
| Zeabur free plan sleep setelah periode idle (lihat ADR-005) | Perlu loading state eksplisit untuk cold start |

---

## 3. Ruang Lingkup Fitur

### 3.1 Prioritas Fitur (MoSCoW)

| Prioritas | Fitur |
|---|---|
| **Must Have** | Autentikasi, Core Tracking, Prediksi Siklus, Reminder Siklus |
| **Should Have** | Insight & Analytics, Export Laporan |
| **Could Have** | Wellness Tracking, Reminder Obat, Konten Edukasi |
| **Won't Have (v1)** | Partner Sharing Mode |

> **Catatan:** Partner Sharing Mode dipindahkan ke v2 agar v1 dapat diselesaikan dan berfungsi utuh terlebih dahulu. Skema database v1 sudah dirancang untuk mengakomodasi fitur ini tanpa migrasi besar.

---

### 3.2 F1 — Autentikasi (Must Have)

**Deskripsi:** Sistem akun untuk mengidentifikasi pemilik data dan menjadi dasar keamanan row-level.

**User Stories**

- Sebagai pengguna, saya ingin mendaftar dengan email & password agar data saya tersimpan dan hanya bisa diakses oleh saya
- Sebagai pengguna, saya ingin tetap login saat membuka aplikasi kembali agar tidak perlu memasukkan kredensial berulang kali
- Sebagai pengguna, saya ingin logout agar data saya aman di perangkat bersama

**Acceptance Criteria**

- [ ] Registrasi via email + password berhasil membuat entri di `auth.users` Supabase
- [ ] Login mengembalikan JWT yang disimpan di secure storage (`expo-secure-store`), **bukan** AsyncStorage
- [ ] Session di-refresh otomatis sebelum token expired
- [ ] Logout menghapus token dari secure storage dan mengembalikan user ke halaman login
- [ ] Password minimal 8 karakter, divalidasi di sisi klien dan server
- [ ] Error message tidak membocorkan informasi (misal: "email atau password salah", bukan "email tidak ditemukan")

---

### 3.3 F2 — Core Tracking (Must Have)

**Deskripsi:** Fondasi aplikasi. Pencatatan periode menstruasi dan kondisi harian.

**User Stories**

- Sebagai pengguna, saya ingin menandai tanggal mulai dan selesai menstruasi agar sistem punya data untuk prediksi
- Sebagai pengguna, saya ingin mencatat intensitas flow harian agar bisa melihat pola volume menstruasi
- Sebagai pengguna, saya ingin mencatat gejala yang saya rasakan agar bisa dikorelasikan dengan fase siklus
- Sebagai pengguna, saya ingin menulis catatan bebas untuk hal yang tidak tercakup di daftar gejala
- Sebagai pengguna, saya ingin melihat kalender visual agar cepat memahami posisi saya dalam siklus

**Sub-fitur**

| Sub-fitur | Detail |
|---|---|
| **Period Logging** | Tandai tanggal mulai & selesai menstruasi |
| **Flow Intensity** | Enum: `light`, `medium`, `heavy` |
| **Symptom Logging** | Multi-select dari daftar preset + kemampuan tambah tag kustom |
| **Mood Logging** | Single-select dari daftar preset |
| **Daily Notes** | Free text, maksimal 500 karakter |
| **Calendar View** | Tampilan bulanan dengan indikator warna per fase siklus |

**Daftar Gejala Preset**

`kram` · `sakit kepala` · `nyeri payudara` · `kembung` · `jerawat` · `kelelahan` · `nyeri punggung` · `mual` · `perubahan nafsu makan` · `sulit tidur` · `keputihan`

**Daftar Mood Preset**

`senang` · `tenang` · `biasa` · `sensitif` · `cemas` · `sedih` · `mudah marah`

**Acceptance Criteria**

- [ ] User dapat menandai satu tanggal sebagai awal menstruasi; sistem menolak jika bertumpuk dengan periode yang sudah tercatat
- [ ] Satu hari hanya boleh punya satu entri log (unique constraint `user_id` + `date`)
- [ ] Gejala dapat dipilih lebih dari satu dalam satu hari
- [ ] Tag gejala kustom tersimpan dan muncul di daftar pilihan pada hari-hari berikutnya
- [ ] Kalender menampilkan pembeda visual untuk: hari menstruasi, prediksi menstruasi, masa subur, dan hari dengan log gejala
- [ ] Log dapat diedit dan dihapus
- [ ] Tanggal di masa depan tidak dapat diberi log gejala (hanya prediksi yang boleh muncul)

---

### 3.4 F3 — Prediksi Siklus & Masa Subur (Must Have)

**Deskripsi:** Logika kalkulasi yang memprediksi siklus berikutnya berdasarkan riwayat, bukan angka statis.

**Aturan Kalkulasi**

| Parameter | Aturan |
|---|---|
| **Panjang siklus** | Rata-rata dari maksimal 6 siklus terakhir. Jika data < 2 siklus, gunakan default 28 hari |
| **Deteksi outlier** | Siklus dengan panjang < 21 atau > 45 hari dikecualikan dari perhitungan rata-rata (tetap disimpan sebagai data) |
| **Durasi menstruasi** | Rata-rata dari maksimal 6 periode terakhir. Default 5 hari |
| **Estimasi ovulasi** | Panjang siklus dikurangi 14 hari (fase luteal) |
| **Masa subur** | 5 hari sebelum ovulasi sampai 1 hari sesudah ovulasi |
| **Confidence level** | `low` (<3 siklus), `medium` (3–5 siklus), `high` (≥6 siklus dengan variasi ≤ 5 hari) |

**Acceptance Criteria**

- [ ] Prediksi ditampilkan di kalender dengan gaya visual berbeda dari data aktual (misal: garis putus-putus / opacity lebih rendah)
- [ ] Confidence level ditampilkan ke user, disertai penjelasan singkat
- [ ] Prediksi diperbarui otomatis setiap kali periode baru dicatat
- [ ] Aplikasi menampilkan disclaimer bahwa prediksi bukan alat kontrasepsi dan bukan diagnosis medis
- [ ] Siklus yang dikecualikan sebagai outlier tetap tampil di riwayat, dengan penanda

---

### 3.5 F4 — Reminder & Notifikasi (Must Have)

**Deskripsi:** Pengingat proaktif berbasis **local notification**, dijadwalkan di perangkat.

**Jenis Reminder**

| Jenis | Waktu | Prioritas |
|---|---|---|
| Menstruasi akan datang | H-2 dan H-1 dari prediksi | Must |
| Masa subur dimulai | Hari pertama fertile window | Should |
| Pengingat obat / pil KB | Harian, jam ditentukan user | Could |
| Medical check-up | Tanggal ditentukan user | Could |

**Acceptance Criteria**

- [ ] Notifikasi dijadwalkan melalui `expo-notifications` secara lokal, tanpa dependensi server
- [ ] Notifikasi tetap muncul saat perangkat offline
- [ ] Jadwal notifikasi diperbarui otomatis ketika prediksi siklus berubah
- [ ] User dapat mengaktifkan/menonaktifkan tiap jenis reminder secara terpisah
- [ ] Aplikasi meminta izin notifikasi dengan konteks yang jelas, bukan langsung saat pertama dibuka
- [ ] Isi notifikasi bersifat diskret (contoh: "Pengingat LLunara", bukan menyebut menstruasi secara eksplisit di lock screen)

---

### 3.6 F5 — Insight & Analytics (Should Have)

**Deskripsi:** Mengubah data mentah menjadi pemahaman yang dapat ditindaklanjuti.

**Insight yang Disediakan**

| Insight | Deskripsi |
|---|---|
| **Ringkasan siklus** | Rata-rata panjang siklus, siklus terpendek & terpanjang, tingkat keteraturan |
| **Tren panjang siklus** | Grafik garis panjang siklus dari waktu ke waktu |
| **Frekuensi gejala** | Gejala yang paling sering muncul, diurutkan |
| **Korelasi gejala–fase** | Distribusi gejala per fase siklus (menstruasi, folikular, ovulasi, luteal) |
| **Pola mood** | Distribusi mood per fase siklus |

**Acceptance Criteria**

- [ ] Insight hanya ditampilkan jika data mencukupi; jika belum, tampilkan *empty state* yang informatif (bukan grafik kosong)
- [ ] Setiap insight disertai penjelasan singkat dalam bahasa awam
- [ ] Perhitungan dilakukan di backend Go, bukan di frontend
- [ ] Insight bersifat deskriptif, tidak memberikan klaim atau saran medis

---

### 3.7 F6 — Export Laporan (Should Have)

**Deskripsi:** Mengeluarkan data dalam format yang bisa dibawa ke dokter atau disimpan sebagai backup pribadi.

**Acceptance Criteria**

- [ ] Export ke CSV berisi seluruh log harian dalam rentang tanggal yang dipilih
- [ ] Export ke PDF berisi ringkasan siklus, statistik, dan riwayat dalam format yang mudah dibaca
- [ ] File dapat dibagikan melalui share sheet perangkat (`expo-sharing`)
- [ ] User dapat memilih rentang tanggal (3 bulan / 6 bulan / 1 tahun / kustom)
- [ ] Export berfungsi sebagai mekanisme backup manual, mengingat free tier Supabase tidak menyediakan automated backup

---

### 3.8 F7 — Wellness Tracking (Could Have)

**Deskripsi:** Tracking sederhana faktor gaya hidup yang berpengaruh pada siklus. Seluruhnya input manual.

| Metrik | Satuan | Input |
|---|---|---|
| Air minum | gelas | Counter tap |
| Tidur | jam | Slider / numeric input |
| Berat badan | kg | Numeric input |

**Acceptance Criteria**

- [ ] Setiap metrik dapat diaktifkan/dinonaktifkan oleh user (tidak memaksa semua tampil)
- [ ] Data wellness ikut terlihat dalam grafik insight sebagai konteks tambahan
- [ ] Tidak ada target atau angka anjuran yang dipaksakan sistem; user menentukan sendiri
- [ ] Tidak ada gamifikasi berbasis streak yang menghukum user saat melewatkan hari

---

### 3.9 F8 — Konten Edukasi (Could Have)

**Deskripsi:** Artikel singkat seputar kesehatan reproduksi, disimpan sebagai konten statis di dalam aplikasi.

**Acceptance Criteria**

- [ ] Konten disimpan lokal dalam aplikasi (JSON/MDX), tidak memerlukan API eksternal
- [ ] Setiap artikel mencantumkan sumber referensi
- [ ] Terdapat disclaimer bahwa konten bersifat informatif, bukan pengganti konsultasi medis

---

### 3.10 F9 — Partner Sharing Mode (v2 — Tidak Dikerjakan di v1)

**Deskripsi:** Memberi akses baca terbatas kepada partner.

**Rancangan awal (untuk referensi v2)**

- Primary user membuat kode undangan; partner memasukkan kode tersebut
- Akses bersifat **read-only** dan **granular** — user memilih kategori data apa yang dibagikan
- Data sensitif (catatan pribadi) tidak pernah dibagikan secara default
- Akses dapat dicabut kapan saja oleh primary user

**Persiapan di v1:** tabel `sharing_permissions` sudah termasuk dalam skema database v1 agar tidak perlu migrasi besar saat fitur ini dikerjakan.

---

## 4. Requirement Non-Fungsional

### 4.1 Keamanan & Privasi

Ini adalah aplikasi data kesehatan reproduksi. Keamanan diperlakukan sebagai requirement fungsional, bukan pelengkap.

| # | Requirement |
|---|---|
| S1 | **Row Level Security (RLS) wajib aktif** di seluruh tabel Supabase. Tidak ada tabel yang boleh dapat dibaca lintas user |
| S2 | JWT disimpan di `expo-secure-store` (Keychain/Keystore OS), bukan AsyncStorage |
| S3 | Backend Go memverifikasi signature JWT Supabase pada setiap request; tidak pernah mempercayai `user_id` yang dikirim dari klien |
| S4 | Seluruh komunikasi melalui HTTPS |
| S5 | Service role key Supabase hanya berada di environment variable backend, tidak pernah ada di kode frontend atau repository |
| S6 | Aplikasi menyediakan opsi kunci layar (PIN atau biometrik) sebelum masuk ke data |
| S7 | Tersedia fitur hapus akun beserta seluruh data terkait (hard delete) |
| S8 | Tidak ada analytics pihak ketiga, tidak ada tracking perilaku user |
| S9 | Isi notifikasi tidak mengungkap informasi sensitif di lock screen |

### 4.2 Performa

| Metrik | Target |
|---|---|
| Cold start aplikasi | < 3 detik |
| Response API (server aktif) | < 500 ms untuk operasi CRUD |
| Kalkulasi insight | < 2 detik untuk data 12 bulan |
| Cold start backend (Zeabur free plan) | Durasi pasti tidak dipublikasikan resmi oleh Zeabur (dokumentasi mereka hanya menyebut "beberapa detik"); **wajib ditangani dengan loading state eksplisit** sebagai jaring pengaman, asumsikan bisa lebih lama dari perkiraan |

### 4.3 Reliability

| Kondisi | Perilaku yang Diharapkan |
|---|---|
| Backend sedang cold start | Tampilkan loading state dengan pesan "Menghubungkan ke server…", bukan spinner tanpa konteks |
| Supabase project ter-pause | Tampilkan error state yang jelas beserta panduan tindakan, bukan crash atau layar kosong |
| Perangkat offline | Tampilkan pesan offline yang eksplisit; aplikasi tidak boleh crash |
| Request timeout | Retry otomatis maksimal 2 kali dengan exponential backoff, lalu tampilkan opsi retry manual |

### 4.4 Usability & Aksesibilitas

- [ ] Dark mode tersedia dan mengikuti preferensi sistem
- [ ] Kontras warna memenuhi WCAG AA (rasio minimal 4.5:1 untuk teks)
- [ ] Ukuran target sentuh minimal 44×44 pt
- [ ] Label aksesibilitas tersedia pada seluruh elemen interaktif
- [ ] Aplikasi mendukung ukuran font sistem (dynamic type)

### 4.5 Nada & Etika Produk

Karena ini menyangkut kesehatan dan citra tubuh, aplikasi harus:

- Menggunakan bahasa netral dan tidak menghakimi
- Tidak membuat klaim medis atau memberi kesan diagnosis
- Tidak menampilkan pesan yang membuat user merasa bersalah saat melewatkan pencatatan
- Menyertakan disclaimer medis di layar onboarding dan di halaman insight
- Tidak menggunakan mekanik streak yang menekan user untuk membuka aplikasi setiap hari

---

## 5. Arsitektur Teknis

### 5.1 Tech Stack

| Layer | Teknologi | Alasan |
|---|---|---|
| **Frontend** | Expo (React Native) + TypeScript | Cross-platform, DX baik, build gratis 15×/bulan |
| **Backend** | Go | Sudah dikuasai developer, performa baik, cocok untuk business logic |
| **Database** | Supabase (PostgreSQL) | Free tier memadai, RLS bawaan, tidak perlu kartu kredit |
| **Auth** | Supabase Auth | Terintegrasi dengan RLS, JWT standar |
| **Hosting BE** | Zeabur (free plan) | Terverifikasi tanpa kartu kredit di dokumentasi resmi, mendukung deploy dari Dockerfile + auto-deploy dari GitHub (lihat ADR-005) |
| **Notifikasi** | `expo-notifications` (local) | Gratis, berfungsi offline, tanpa server |
| **Repository** | 2 repo terpisah: `llunara-mobile` & `llunara-api` | Pemisahan concern, deployment pipeline independen |

### 5.2 Diagram Arsitektur

```
┌─────────────────────────────────────────────┐
│              Expo Mobile App                │
│  ┌──────────────┐      ┌─────────────────┐  │
│  │  UI Layer    │      │ Local Notif     │  │
│  │  (Screens)   │      │ Scheduler       │  │
│  └──────┬───────┘      └─────────────────┘  │
│         │                                    │
│  ┌──────▼───────────────────────────────┐   │
│  │       API Client Layer               │   │
│  │  ┌────────────┐   ┌───────────────┐  │   │
│  │  │ Supabase   │   │  Go API       │  │   │
│  │  │ Client     │   │  Client       │  │   │
│  │  └─────┬──────┘   └───────┬───────┘  │   │
│  └────────┼──────────────────┼──────────┘   │
└───────────┼──────────────────┼──────────────┘
            │                  │
   READ ONLY│ (RLS)            │ WRITE + COMPLEX READ
            │                  │ (JWT verified)
            │                  ▼
            │        ┌──────────────────────┐
            │        │   Go API (Zeabur)    │
            │        │  ┌────────────────┐  │
            │        │  │ JWT Middleware │  │
            │        │  ├────────────────┤  │
            │        │  │ Handler        │  │
            │        │  ├────────────────┤  │
            │        │  │ Service        │  │
            │        │  │ (prediksi,     │  │
            │        │  │  insight)      │  │
            │        │  ├────────────────┤  │
            │        │  │ Repository     │  │
            │        │  └───────┬────────┘  │
            │        └──────────┼───────────┘
            │                   │
            ▼                   ▼
    ┌───────────────────────────────────┐
    │   Supabase (PostgreSQL + Auth)    │
    │   Row Level Security aktif        │
    └───────────────────────────────────┘
```

### 5.3 Aturan Pembagian Tanggung Jawab

Untuk menghindari kebingungan akibat arsitektur hybrid, berlaku **satu aturan tegas**:

> **Semua operasi WRITE dan semua READ yang membutuhkan kalkulasi → melalui Go API.**
> **Frontend hanya boleh memanggil Supabase secara langsung untuk READ data mentah sederhana.**

| Operasi | Jalur | Alasan |
|---|---|---|
| Login / Register / Refresh token | Supabase Auth SDK langsung | Auth adalah domain Supabase |
| Ambil daftar log harian (mentah) | Supabase langsung (dilindungi RLS) | Query sederhana, tidak butuh logic |
| Ambil daftar gejala preset | Supabase langsung | Data statis |
| Simpan / ubah / hapus log harian | **Go API** | Butuh validasi bisnis & trigger recalculation |
| Simpan periode menstruasi | **Go API** | Butuh validasi tumpang tindih & recalculation prediksi |
| Ambil prediksi siklus | **Go API** | Hasil kalkulasi |
| Ambil data insight | **Go API** | Hasil agregasi & kalkulasi |
| Generate export PDF/CSV | **Go API** | Pemrosesan dokumen |

Pola ini merupakan varian **BFF (Backend for Frontend)** yang dikombinasikan dengan direct database read untuk query ringan.

---

## 6. Skema Database

### 6.1 Diagram Relasi

```
auth.users (Supabase managed)
     │
     ├──1:1──► profiles
     │
     ├──1:N──► cycles
     │            │
     │            └──1:N──► daily_logs
     │
     ├──1:N──► daily_logs
     │            │
     │            ├──N:M──► symptoms (via daily_log_symptoms)
     │            │
     │            └──1:1──► wellness_logs
     │
     ├──1:N──► symptoms (custom tags)
     ├──1:N──► reminders
     └──1:N──► sharing_permissions  (disiapkan untuk v2)
```

### 6.2 Definisi Tabel

**`profiles`**

| Kolom | Tipe | Keterangan |
|---|---|---|
| `id` | uuid, PK | FK ke `auth.users.id` |
| `display_name` | text | Nama tampilan |
| `birth_year` | int, nullable | Untuk konteks insight |
| `default_cycle_length` | int, default 28 | Fallback saat data belum cukup |
| `default_period_length` | int, default 5 | Fallback saat data belum cukup |
| `preferences` | jsonb | Pengaturan tema, toggle wellness, dll |
| `created_at` | timestamptz | |
| `updated_at` | timestamptz | |

**`cycles`**

| Kolom | Tipe | Keterangan |
|---|---|---|
| `id` | uuid, PK | |
| `user_id` | uuid, FK | |
| `start_date` | date, not null | Hari pertama menstruasi |
| `end_date` | date, nullable | Null jika masih berlangsung |
| `cycle_length` | int, nullable | Dihitung saat siklus berikutnya dimulai |
| `period_length` | int, nullable | |
| `is_outlier` | boolean, default false | Ditandai jika di luar rentang 21–45 hari |
| `created_at` | timestamptz | |

Constraint: `UNIQUE(user_id, start_date)`

**`daily_logs`**

| Kolom | Tipe | Keterangan |
|---|---|---|
| `id` | uuid, PK | |
| `user_id` | uuid, FK | |
| `cycle_id` | uuid, FK, nullable | |
| `date` | date, not null | |
| `flow_intensity` | enum, nullable | `light` / `medium` / `heavy` |
| `mood` | text, nullable | |
| `notes` | text, nullable | Maks 500 karakter |
| `created_at` | timestamptz | |
| `updated_at` | timestamptz | |

Constraint: `UNIQUE(user_id, date)`

**`symptoms`**

| Kolom | Tipe | Keterangan |
|---|---|---|
| `id` | uuid, PK | |
| `user_id` | uuid, FK, nullable | Null berarti gejala preset milik sistem |
| `name` | text, not null | |
| `category` | text | `physical` / `emotional` / `other` |
| `is_custom` | boolean, default false | |

**`daily_log_symptoms`** (junction table)

| Kolom | Tipe |
|---|---|
| `daily_log_id` | uuid, FK |
| `symptom_id` | uuid, FK |

Primary key gabungan: `(daily_log_id, symptom_id)`

**`wellness_logs`**

| Kolom | Tipe | Keterangan |
|---|---|---|
| `id` | uuid, PK | |
| `user_id` | uuid, FK | |
| `date` | date, not null | |
| `water_glasses` | int, nullable | |
| `sleep_hours` | numeric(3,1), nullable | |
| `weight_kg` | numeric(5,2), nullable | |

Constraint: `UNIQUE(user_id, date)`

**`reminders`**

| Kolom | Tipe | Keterangan |
|---|---|---|
| `id` | uuid, PK | |
| `user_id` | uuid, FK | |
| `type` | enum | `period_upcoming` / `fertile_window` / `medication` / `checkup` |
| `is_enabled` | boolean, default true | |
| `time_of_day` | time, nullable | |
| `days_before` | int, nullable | Untuk reminder berbasis prediksi |
| `custom_message` | text, nullable | |

**`sharing_permissions`** (disiapkan untuk v2, belum dipakai di v1)

| Kolom | Tipe | Keterangan |
|---|---|---|
| `id` | uuid, PK | |
| `owner_user_id` | uuid, FK | |
| `partner_user_id` | uuid, FK, nullable | Null sebelum undangan diterima |
| `invite_code` | text, unique | |
| `shared_categories` | jsonb | Kategori data yang dibagikan |
| `status` | enum | `pending` / `active` / `revoked` |
| `expires_at` | timestamptz, nullable | |

### 6.3 Row Level Security

Setiap tabel wajib memiliki policy dasar berikut:

```sql
-- Contoh untuk daily_logs
alter table daily_logs enable row level security;

create policy "user_can_select_own_logs"
  on daily_logs for select
  using (auth.uid() = user_id);

create policy "user_can_insert_own_logs"
  on daily_logs for insert
  with check (auth.uid() = user_id);

create policy "user_can_update_own_logs"
  on daily_logs for update
  using (auth.uid() = user_id);

create policy "user_can_delete_own_logs"
  on daily_logs for delete
  using (auth.uid() = user_id);
```

**Catatan penting:** Backend Go menggunakan service role key yang **melewati RLS**. Oleh karena itu, backend **wajib** melakukan filter `user_id` secara eksplisit di setiap query, berdasarkan JWT yang telah diverifikasi — bukan berdasarkan nilai yang dikirim klien.

---

## 7. Kontrak API (Go Backend)

Base URL: `https://llunara-api.onrender.com/api/v1`

Seluruh endpoint memerlukan header `Authorization: Bearer <supabase_jwt>`.

### 7.1 Daftar Endpoint

| Method | Path | Deskripsi |
|---|---|---|
| `GET` | `/health` | Health check (tanpa auth) |
| `POST` | `/cycles` | Catat awal menstruasi baru |
| `PATCH` | `/cycles/:id` | Perbarui siklus (mis. tanggal selesai) |
| `DELETE` | `/cycles/:id` | Hapus siklus |
| `GET` | `/cycles/prediction` | Ambil prediksi siklus & masa subur |
| `POST` | `/daily-logs` | Buat / perbarui log harian (upsert) |
| `DELETE` | `/daily-logs/:date` | Hapus log harian |
| `GET` | `/insights/summary` | Ringkasan statistik siklus |
| `GET` | `/insights/symptoms` | Frekuensi & korelasi gejala |
| `GET` | `/insights/mood` | Pola mood per fase siklus |
| `POST` | `/export` | Generate laporan (query: `format`, `from`, `to`) |
| `DELETE` | `/account` | Hapus akun & seluruh data |

### 7.2 Contoh Response

`GET /cycles/prediction`

```json
{
  "data": {
    "next_period_start": "2026-08-12",
    "next_period_end": "2026-08-16",
    "estimated_ovulation": "2026-07-29",
    "fertile_window": {
      "start": "2026-07-24",
      "end": "2026-07-30"
    },
    "current_phase": "follicular",
    "day_of_cycle": 9,
    "confidence": "medium",
    "based_on_cycles": 4,
    "average_cycle_length": 29
  }
}
```

### 7.3 Format Error

Seluruh error mengikuti struktur konsisten:

```json
{
  "error": {
    "code": "CYCLE_OVERLAP",
    "message": "Tanggal ini bertumpuk dengan siklus yang sudah tercatat",
    "details": {
      "conflicting_cycle_id": "uuid-here"
    }
  }
}
```

**Kode error standar**

| Kode | HTTP Status |
|---|---|
| `UNAUTHORIZED` | 401 |
| `FORBIDDEN` | 403 |
| `NOT_FOUND` | 404 |
| `VALIDATION_ERROR` | 422 |
| `CYCLE_OVERLAP` | 409 |
| `INSUFFICIENT_DATA` | 422 |
| `INTERNAL_ERROR` | 500 |

---

## 8. Struktur Repository

### 8.1 `llunara-api` (Go)

Mengikuti pendekatan *Standard Go Project Layout* dengan pemisahan layer yang jelas.

```
llunara-api/
├── cmd/
│   └── api/
│       └── main.go                 # Entry point
├── internal/
│   ├── config/
│   │   └── config.go               # Load env variables
│   ├── handler/                    # HTTP layer (parsing, response)
│   │   ├── cycle_handler.go
│   │   ├── daily_log_handler.go
│   │   ├── insight_handler.go
│   │   └── export_handler.go
│   ├── service/                    # Business logic (inti nilai project ini)
│   │   ├── cycle_service.go
│   │   ├── prediction_service.go   # Algoritma prediksi
│   │   ├── insight_service.go      # Agregasi & korelasi
│   │   └── export_service.go
│   ├── repository/                 # Akses database
│   │   ├── cycle_repository.go
│   │   ├── daily_log_repository.go
│   │   └── postgres.go
│   ├── model/                      # Domain entities
│   │   ├── cycle.go
│   │   ├── daily_log.go
│   │   └── insight.go
│   ├── middleware/
│   │   ├── auth.go                 # Verifikasi JWT Supabase
│   │   ├── logger.go
│   │   ├── cors.go
│   │   └── rate_limit.go
│   └── pkg/
│       ├── validator/
│       └── apierror/               # Struktur error terstandardisasi
├── migrations/                     # SQL migration files
│   ├── 001_init_schema.sql
│   ├── 002_rls_policies.sql
│   └── 003_seed_symptoms.sql
├── docs/
│   ├── ARCHITECTURE.md
│   └── API.md
├── .env.example
├── Dockerfile
├── Makefile
├── go.mod
└── README.md
```

### 8.2 `llunara-mobile` (Expo)

```
llunara-mobile/
├── app/                            # Expo Router (file-based routing)
│   ├── (auth)/
│   │   ├── login.tsx
│   │   └── register.tsx
│   ├── (tabs)/
│   │   ├── index.tsx               # Dashboard / Today
│   │   ├── calendar.tsx
│   │   ├── insights.tsx
│   │   └── settings.tsx
│   ├── log/
│   │   └── [date].tsx              # Detail & edit log harian
│   └── _layout.tsx
├── src/
│   ├── api/
│   │   ├── supabase.ts             # Supabase client (read-only queries)
│   │   ├── client.ts               # Go API client + interceptor
│   │   └── endpoints/
│   ├── components/
│   │   ├── ui/                     # Komponen dasar reusable
│   │   ├── calendar/
│   │   ├── charts/
│   │   └── feedback/               # Loading, error, empty state
│   ├── hooks/
│   │   ├── useAuth.ts
│   │   ├── useCycle.ts
│   │   └── useNotifications.ts
│   ├── store/                      # State management
│   ├── services/
│   │   └── notification.ts         # Penjadwalan local notification
│   ├── constants/
│   │   ├── symptoms.ts
│   │   └── theme.ts
│   ├── types/
│   └── utils/
├── assets/
├── app.json
├── eas.json
├── .env.example
├── tsconfig.json
└── README.md
```

---

## 9. Alur Data (Data Flow)

### 9.1 Alur: Mencatat Awal Menstruasi

```
User tap tanggal di kalender
        │
        ▼
Konfirmasi "Tandai sebagai awal menstruasi?"
        │
        ▼
POST /cycles  { start_date }
        │
        ▼
Go: Middleware verifikasi JWT → ambil user_id dari token
        │
        ▼
Go: Service validasi — apakah bertumpuk dengan siklus lain?
        │
        ├── Ya  ──► Response 409 CYCLE_OVERLAP
        │
        └── Tidak
             │
             ▼
        Go: Repository INSERT ke tabel cycles
             │
             ▼
        Go: Tutup siklus sebelumnya, hitung cycle_length
             │
             ▼
        Go: Tandai outlier jika di luar rentang 21–45 hari
             │
             ▼
        Go: Hitung ulang prediksi siklus berikutnya
             │
             ▼
        Response 201 + data prediksi terbaru
             │
             ▼
        FE: Perbarui cache & UI kalender
             │
             ▼
        FE: Batalkan notifikasi lama, jadwalkan ulang notifikasi baru
```

### 9.2 Alur: Membuka Dashboard (menangani cold start)

```
App dibuka
   │
   ▼
Cek JWT di secure store
   │
   ├── Tidak ada / expired ──► Redirect ke halaman login
   │
   └── Valid
        │
        ├──► Supabase (langsung): ambil log harian bulan ini  [cepat]
        │         └──► Tampilkan kalender segera
        │
        └──► Go API: GET /cycles/prediction  [mungkin cold start]
                  │
                  ├── Response < 3 detik ──► Tampilkan prediksi
                  │
                  └── Response > 3 detik
                            │
                            ▼
                  Tampilkan: "Menghubungkan ke server…"
                  Kalender tetap tampil dari data Supabase
                            │
                            ▼
                  Setelah response tiba ──► Overlay prediksi ditambahkan
```

Pola ini membuat aplikasi tetap terasa responsif meskipun backend sedang cold start — data mentah tampil lebih dulu, hasil kalkulasi menyusul.

---

## 10. Architecture Decision Records

Bagian ini mendokumentasikan keputusan teknis beserta alasannya. Ini penting untuk konteks portofolio.

### ADR-001 — Arsitektur Hybrid: Supabase Direct + Go API

**Status:** Diterima

**Konteks:** Supabase menyediakan REST API otomatis. Menulis ulang seluruh CRUD di Go akan menduplikasi fungsionalitas tersebut.

**Keputusan:** Frontend mengakses Supabase secara langsung untuk read sederhana; seluruh write dan read yang membutuhkan kalkulasi melalui Go API.

**Konsekuensi:**
- (+) Waktu pengembangan lebih singkat; backend Go fokus pada logic bernilai tinggi
- (+) Beban request ke backend berkurang, mengurangi frekuensi cold start
- (−) Terdapat dua jalur akses data yang harus dijaga konsistensinya
- (−) Keamanan diberlakukan di dua tempat (RLS dan middleware JWT)

**Mitigasi:** Aturan tegas di Bagian 5.3, didokumentasikan di README kedua repository.

### ADR-002 — Cloud-Only, Bukan Local-First

**Status:** Diterima

**Konteks:** Local-first memberikan pengalaman offline yang lebih baik, tetapi menambah kompleksitas sync dan conflict resolution secara signifikan.

**Keputusan:** Seluruh data disimpan di Supabase. Tidak ada database lokal.

**Konsekuensi:**
- (+) Pengembangan jauh lebih sederhana; tidak ada sync logic
- (+) Data konsisten di semua perangkat secara otomatis
- (−) Aplikasi tidak berfungsi tanpa koneksi internet
- (−) Jika Supabase project ter-pause, aplikasi tidak dapat digunakan sampai di-resume manual

**Mitigasi:** Error state yang informatif, keep-alive via GitHub Actions, dan fitur export manual sebagai backup.

### ADR-003 — Local Notification, Bukan Push Notification

**Status:** Diterima

**Konteks:** Reminder bersifat dapat diprediksi dari data siklus, tidak memerlukan trigger real-time dari server.

**Keputusan:** Menggunakan `expo-notifications` untuk penjadwalan lokal.

**Konsekuensi:**
- (+) Gratis, tidak memerlukan FCM, `google-services.json`, maupun server push
- (+) Berfungsi meskipun perangkat offline
- (−) Jadwal notifikasi harus diperbarui manual dari sisi klien setiap kali prediksi berubah

### ADR-004 — Render Free Tier untuk Hosting Backend

**Status:** Digantikan oleh ADR-005 (24 Juli 2026)

**Konteks:** Membutuhkan hosting Go tanpa kartu kredit dan tanpa risiko tagihan tak terduga.

**Keputusan:** Menggunakan Render free tier tanpa mendaftarkan metode pembayaran.

**Konsekuensi:**
- (+) Tidak ada risiko tagihan — layanan di-suspend saat limit tercapai, bukan ditagih
- (+) Deployment langsung dari GitHub
- (−) Service tidur setelah 15 menit idle, menghasilkan cold start 30–60 detik

**Mitigasi:** Loading state eksplisit di frontend. Cold start diperlakukan sebagai perilaku yang diketahui dan ditangani, bukan disembunyikan.

**Catatan retrospektif:** Ternyata Render kini mewajibkan kartu kredit saat pendaftaran, melanggar constraint utama project ini (Bagian 2.3). Digantikan oleh ADR-005.

---

### ADR-005 — Zeabur sebagai Pengganti Render untuk Hosting Backend

**Status:** Diterima

**Konteks:** Render kini mewajibkan pendaftaran kartu kredit di seluruh alur signup-nya, melanggar batasan mutlak "tanpa kartu kredit di layanan manapun" (Bagian 1.1 & 2.3). Riset terhadap alternatif populer (Koyeb, Fly.io, Railway, Google Cloud Run, Oracle Cloud Free Tier) menunjukkan seluruhnya kini juga mewajibkan kartu kredit untuk verifikasi identitas — beberapa bahkan memiliki laporan pengguna tertagih riil akibat kesalahan pemilihan plan saat signup. Zeabur adalah satu-satunya platform yang terverifikasi di dokumentasi resminya sendiri tidak memerlukan kartu kredit sama sekali untuk free plan-nya.

**Keputusan:** Pindah hosting backend dari Render ke Zeabur free plan. Deploy tetap menggunakan `Dockerfile` yang sama (Zeabur mendukung deploy dari Dockerfile + auto-deploy dari GitHub, sehingga tidak ada perubahan kode aplikasi yang diperlukan).

**Konsekuensi:**
- (+) Tidak ada kartu kredit terdaftar di titik manapun, sesuai constraint utama project
- (+) Deployment langsung dari GitHub dengan auto-deploy on push, sama seperti rencana semula
- (+) Tidak perlu perubahan pada `Dockerfile` — Zeabur membaca port dari instruksi `EXPOSE` atau env var `PORT`, sama seperti asumsi awal untuk Render
- (−) Durasi cold start pasti tidak dipublikasikan resmi oleh Zeabur (dokumentasi mereka hanya menyebut "beberapa detik" setelah idle) — kurang presisi dibanding Render yang eksplisit 30–60 detik
- (−) Region terbatas sesuai ketersediaan free plan Zeabur, kemungkinan tidak sedekat Singapore dibanding Render

**Mitigasi:** Tetap pertahankan loading state progresif di frontend sebagai jaring pengaman terlepas dari durasi cold start aktual — desain ini sudah defensif terhadap ketidakpastian waktu cold start dari provider manapun.

---

## 11. Rencana Pengembangan

| Milestone | Cakupan | Deliverable |
|---|---|---|
| **M1 — Fondasi** | Setup 2 repo, skema DB + RLS, deploy skeleton Go ke Zeabur, setup Expo | Health check endpoint dapat diakses dari aplikasi |
| **M2 — Autentikasi** | Register, login, secure token storage, JWT middleware di Go | User dapat login dan mengakses endpoint terproteksi |
| **M3 — Core Tracking** | Tampilan kalender, pencatatan periode, log harian, gejala | Fitur pencatatan berfungsi utuh |
| **M4 — Prediksi** | Algoritma prediksi, tampilan fase siklus, confidence level | Prediksi tampil akurat di kalender |
| **M5 — Reminder** | Penjadwalan local notification, pengaturan reminder | Notifikasi muncul sesuai jadwal |
| **M6 — Insight** | Endpoint agregasi, grafik, empty state | Halaman insight berfungsi |
| **M7 — Polish** | Export, wellness tracking, dark mode, aksesibilitas, error handling | Aplikasi siap dipakai sehari-hari |
| **M8 — Dokumentasi** | README, ARCHITECTURE.md, diagram, demo video | Repository siap ditampilkan sebagai portofolio |

---

## 12. Risiko

| Risiko | Dampak | Mitigasi |
|---|---|---|
| Supabase project ter-pause karena idle | Aplikasi tidak dapat diakses | Keep-alive via GitHub Actions; error state yang jelas |
| Cold start Zeabur mengganggu pengalaman | Aplikasi terasa lambat | Loading state eksplisit; data Supabase tampil lebih dulu |
| Tidak ada automated backup di free tier | Risiko kehilangan data | Fitur export manual; backup berkala ke penyimpanan pribadi |
| Scope creep pada fitur wellness & edukasi | Project tidak selesai | Prioritas MoSCoW ditegakkan; Could Have dikerjakan paling akhir |
| Prediksi tidak akurat pada siklus tidak teratur | User kehilangan kepercayaan | Confidence level ditampilkan transparan; deteksi outlier |
| Ketidakkonsistenan antara jalur Supabase dan Go API | Bug sulit dilacak | Aturan pembagian di 5.3; kontrak API terdokumentasi |

---

## 13. Definition of Done

Sebuah fitur dianggap selesai apabila:

- [ ] Seluruh acceptance criteria terpenuhi
- [ ] Menangani loading, error, dan empty state
- [ ] Endpoint terkait memiliki unit test untuk logic-nya (khususnya prediksi & insight)
- [ ] Aman terhadap akses lintas user (diverifikasi dengan dua akun uji)
- [ ] Berfungsi pada mode terang dan gelap
- [ ] Tidak ada credential yang ter-commit ke repository
- [ ] Terdokumentasi di README atau `API.md`

---

## 14. Disclaimer Produk

Teks berikut wajib ditampilkan saat onboarding dan pada halaman insight:

> LLunara adalah alat bantu pencatatan pribadi. Prediksi yang ditampilkan merupakan estimasi berdasarkan data yang kamu masukkan, dan **bukan** metode kontrasepsi, bukan diagnosis, serta bukan pengganti konsultasi dengan tenaga medis. Untuk kekhawatiran terkait kesehatan, silakan berkonsultasi dengan dokter atau bidan.

---

## 15. Lampiran — Checklist Setup Layanan Gratis

| Layanan | Perlu Kartu Kredit | Batasan Free Tier | Catatan |
|---|---|---|---|
| Supabase | Tidak | 500 MB DB, 5 GB egress, 50.000 MAU, 2 project | Project ter-pause setelah 7 hari idle |
| Zeabur | Tidak | Free plan; limit resource rinci belum dipublikasikan resmi | Sleep setelah periode idle, wake otomatis saat ada request (lihat ADR-005) |
| Expo EAS Build | Tidak | 15 build Android + 15 build iOS per bulan | Build lokal selalu gratis |
| GitHub | Tidak | Actions gratis untuk repo publik | Digunakan untuk keep-alive & CI |
| Android Keystore | Tidak | — | File lokal, di-generate `keytool` atau EAS |
| `google-services.json` | Tidak | — | **Tidak diperlukan** karena memakai local notification |

**Aturan pengamanan biaya:** jangan mendaftarkan metode pembayaran pada layanan manapun di atas. Tanpa kartu terdaftar, skenario terburuk adalah layanan di-suspend — bukan tagihan.

---

*Dokumen ini bersifat hidup dan akan diperbarui seiring berjalannya pengembangan. Setiap perubahan keputusan arsitektur dicatat sebagai ADR baru di Bagian 10.*
