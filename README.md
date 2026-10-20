# CARLITOS

cd backend && go run ./cmd/api


A production-ready **multi-vendor fashion & lifestyle e-commerce marketplace**.

CARLITOS separates three portals that share one backend and one database:

| Portal        | Base route | Dev URL                 |
| ------------- | ---------- | ----------------------- |
| Customer Store| `/`        | http://localhost:5173   |
| Seller Desk   | `/seller`  | http://localhost:5174   |
| Admin Desk    | `/admin`   | http://localhost:5175   |

> CARLITOS is inspired only by the general operational concepts of large Indian
> marketplaces. It contains no third-party source code, assets, logos, or branding.
> It has its own clean, premium, minimal identity.

---

## Tech stack

**Frontend:** Vue 3, TypeScript, Vite, Vue Router, Pinia, Tailwind CSS, typed Axios client, VueUse, Lucide icons, Zod.

**Backend:** Go, Fiber, REST, WebSocket (where useful), pgx, sqlc-style queries, PostgreSQL, Redis.

**Infrastructure:** Docker, Docker Compose, PostgreSQL, Redis, env-based config.

**Storage:** local disk — uploaded media is served by the API's `/media` route (no object storage service required).

---

## Monorepo layout

```
carlitos/
├── apps/
│   ├── customer-web/     # Vue 3 customer store  (port 5173)
│   ├── seller-web/       # Vue 3 seller desk     (port 5174)
│   └── admin-web/        # Vue 3 admin desk      (port 5175)
├── backend/
│   ├── cmd/              # api, migrate, seed entrypoints
│   ├── internal/         # domain modules (auth, catalog, orders, ...)
│   ├── migrations/       # SQL migrations
│   └── sql/              # sqlc queries / schema
├── packages/
│   ├── types/            # shared TS types
│   ├── api-client/       # typed API client
│   └── shared/           # shared utils / constants
├── infrastructure/
│   ├── docker/
│   └── nginx/
├── docs/
│   ├── architecture/
│   ├── api/
│   └── database/
├── docker-compose.yml
├── .env.example
└── README.md
```

---

## Quick start

> Full, step-by-step commands (install, migrate, seed, run all services) are
> finalized in the last part of the build. The skeleton below is the intended flow.

```bash
# 0. Configure environment
cp .env.example .env

# 1. Start infrastructure (Postgres + Redis)
docker compose up -d postgres redis

# 2. Backend: install deps, run migrations, seed, then run
cd backend
go mod download
go run ./cmd/migrate up
go run ./cmd/seed
go run ./cmd/api

# 3. Frontends (in separate terminals)
cd apps/customer-web && npm install && npm run dev   # http://localhost:5173
cd apps/seller-web   && npm install && npm run dev   # http://localhost:5174
cd apps/admin-web    && npm install && npm run dev   # http://localhost:5175
```

---

## Documentation

- [`docs/architecture/`](docs/architecture) — system & module architecture
- [`docs/api/`](docs/api) — API architecture and endpoints
- [`docs/database/`](docs/database) — schema and entity relationships

Lifecycles documented: **authentication flow · roles/RBAC · order lifecycle ·
seller lifecycle · product lifecycle**.
