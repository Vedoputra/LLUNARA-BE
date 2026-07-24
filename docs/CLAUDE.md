# CLAUDE.md — LLunara

Panduan untuk Claude Code saat bekerja pada project ini.

> **Cara pakai:** file ini dirancang untuk disalin ke **kedua** repository (`llunara-api` dan `llunara-mobile`). Bagian yang bersifat umum berlaku di keduanya. Bagian yang khusus repo ditandai dengan jelas — kamu boleh menghapus bagian yang tidak relevan di masing-masing repo.

---

## 1. Konteks Project

LLunara adalah aplikasi mobile untuk mencatat dan memantau siklus menstruasi, dilengkapi analitik personal, pengingat siklus, dan tracking wellness harian.

**Sifat project:**
- Portfolio project, dikerjakan solo developer
- Pengguna nyata: 1–2 orang
- Target kualitas: mendekati standar industri, bukan sekadar demo
- **Batasan mutlak:** seluruh stack berjalan di tier gratis, tanpa kartu kredit di layanan manapun

**Dokumen rujukan:**
- `PRD.md` — spesifikasi produk lengkap, skema database, kontrak API
- `EXECUTION_PLAN_BE.md` — rencana eksekusi backend, task per task
- `EXECUTION_PLAN_FE.md` — rencana eksekusi frontend, task per task

Sebelum mengerjakan sesuatu, periksa dokumen-dokumen ini lebih dulu. Jangan berimprovisasi di luar spesifikasi tanpa konfirmasi.

---

## 2. Arsitektur

```
Expo App ──┬── Supabase (langsung)  → READ sederhana + Auth
           └── Go API (Vercel)      → WRITE + kalkulasi
                    └── Supabase (service role)
```

### Aturan pembagian jalur data — WAJIB DIPATUHI

| Operasi | Jalur |
|---|---|
| Login, register, refresh token | Supabase langsung |
| Baca log harian mentah | Supabase langsung |
| Baca daftar gejala | Supabase langsung |
| **Semua operasi tulis** | Go API |
| Prediksi siklus | Go API |
| Insight & analytics | Go API |
| Export laporan | Go API |

**Jika ragu, gunakan Go API.** Membaca langsung dari Supabase adalah optimasi terbatas, bukan default.

### Tech Stack

**Backend (`llunara-api`)**

| Komponen | Pilihan |
|---|---|
| Bahasa | Go (versi stabil terbaru) |
| Router | `go-chi/chi/v5` |
| Database | `jackc/pgx/v5` |
| JWT | `golang-jwt/jwt/v5` |
| Validasi | `go-playground/validator/v10` |
| Logging | `log/slog` (stdlib) |
| PDF | `johnfercher/maroto/v2` |
| Hosting | Vercel (Hobby plan, Go Framework Preset) |

**Frontend (`llunara-mobile`)**

| Komponen | Pilihan |
|---|---|
| Framework | Expo + TypeScript (mode strict) |
| Routing | Expo Router |
| Server state | TanStack Query |
| Client state | Zustand |
| Storage | `expo-secure-store` |
| Notifikasi | `expo-notifications` (lokal) |
| Kalender | `react-native-calendars` |
| Grafik | `react-native-gifted-charts` |
| Form | `react-hook-form` + `zod` |

**Database:** Supabase (PostgreSQL) dengan Row Level Security aktif di seluruh tabel.

---

## 3. Aturan Keamanan — Tidak Dapat Dinegosiasikan

Ini aplikasi data kesehatan reproduksi. Kesalahan di area ini lebih serius daripada bug fungsional.

1. **`user_id` hanya berasal dari JWT yang sudah diverifikasi.** Backend tidak boleh menerima `user_id` dari body request, query parameter, header kustom, atau sumber lain manapun.

2. **Setiap query di repository wajib memfilter `user_id`.** Backend memakai service role key yang mem-bypass RLS, jadi filter ini adalah satu-satunya lapisan pengaman.

3. **`SUPABASE_SECRET_KEY` (nama baru Supabase untuk `service_role key`) hanya boleh berada di environment variable backend.** Tidak boleh muncul di kode frontend, log, pesan error, commit, maupun dokumentasi.

4. **Token disimpan di `expo-secure-store`**, tidak pernah di AsyncStorage.

5. **Jangan pernah mencatat data sensitif ke log:** token, password, isi catatan harian, detail gejala. Cukup `user_id` untuk penelusuran.

6. **RLS wajib aktif di seluruh tabel.** Tidak ada pengecualian.

7. **Pesan error ke klien tidak boleh membocorkan detail internal.** Error asli dicatat di log; klien menerima pesan yang aman dan generik.

8. **Penghapusan akun adalah hard delete.** Tidak ada soft delete untuk data kesehatan.

9. **Jangan pernah menyarankan atau menambahkan layanan berbayar** tanpa persetujuan eksplisit. Ini termasuk Redis, layanan monitoring pihak ketiga, penyimpanan berbayar, atau apapun yang membutuhkan kartu kredit.

---

## 4. Konvensi Backend (`llunara-api`)

### Arah dependency

```
handler  →  service  →  repository  →  database
```

- `handler` **tidak boleh** mengakses `repository` secara langsung
- `repository` **tidak boleh** mengimpor `service` atau `handler`
- `model` **tidak boleh** mengimpor layer manapun

### Tanggung jawab tiap layer

| Layer | Boleh | Tidak Boleh |
|---|---|---|
| `handler` | Parsing request, validasi format, penulisan response | Business logic, akses database |
| `service` | Aturan bisnis, kalkulasi, orkestrasi | Menyentuh `http.Request` atau `http.ResponseWriter` |
| `repository` | Query database | Business logic |
| `model` | Definisi struktur data | Apapun selain itu |

### Aturan penulisan kode

- Selalu gunakan parameterized query. Tidak ada string concatenation dalam pembuatan SQL
- Seluruh fungsi yang melakukan I/O menerima `context.Context` sebagai parameter pertama
- Bungkus error dengan konteks: `fmt.Errorf("create cycle: %w", err)`
- Gunakan `apierror` package untuk seluruh error yang dikembalikan ke klien
- Operasi yang menyentuh beberapa tabel harus berada dalam satu transaksi
- Logic kalkulasi ditulis sebagai pure function agar mudah diuji — terutama `prediction_service.go`

### Testing

- Seluruh fungsi di `service` wajib punya unit test
- `prediction_service.go` wajib mencakup kasus batas berikut:
  - User tanpa riwayat sama sekali
  - Satu siklus tercatat
  - Enam siklus teratur
  - Enam siklus dengan variasi tinggi
  - Riwayat yang mengandung outlier
  - Siklus yang melewati pergantian tahun
- Jalankan test dengan flag `-race`

### Perintah

```bash
make run       # jalankan lokal
make build     # compile binary
make test      # jalankan seluruh test
make lint      # gofmt + go vet
```

---

## 5. Konvensi Frontend (`llunara-mobile`)

### Struktur

- `app/` hanya berisi komposisi layar. Business logic tidak boleh berada di sini
- Setiap pemanggilan data dibungkus custom hook di `src/hooks/`
- Komponen di `src/components/ui/` tidak boleh memanggil API secara langsung
- Seluruh warna, spasi, dan tipografi diambil dari `src/constants/theme.ts` — tidak ada nilai literal di komponen

### Penanganan state jaringan

Setiap layar yang mengambil data **wajib** menangani empat kondisi:

1. **Loading** — gunakan `LoadingState` dengan pesan progresif
2. **Error** — gunakan `ErrorState` dengan tombol coba lagi
3. **Empty** — gunakan `EmptyState` dengan penjelasan dan ajakan tindakan
4. **Success** — tampilan normal

Layar yang hanya menangani loading dan success dianggap **belum selesai**.

### Penanganan cold start — penting

Backend berjalan di Vercel Hobby plan (Go Framework Preset, model serverless "Fluid Compute") dan dapat cold start setelah periode benar-benar idle — durasi pasti tidak dipublikasikan resmi oleh Vercel, jangan diasumsikan selalu cepat.

Konsekuensinya:

- Timeout HTTP client diset **60 detik**, bukan 10 detik
- `LoadingState` mengubah pesannya secara progresif:
  - 0–5 detik: spinner biasa
  - 5–15 detik: "Menghubungkan ke server…"
  - Lebih dari 15 detik: "Server sedang aktif kembali, mohon tunggu sebentar" (angka ini konservatif — anggap sebagai jaring pengaman, bukan durasi cold start Vercel yang sebenarnya, karena Vercel tidak mempublikasikan angka resmi)
- Dashboard menerapkan **pemuatan bertahap**: data dari Supabase tampil lebih dulu (cepat), data dari Go API menyusul dengan skeleton pada bagiannya saja
- Layar tidak boleh kosong sambil menunggu Go API

### Konvensi query key TanStack Query

```
['cycles']
['cycles', 'prediction']
['daily-logs', from, to]
['insights', 'summary']
['insights', 'symptoms', months]
['symptoms']
['wellness', from, to]
```

Setelah operasi tulis, lakukan invalidasi pada key yang relevan.

### Aksesibilitas

- Setiap elemen interaktif punya `accessibilityLabel` dan `accessibilityRole`
- Area sentuh minimal 44×44 pt
- Rasio kontras minimal 4.5:1
- Dukung ukuran font sistem

---

## 6. Aturan Produk & Nada Bahasa

Karena ini menyangkut kesehatan dan citra tubuh, aturan berikut berlaku pada seluruh teks yang tampil di aplikasi:

**Yang harus dilakukan:**
- Gunakan bahasa Indonesia yang netral dan tidak menghakimi
- Sajikan insight secara **deskriptif**, bukan preskriptif
- Tampilkan tingkat keyakinan prediksi secara jujur
- Cantumkan disclaimer medis pada onboarding dan halaman insight

**Yang tidak boleh dilakukan:**
- Membuat klaim medis, diagnosis, atau saran pengobatan
- Menggunakan kata "menyebabkan", "disarankan", "tidak normal", "bermasalah"
- Menampilkan target harian yang ditentukan sistem pada fitur wellness
- Menampilkan mekanik streak atau pesan yang menyalahkan saat user melewatkan pencatatan
- Menampilkan evaluasi terhadap berat badan atau indikator "ideal"
- Menyebut informasi sensitif secara eksplisit pada judul notifikasi

**Contoh perbedaan:**

| Tepat | Tidak tepat |
|---|---|
| "Kram paling sering kamu catat pada fase menstruasi." | "Kram kamu disebabkan oleh fase menstruasi." |
| "Panjang siklusmu cukup bervariasi." | "Siklusmu tidak normal." |
| "Perkiraan awal — akurasi meningkat setelah beberapa siklus tercatat." | "Prediksi kurang akurat." |

---

## 7. Cara Bekerja pada Project Ini

### Sebelum mulai

1. Baca task yang relevan di `EXECUTION_PLAN_BE.md` atau `EXECUTION_PLAN_FE.md`
2. Periksa apakah dependensinya (field **Butuh** / **Blocking**) sudah terpenuhi
3. Periksa `PRD.md` untuk detail spesifikasi, terutama skema database dan kontrak API

### Saat mengerjakan

- Kerjakan **satu task sampai tuntas** sebelum berpindah. Jangan mengerjakan beberapa task sekaligus
- Patuhi kriteria di field **Selesai Jika**. Task belum selesai kalau kriteria itu belum terpenuhi
- Kalau spesifikasi terasa ambigu, **tanyakan** — jangan berasumsi lalu melanjutkan
- Kalau menemukan cara yang lebih baik daripada yang tertulis di plan, sampaikan alasannya dan tunggu konfirmasi sebelum menyimpang

### Yang tidak boleh dilakukan tanpa konfirmasi

- Menambahkan dependency baru
- Mengubah skema database setelah Fase 1 selesai
- Mengubah kontrak API yang sudah dipakai frontend
- Menambahkan layanan eksternal apapun
- Mengubah aturan pembagian jalur data di Bagian 2

### Commit

Gunakan format conventional commit dengan referensi ID task:

```
feat(cycle): implement overlap validation [BE-3.3]
fix(auth): handle expired token refresh [FE-1.4]
docs: add architecture diagram
test(prediction): add outlier edge cases [BE-4.1]
```

---

## 8. Keterbatasan yang Diketahui

Ini bukan bug. Ini konsekuensi sadar dari pilihan arsitektur, dan sudah didokumentasikan sebagai ADR di `PRD.md` Bagian 10.

| Keterbatasan | Penanganan |
|---|---|
| Backend dapat cold start setelah idle (Vercel Hobby plan) | `LoadingState` progresif, timeout 60 detik, pemuatan bertahap |
| Rate limiter in-memory tidak konsisten 100% lintas instance saat concurrency scaling Vercel terjadi | Dampak diabaikan untuk skala 1–2 pengguna; didokumentasikan sebagai keterbatasan yang diketahui |
| Supabase project ter-pause setelah 7 hari idle | GitHub Action keep-alive setiap 3 hari |
| Aplikasi tidak berfungsi offline (cloud-only, lihat ADR-002) | `OfflineBanner`, aksi tulis dinonaktifkan dengan penjelasan |
| Tidak ada automated backup di free tier | Fitur export manual sebagai cadangan |
| EAS Build terbatas 15 build per platform per bulan | Uji melalui Expo Go semaksimal mungkin sebelum build |

**Jangan mencoba "memperbaiki" keterbatasan ini dengan menambahkan layanan berbayar.** Penanganan yang jujur dan terkendali justru merupakan bagian dari nilai project ini.

---

## 9. Definition of Done

Sebuah task dianggap selesai jika:

- [ ] Seluruh kriteria di field **Selesai Jika** terpenuhi
- [ ] Menangani kondisi loading, error, dan empty (untuk frontend)
- [ ] Logic bisnis punya unit test (untuk backend)
- [ ] Aman terhadap akses lintas user (diverifikasi dengan dua akun uji)
- [ ] Berfungsi pada mode terang dan gelap (untuk frontend)
- [ ] Tidak ada credential yang ter-commit
- [ ] Terdokumentasi di README atau `docs/API.md` bila relevan
