# Spesifikasi Backend — Reminder & Taman Luna

Dokumen ini melengkapi dua fitur yang **ada di desain UI tetapi belum punya endpoint** di backend LLunara:

1. **Reminder** — preferensi pengingat (tabel sudah ada, kode belum)
2. **Taman Luna** — gamifikasi ringan (belum ada apa pun)

Ditulis agar bisa langsung dikerjakan sebagai lanjutan `EXECUTION_PLAN_BE.md`, mengikuti pola layer yang sama (handler → service → repository → model) dan aturan keamanan yang sama (`user_id` selalu dari JWT, filter `user_id` di setiap query, dsb).

---

# FITUR 1 — Reminder

## 1.1 Klarifikasi: ini Local Notification, bukan Push

Ya — reminder LLunara adalah **local notification**: notifikasi dijadwalkan dan dimunculkan **di perangkat** oleh OS (via `expo-notifications`), bukan dikirim (push) dari server. Ini keputusan yang sudah diambil di PRD (ADR-003) dan konsekuensinya penting untuk memahami peran backend di sini:

**Peran backend hanya menyimpan preferensi reminder, bukan mengirim notifikasi.**

Alur lengkapnya:

```
1. User atur preferensi reminder di layar Pengaturan (nyalakan/matikan,
   pilih jam obat, dsb)
        │
        ▼
2. FE menyimpan preferensi itu ke backend  ──►  tabel `reminders`
        │
        ▼
3. FE membaca prediksi siklus (GET /cycles/prediction) + preferensi reminder
        │
        ▼
4. FE menjadwalkan notifikasi LOKAL di perangkat via expo-notifications
   (misal: "2 hari sebelum next_period_start, jam 09:00")
        │
        ▼
5. OS perangkat yang memunculkan notifikasi pada waktunya — server tidak
   terlibat sama sekali di titik ini, bahkan saat perangkat offline
```

**Kenapa preferensi tetap perlu disimpan di server** (bukan hanya di perangkat)? Supaya pengaturan reminder tidak hilang saat user ganti/instal ulang perangkat. Ini konsisten dengan arsitektur cloud-only (ADR-002): sumber kebenaran ada di server, perangkat hanya penjadwal.

**Yang backend TIDAK lakukan:** backend tidak punya cron, tidak mengirim FCM/push, tidak tahu kapan notifikasi benar-benar muncul. Semua penjadwalan ada di FE. Jadi jangan buat endpoint semacam "kirim notifikasi" — itu di luar arsitektur.

## 1.2 Tabel: `reminders` (sudah ada, tidak perlu tabel baru)

Tabel ini **sudah dibuat** di `migrations/001_init_schema.up.sql` lengkap dengan RLS di `002`. Tidak perlu migration baru. Strukturnya:

| Kolom | Tipe | Keterangan |
|---|---|---|
| `id` | uuid, PK | |
| `user_id` | uuid, FK → `auth.users`, `ON DELETE CASCADE` | Diisi dari JWT, tidak dari client |
| `type` | enum `reminder_type` | `period_upcoming` / `fertile_window` / `medication` / `checkup` |
| `is_enabled` | boolean, default `true` | Sakelar nyala/mati |
| `time_of_day` | time, nullable | Jam munculnya notifikasi (mis. untuk obat). Format `HH:MM` |
| `days_before` | int, nullable | Untuk reminder berbasis prediksi (mis. `period_upcoming` H-2 → `days_before = 2`) |
| `custom_message` | text, nullable | Pesan kustom (mis. nama obat) |
| `created_at` / `updated_at` | timestamptz | |

**Relasi:** hanya ke `auth.users` (via `user_id`). Tidak ada relasi ke `cycles` atau `daily_logs` — reminder berdiri sendiri sebagai preferensi. Keterkaitan dengan prediksi siklus terjadi di FE saat penjadwalan (FE menggabungkan `reminders` + `GET /cycles/prediction`), bukan di database.

**Catatan tentang `type`:** satu user idealnya punya maksimal satu baris per `type` (satu preferensi "period_upcoming", satu "fertile_window", dst) — kecuali `medication` yang mungkin ingin lebih dari satu (beberapa obat, jam berbeda). Karena skema saat ini **tidak** punya unique constraint pada `(user_id, type)`, tangani ini di service layer (lihat 1.4). Kalau nanti diputuskan `medication` boleh banyak, biarkan tanpa unique constraint dan bedakan lewat `custom_message`.

## 1.3 Model (`internal/model/reminder.go`) — file baru

Ikuti pola tiga-bentuk yang sudah dipakai model lain (domain / request DTO / response DTO).

```go
package model

import (
    "time"
    "github.com/google/uuid"
)

type ReminderType string

const (
    ReminderPeriodUpcoming ReminderType = "period_upcoming"
    ReminderFertileWindow  ReminderType = "fertile_window"
    ReminderMedication     ReminderType = "medication"
    ReminderCheckup        ReminderType = "checkup"
)

// Domain
type Reminder struct {
    ID            uuid.UUID
    UserID        uuid.UUID
    Type          ReminderType
    IsEnabled     bool
    TimeOfDay     *string // "HH:MM", nullable
    DaysBefore    *int
    CustomMessage *string
    CreatedAt     time.Time
    UpdatedAt     time.Time
}

// Request DTO — dipakai untuk upsert
type UpsertReminderRequest struct {
    Type          string  `json:"type" validate:"required,oneof=period_upcoming fertile_window medication checkup"`
    IsEnabled     bool    `json:"is_enabled"`
    TimeOfDay     *string `json:"time_of_day,omitempty" validate:"omitempty,datetime=15:04"`
    DaysBefore    *int    `json:"days_before,omitempty" validate:"omitempty,min=0,max=14"`
    CustomMessage *string `json:"custom_message,omitempty" validate:"omitempty,max=200"`
}

// Response DTO
type ReminderResponse struct {
    ID            string  `json:"id"`
    Type          string  `json:"type"`
    IsEnabled     bool    `json:"is_enabled"`
    TimeOfDay     *string `json:"time_of_day,omitempty"`
    DaysBefore    *int    `json:"days_before,omitempty"`
    CustomMessage *string `json:"custom_message,omitempty"`
    CreatedAt     string  `json:"created_at"`
    UpdatedAt     string  `json:"updated_at"`
}
```

## 1.4 Endpoint

Semua di bawah `/api/v1`, wajib JWT, mengikuti format amplop `{ "data": ... }` dan format error yang sama seperti endpoint lain.

### `GET /api/v1/reminders`

Daftar seluruh preferensi reminder milik user.

**Response 200:**
```json
{
  "data": [
    {
      "id": "…",
      "type": "period_upcoming",
      "is_enabled": true,
      "time_of_day": "09:00",
      "days_before": 2,
      "custom_message": null,
      "created_at": "…",
      "updated_at": "…"
    }
  ]
}
```

Kalau user belum pernah mengatur apa pun, kembalikan `{ "data": [] }` (bukan error). FE bisa menampilkan default (semua mati) dari array kosong ini.

### `PUT /api/v1/reminders`  (upsert)

Menyimpan/memperbarui satu preferensi reminder. Pakai `PUT` (bukan `POST`) karena idempoten berdasarkan `type` untuk tipe non-medication.

**Body:**
```json
{
  "type": "period_upcoming",
  "is_enabled": true,
  "time_of_day": "09:00",
  "days_before": 2
}
```

**Aturan service:**
- Untuk `type` selain `medication`: upsert berdasarkan `(user_id, type)` — kalau sudah ada, update; kalau belum, insert. Satu baris per tipe.
- Untuk `medication`: karena bisa lebih dari satu, gunakan endpoint terpisah di bawah (`POST`/`DELETE`), **bukan** `PUT` ini. Atau, kalau ingin sederhana untuk v1, batasi `medication` juga jadi satu baris dan pakai `PUT` yang sama — putuskan saat develop. Rekomendasi: v1 satu baris per tipe (paling sederhana), multi-medication masuk roadmap.

**Response 200:** `{ "data": ReminderResponse }`

**Errors:** `422 VALIDATION_ERROR` (type/jam/days_before tidak valid).

### `DELETE /api/v1/reminders/{id}`

Menghapus satu preferensi reminder (mis. user mematikan total reminder obat). Alternatif dari mematikan lewat `is_enabled: false` — dua-duanya valid; FE bisa pilih pakai toggle (`PUT` dengan `is_enabled`) untuk on/off, dan `DELETE` hanya kalau benar-benar ingin menghapus baris.

**Response 204.** Errors: `404 NOT_FOUND` (id bukan milik user / tidak ada).

## 1.5 Repository & Service — poin penting

- **Repository** (`internal/repository/reminder_repository.go`): `ListByUser`, `UpsertByType`, `GetByID`, `Delete`. Setiap query **wajib** memfilter `user_id` (backend pakai service role yang mem-bypass RLS).
- **Service** (`internal/service/reminder_service.go`): validasi kombinasi field yang logis, contoh:
  - `medication` sebaiknya punya `time_of_day` (kalau kosong, FE tidak tahu jam berapa menjadwalkan). Beri `422` atau default yang jelas.
  - `period_upcoming` / `fertile_window` mengandalkan `days_before`, bukan `time_of_day` wajib (default jam bisa ditentukan FE).
  - `checkup` boleh pakai `custom_message` + `time_of_day`.
  - Ini validasi lunak — tidak perlu kaku, cukup mencegah data yang tidak bisa dipakai FE.

## 1.6 Routing (tambahan di `cmd/api/main.go`)

```go
r.Get("/reminders", reminderHandler.ListReminders)
r.Put("/reminders", reminderHandler.UpsertReminder)
r.Delete("/reminders/{id}", reminderHandler.DeleteReminder)
```

---

# FITUR 2 — Taman Luna (Gamifikasi)

## 2.1 Apa yang ditampilkan UI

Dari desain, layar Taman Luna menampilkan tiga hal:

1. **Kebun yang tumbuh** — ilustrasi taman yang makin semarak seiring user mencatat. Ada label seperti "Kebunmu sedang mekar" dan "4 bunga baru minggu ini".
2. **Kartu konsistensi** — "Bulan ini kamu sudah mencatat 12 hari" dengan progress arc.
3. **Koleksi stiker suasana** — stiker mood yang "terkumpul" vs "belum" (placeholder bergaris putus-putus).

Prinsip wajib (dari PRD 4.5 & DESIGN.md): **positive-only**. Tidak ada tanaman layu, tidak ada streak yang runtuh, tidak ada bahasa menyalahkan. Backend harus mencerminkan ini — tidak menghitung "berapa hari bolong", tidak ada konsep "streak putus".

## 2.2 Keputusan arsitektur: hitung dari data yang sudah ada

**Rekomendasi: TIDAK perlu tabel baru.** Seluruh angka di Taman Luna bisa **diturunkan (derived)** dari data yang sudah ada — terutama `daily_logs`. Ini pilihan yang lebih bersih karena:

- Tidak ada duplikasi state (tidak perlu menjaga "jumlah bunga" tetap sinkron dengan jumlah catatan)
- Tidak ada risiko data gamifikasi melenceng dari data asli
- Lebih sedikit yang bisa rusak

Yang perlu dibuat hanya **satu endpoint read-only** yang mengagregasi. Logikanya:

| Elemen UI | Diturunkan dari |
|---|---|
| "Mencatat X hari bulan ini" | `COUNT(DISTINCT date)` dari `daily_logs` untuk user di bulan berjalan |
| Total pertumbuhan kebun | Total `COUNT(DISTINCT date)` dari `daily_logs` sepanjang waktu |
| "N bunga baru minggu ini" | `COUNT(DISTINCT date)` dari `daily_logs` dalam 7 hari terakhir |
| Stiker mood "terkumpul" | `DISTINCT mood` yang pernah tercatat di `daily_logs` (mood non-null) |
| Stiker mood "belum" | Daftar mood preset dikurangi yang sudah terkumpul |

Perlu **satu keputusan produk** sebelum develop: **definisi "1 bunga"**. Contoh aturan sederhana yang cukup: *1 hari tercatat = 1 tanaman di kebun*. Atau *setiap 3 hari tercatat = 1 bunga mekar*. Ini bebas ditentukan — yang penting deterministik dan bisa dihitung dari `daily_logs`. Rekomendasi termudah: **1 hari tercatat = 1 unit pertumbuhan**, lalu FE yang memetakan jumlah unit ke visual kebun (berapa unit untuk memunculkan bunga baru adalah urusan tampilan).

### Kapan tabel baru diperlukan?

Hanya kalau nanti kamu ingin gamifikasi yang **tidak bisa diturunkan** dari data catatan, misalnya:
- Stiker yang didapat dari aksi selain mencatat (mis. "buka app 7 hari", "isi profil")
- Item kebun yang bisa dibeli/ditukar
- Progress yang harus "beku" (mis. badge yang sudah didapat tidak boleh hilang walau data catatan dihapus)

Kalau salah satu itu masuk scope, baru buat tabel. Desainnya ada di 2.5 sebagai opsi, **tapi untuk v1 tidak dipakai**.

## 2.3 Endpoint

### `GET /api/v1/garden`

Satu endpoint yang mengembalikan seluruh data Taman Luna, sudah terhitung di server.

**Response 200:**
```json
{
  "data": {
    "total_logged_days": 34,
    "logged_days_this_month": 12,
    "new_this_week": 4,
    "collected_moods": ["senang", "tenang", "sensitif"],
    "uncollected_moods": ["biasa", "cemas", "sedih", "mudah marah"],
    "message": "Setiap hari kecil berarti. Tidak apa-apa kalau ada yang terlewat."
  }
}
```

Penjelasan field:
- `total_logged_days` — total hari unik yang punya catatan (untuk skala kebun keseluruhan).
- `logged_days_this_month` — untuk kartu "Bulan ini kamu sudah mencatat X hari".
- `new_this_week` — hari unik tercatat dalam 7 hari terakhir (untuk "N bunga baru minggu ini").
- `collected_moods` — mood unik yang pernah dicatat, untuk menandai stiker mana yang sudah "terkumpul".
- `uncollected_moods` — daftar mood preset yang belum pernah dicatat (untuk placeholder). Backend yang menghitung selisihnya supaya FE tidak perlu tahu daftar lengkap mood preset.
- `message` — kalimat hangat dari Luna. Bisa statis, atau backend memilih dari beberapa kalimat. **Wajib** bernada positive-only. Boleh juga dipindah ke FE kalau ingin backend murni data — bebas.

**Selalu 200**, termasuk untuk user baru tanpa catatan sama sekali (semua angka `0`, `collected_moods` kosong, `uncollected_moods` berisi semua mood preset). Bukan error — konsisten dengan pola `GET /cycles/prediction`.

**Catatan penting (positive-only):** endpoint ini **tidak boleh** mengembalikan apa pun yang mengukur ketidakhadiran — tidak ada `missed_days`, tidak ada `current_streak` yang bisa putus, tidak ada `days_since_last_log` yang ditonjolkan. Kalau butuh konsep "aktif", gunakan framing positif (`total_logged_days`), bukan negatif.

## 2.4 Implementasi — poin penting

- **Service** (`internal/service/garden_service.go`): seluruh perhitungan sebisa mungkin sebagai agregasi SQL (`COUNT(DISTINCT date)`, `DISTINCT mood`) supaya hemat memori — sama seperti insight service. Untuk `uncollected_moods`, service menyimpan daftar mood preset (konstanta) lalu mengurangi dengan `collected_moods`.
- **Repository**: tambahkan query agregasi di `daily_log_repository.go` (atau repository baru `garden_repository.go`) — mis. `CountDistinctLoggedDays(ctx, userID, from, to)` dan `DistinctMoods(ctx, userID)`. Semua **wajib** filter `user_id`.
- **Handler** (`internal/handler/garden_handler.go`): tipis, hanya ambil `user_id` dari context, panggil service, bungkus `{ "data": ... }`.
- **Mood preset**: definisikan daftar mood kanonik sebagai konstanta di satu tempat (mis. `internal/model/mood.go`) agar `uncollected_moods` konsisten. Daftar dari PRD: `senang`, `tenang`, `biasa`, `sensitif`, `cemas`, `sedih`, `mudah marah`.

## 2.5 (OPSIONAL, TIDAK UNTUK v1) Tabel jika gamifikasi diperluas

Hanya kalau nanti butuh state yang tidak bisa diturunkan dari `daily_logs`. Jangan buat di v1.

```sql
-- HANYA JIKA DIPERLUKAN DI MASA DEPAN — jangan jalankan di v1
create table garden_rewards (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references auth.users(id) on delete cascade,
  reward_type text not null,       -- mis. 'sticker', 'plant', 'badge'
  reward_key text not null,        -- identitas item, mis. 'sticker_senang'
  earned_at timestamptz not null default now(),
  unique (user_id, reward_type, reward_key)
);
```

Relasinya: `user_id` → `auth.users`, dengan RLS pola sama seperti tabel lain (4 policy `auth.uid() = user_id`). Tapi sekali lagi — **untuk v1 semua diturunkan dari `daily_logs`, tabel ini tidak dibuat.**

## 2.6 Routing (tambahan di `cmd/api/main.go`)

```go
r.Get("/garden", gardenHandler.GetGarden)
```

---

# Ringkasan Perubahan yang Dibutuhkan

| Item | Reminder | Taman Luna |
|---|---|---|
| Tabel baru? | Tidak (`reminders` sudah ada) | Tidak (diturunkan dari `daily_logs`) |
| Migration baru? | Tidak | Tidak |
| Model baru | `reminder.go` | (opsional) `mood.go` untuk konstanta |
| Repository baru | `reminder_repository.go` | query agregasi di `daily_log_repository.go` atau `garden_repository.go` |
| Service baru | `reminder_service.go` | `garden_service.go` |
| Handler baru | `reminder_handler.go` | `garden_handler.go` |
| Endpoint | `GET/PUT/DELETE /reminders` | `GET /garden` |
| Catatan | Backend hanya simpan preferensi; notifikasi dijadwalkan lokal di FE | Positive-only; tidak ada streak/missed days |

## Soal "Mood Swing" di layar Statistik

Ini bukan masalah skema, cukup disesuaikan saat develop FE. Yang perlu diingat saat implementasi:

- Data gejala per fase datang dari `GET /insights/symptoms` (key = nama gejala dari tabel `symptoms`).
- Data mood per fase datang dari `GET /insights/mood` (key = nama mood: `senang`, `tenang`, dst).
- "Mood Swing" bukan salah satu dari keduanya — itu label karangan saat generate desain. Saat develop, ganti baris "Mood Swing" itu dengan salah satu gejala asli (mis. dari `/insights/symptoms`) **atau** pindahkan ke visualisasi mood (`/insights/mood`) dengan label mood yang benar. Tidak ada perubahan backend yang diperlukan.
