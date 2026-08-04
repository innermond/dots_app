# Volt Dots

A multi-tenant Go backend (module `github.com/innermond/dots`) for tracking **companies**, the **deeds** (goods/contracts you can supply, e.g. a purchased quantity at a unit price) they hold, and the **entries** (stock movements) that get **drained**/distributed against those deeds over time.

## Domain model

| Type | Package-level file | Description |
|---|---|---|
| `Company` | `api/company.go` | A legal entity (`longname`, `tin`, `rn`) owned by a user (`tid`). |
| `EntryType` | `api/entry_type.go` | A category of stock/resource (`code`, `unit`, `description`) owned by a user. |
| `Entry` | `api/entry.go` | A dated quantity of a given `EntryType` added for a `Company`. |
| `Deed` | `api/deed.go` | A quantity of a `Title`/`Unit` at a `UnitPrice`, belonging to a `Company`, that can be drained by entries. |
| `Drain` | `api/drain.go` | Links an `Entry` to a `Deed`, recording how much quantity of the deed was consumed. |
| `User` | `api/user.go` | Account identified by a KSUID, with `Powers` (see below) and OAuth `Auth` records. |
| `Auth` | `api/auth.go` | An OAuth identity (Google) or credential linked to a `User`. |
| `Power` | `api/power.go` | Authorization scopes: `do_anything`, `create_own`, `write_own`, `read_own`, `delete_own` (`api/autz.go` enforces them). |

Every domain type follows the same shape: a struct, a `Validate`/`Valid` method, a `...Filter` struct for querying, an `...Update` struct for partial updates, and a `...Service` interface implemented against Postgres.

The most iterated, business-critical logic is the **deed/entry-type distribution engine** (`api/postgres/deed.go`, `api/postgres/distribute.go`): given a wanted quantity per entry type for a company, it checks there's enough undrained quantity available and distributes the draw across existing entries using a selectable ordering strategy (`new_many`, `old_few`, `many_new`, ... — newest/oldest × largest/smallest quantity first).

## Repository layout

```
api/
  *.go                domain types & service interfaces (package dots)
  http/                HTTP transport: gorilla/mux router, handlers, session/auth middleware (package http)
    token/             paseto (v4 local) bearer token creation/verification
  postgres/            Postgres implementations of the service interfaces (package postgres), + *_test.go
  cmd/dotsd/           main.go — the server entrypoint / binary
  migrations/          golang-migrate SQL migrations + up.sh/down.sh/new.sh/force.sh helper scripts
  .env / .env.example  runtime configuration (see Configuration below)
  build                build script
  dist/server          compiled binary output (gitignored)
  README.md            local dev setup notes (migrations, docker, securecookie)
files/                 files copied into the Docker image (TLS certs for dots.volt.com)
Dockerfile             dev image: Go 1.20 + delve debugger
docker-compose.yml     runs the dots_api container, exposes 8080 (HTTP), 8443 (HTTPS), 40000 (delve)
LICENSE                MIT
```

## Architecture

* **`dots` (root `api` package)** defines the domain: structs, validation, and `XService` interfaces — no HTTP or SQL leaks in.
* **`http`** wires a `gorilla/mux` router to those service interfaces. Auth works two ways: a **bearer token** (paseto v4 local, via `TokenService`/`http/token`) or a **secure cookie session** (`gorilla/securecookie`). Route groups use `yesAuthenticate`/`noAuthenticate` middleware to require or forbid a logged-in user. Additional middleware: `reportPanic` (recover + report), `allowRequestsFromApp` (CORS allowlist for `dots.volt.com` / `localhost:3000`), and (dev-only) `devine`/`interceptAbort` for injecting fault responses/latency and tracing request cancellation. The whole router is wrapped in `http.TimeoutHandler` (10s) with tight read/write/idle timeouts on the underlying `http.Server`.
* **`postgres`** implements every `XService` against Postgres (`pgx`), including soft-delete (`deleted_at`), filter-based finds, stats/depletion aggregation, and the drain-distribution engine.
* **`cmd/dotsd`** is the composition root: loads `.env`, opens the DB, builds each service, wires them into the `http.Server`, and serves on `:8080`.

## Authorization model

Powers (`api/power.go`) are attached to a `User` and checked per-request via helpers in `api/autz.go`:

* `DoAnything` — unrestricted.
* `CreateOwn` / `WriteOwn` / `ReadOwn` / `DeleteOwn` — scoped to resources owned by the acting user (matched by `tid`/KSUID).

Multi-tenancy is additionally enforced at the database level: Postgres **row-level security** policies on `core.company`, `core.deed`, `core.drain`, `core.entry`, `core.entry_type` restrict rows to the caller's tenant (`core.get_tenent()`), and the app talks to Postgres through an `api` schema of `security_invoker` views over the `core` tables rather than the base tables directly.

## HTTP API (`/v1` prefix)

| Path | Methods | Auth |
|---|---|---|
| `/` | GET | optional (returns current session) |
| `/login` | GET, POST | must be logged out |
| `/logout` | GET | must be logged out route group |
| `/oauth/google`, `/oauth/google/callback` | GET | must be logged out |
| `/me` | GET | required |
| `/companies`, `/companies/{id}` | POST, GET, PATCH, DELETE | required |
| `/companies/stats` | GET | required |
| `/companies/depletion` | GET | required |
| `/entry-types`, `/entry-types/{id}` | POST, GET, PATCH, DELETE | required |
| `/entry-types?units` | GET | required — list distinct units in use |
| `/entry-types?stats&id&kind=default` | GET | required — per-entry-type stats |
| `/entries`, `/entries/{id}` | POST, GET, PATCH, DELETE | required |
| `/deeds`, `/deeds/{id}` | POST, GET, PATCH | required |
| `/drains` | POST | required — create/update a drain (also runs the distribution engine's ownership/quantity checks) |

Requests/responses are JSON. Unknown JSON fields in request bodies are rejected. All list/read endpoints accept a JSON filter body (offset/limit + per-field filters, including `deleted_at_from`/`deleted_at_to`).

## Database

Schema lives in `api/migrations` (golang-migrate, sequential `.up.sql`/`.down.sql` pairs, currently `001`–`004`). It's organized into three Postgres schemas:

* **`core`** — base tables: `user`, `auth`, `company`, `entry_type`, `entry`, `deed`, `drain`, plus `package`/`user_restriction` for plan-based limits. Row-level security is enabled on the tenant-owned tables. IDs are mostly `IDENTITY` columns; user/owner references (`tid`) use a custom `KSUID` domain.
* **`api`** — `security_invoker` views over `core` (filtering out soft-deleted rows) that the application actually queries, plus `api.entry_with_quantity_drained`, a view computing each entry's remaining (undrained) quantity — the input to the distribution engine.
* **`mock`** — seed/test data helpers (e.g. `mock.word`).

`001_init-schema` is a full schema dump (`pg_dump --schema-only`) that replaced the earlier incremental migrations; `002` adds the `api` views; `003` removes now-redundant triggers; `004` adds the drained-quantity view.

## Configuration

`cmd/dotsd` loads `api/.env` via `godotenv` at startup. See `api/.env.example` for the full list:

```
DOTS_DSN                     Postgres connection string
DOTS_GITHUB_CLIENT_ID        GitHub OAuth client id (not currently wired up)
DOTS_GITHUB_CLIENT_SECRET    GitHub OAuth client secret (not currently wired up)
DOTS_GOOGLE_CLIENT_ID        Google OAuth client id
DOTS_GOOGLE_CLIENT_SECRET    Google OAuth client secret
DOTS_TOKEN_SECRET            secret for paseto bearer tokens
DOTS_TOKEN_TTL               token time-to-live, in seconds
DOTS_TOKEN_PREFIX            prefix for issued tokens
```

Copy it to get started: `cp api/.env.example api/.env`.

A `.securecookie` file (two hex lines: hash key, block key) is required for cookie sessions — see "Local development" below to generate one.

## Local development

1. **Start Postgres** (and the app container): `docker compose up`
2. **Configure environment**: `cp api/.env.example api/.env` and fill in the values.
3. **Create `.securecookie`**:
   ```
   openssl rand -hex 64 > api/.securecookie && openssl rand -hex 32 >> api/.securecookie
   ```
4. **Run migrations** against the dockerized Postgres, using the helper scripts in `api/migrations` (they target `postgres.dots.volt.com`, matching the `docker-compose.yml` `extra_hosts` entry, with the `dots_owner` role):
   ```
   api/migrations/up.sh          # migrate up
   api/migrations/down.sh [n]    # migrate down n steps (default 1)
   api/migrations/new.sh <name>  # scaffold a new migration pair
   api/migrations/force.sh <n>   # force a migration version
   ```
   Or invoke `migrate` directly: `migrate -path api/migrations -database "postgresql://<user>:<password>@localhost:5432/dots?sslmode=disable" -verbose up`. Quote reserved words like `"user"` when writing raw SQL.
5. **Build & run the server**:
   ```
   cd api && ./build && ./dist/server
   ```
   (`./build debug` compiles with debug flags for delve; `docker-compose.yml` exposes port `40000` for remote debugging.)

To connect to the dockerized Postgres directly:
```
docker container exec -it dots-db-1 /bin/bash
su postgres
psql
\c dots
```

## Testing

Go tests live alongside the code: `api/postgres/*_test.go` (exercise the Postgres service implementations, including the distribution engine in `distribute_test.go`, against a real database), `api/http/server_test.go`, and `api/http/token/token_test.go`. Run with:

```
cd api && go test ./...
```
