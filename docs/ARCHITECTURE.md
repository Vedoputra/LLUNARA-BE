# Arsitektur — LLunara API

## Struktur Folder

```
cmd/api/                    # Entry point aplikasi
internal/
├── config/                 # Load & validasi environment variable
├── handler/                 # HTTP layer: parsing request, penulisan response
├── service/                 # Business logic & kalkulasi
├── repository/               # Akses database (query, tanpa business logic)
├── model/                    # Domain entities, request/response DTO
├── middleware/                # Auth, logging, CORS, rate limit
└── pkg/
    ├── apierror/              # Struktur error terstandardisasi ke klien
    └── validator/              # Wrapper validasi input
migrations/                  # SQL migration files, dijalankan berurutan
```

## Arah Dependency

```
handler  →  service  →  repository  →  database
   ↓          ↓             ↓
        model (boleh diakses semua layer)
```

Aturan yang wajib dipatuhi:

- `handler` **tidak boleh** mengakses `repository` secara langsung — selalu lewat `service`.
- `repository` **tidak boleh** mengimpor `service` atau `handler`.
- `model` **tidak boleh** mengimpor layer manapun — hanya struct data murni.

## Tanggung Jawab Tiap Layer

| Layer | Boleh | Tidak Boleh |
|---|---|---|
| `handler` | Parsing request, validasi format, penulisan response | Business logic, akses database |
| `service` | Aturan bisnis, kalkulasi, orkestrasi | Menyentuh `http.Request` atau `http.ResponseWriter` |
| `repository` | Query database | Business logic |
| `model` | Definisi struktur data | Apapun selain itu |

## Kenapa `chi`

`go-chi/chi/v5` dipilih karena kompatibel penuh dengan `net/http` standar (tidak mengunci ke abstraksi custom), middleware-nya composable, dan cukup ringan untuk berjalan nyaman di Render free tier (512 MB RAM).

## Ringkasan Architecture Decision Records

Detail lengkap ADR ada di `docs/PRD.md` Bagian 10. Ringkasan:

| ADR | Keputusan |
|---|---|
| ADR-001 | Hybrid: Supabase langsung untuk read sederhana, Go API untuk seluruh write & read yang butuh kalkulasi |
| ADR-002 | Cloud-only, tanpa local-first / offline database, untuk menghindari kompleksitas sync |
| ADR-003 | Local notification (`expo-notifications`), bukan push notification server |
| ADR-004 | Hosting di Render free tier, menerima trade-off cold start 30–60 detik demi Rp 0 biaya operasional |

## Diagram Arsitektur

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
            │        │   Go API (Render)    │
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
