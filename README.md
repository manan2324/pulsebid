# PulseBid

A real-time bidding exchange in Go. The [build plan](PulseBid%20Build%20Plan.md)
defines the contracts and phased implementation.

**Current status:** All 13 phases (Phase 0 through Phase 12) **100% complete**. The platform includes
the core `POST /v1/bid` exchange engine on `:8080` with in-memory targeting evaluation,
parallel fan-out with bulkheads and circuit breakers, second-price auctions, atomic
Redis budget reservation and pacing, transparent 1x1 GIF impression pixel tracking
(`GET /v1/events/impression`), HTTP 302 click tracking (`GET /v1/events/click`), HMAC-SHA256
signed event tokens, durable Kafka publishing (`pulsebid.events.v1`), high-throughput
event consumer with atomic deduplication and hourly rollups in PostgreSQL, reporting API on `:8083`,
comprehensive resilience hardening (bidder fault isolation, breaker tripping/probing,
bulkheads, fail-closed budget safety, and graceful shutdown), complete Prometheus metrics
collection across all services, end-to-end distributed tracing in Jaeger with child fan-out spans,
slog JSON structured logging with trace context, 4 provisioned Grafana dashboards, active Prometheus alert rules,
and proven load testing (k6 constant-arrival-rate, zero budget overspend proof, zero double-counting duplicate replay proof, and pprof profiling under load).
Mock DSP bidders run on `:9101`–`:9108`, and campaign-service provides REST CRUD on `:8081` and gRPC catalog
snapshots on `:8082`.

---

## ⚡ Quickstart: Run & Verify in < 15 Minutes

### 1. Prerequisites
- **Go 1.27.3** or higher
- **Docker Engine / Docker Desktop** with Linux containers and Compose v2
- **Bash**, **curl**, and **GNU Make** (or `mingw32-make` on Windows)

### 2. Start Local Infrastructure & Seed Data
```bash
# 1. Initialize environment file and start backing services (Postgres, Redis, Kafka, Prometheus, Grafana, Jaeger)
test -f .env || cp .env.example .env
make up
make topics
make migrate-up

# 2. Populate 8 mock bidders and 40 active campaigns
make seed
```

### 3. Start the Core Services
In separate terminal windows (or using the Compose `apps` profile):
```bash
# Terminal 1: Mock DSP Bidders (ports 9101-9108)
make bidders

# Terminal 2: Campaign Service (REST :8081, gRPC :8082, Admin :8091)
make run-campaign

# Terminal 3: Event Aggregator (Reporting Consumer & Rollups :8093)
go run ./cmd/event-aggregator

# Terminal 4: Bid Gateway (Real-Time Exchange Engine :8080, Admin :8090)
go run ./cmd/bid-gateway
```

### 4. Verify End-to-End Functionality (Newman Test Suite)
Run the automated Postman test suite covering all 11 folders:
```bash
bash scripts/test-api.sh
# Generates comprehensive HTML & JUnit reports under docs/test-reports/<date>/phase-12/
```

### 5. Run Performance & Non-Functional Verification
```bash
# A. Load Testing with k6 (1,000 rps constant-arrival-rate)
./loadtest/k6/k6 run -e RATE=1000 -e DURATION=30s loadtest/k6/bid.js

# B. Slow-Bidder Isolation Drill (proves stragglers do not cascade)
./loadtest/k6/k6 run loadtest/k6/slow-bidder.js

# C. Multi-Replica Zero Budget Overspend Proof
bash scripts/test-multi-replica-overspend.sh

# D. Idempotent Deduplication & Crash Resilience Proof
bash scripts/test-duplicate-replay.sh
```

### 6. Observe Dashboards & Traces
- **Grafana Dashboards:** [http://localhost:3000](http://localhost:3000) (Login: `admin` / `admin`)
  - *PulseBid: Exchange Overview*
  - *PulseBid: Bidders & Demand*
  - *PulseBid: Budget & Events*
  - *PulseBid: Go Runtime & Process Health*
- **Jaeger Distributed Tracing:** [http://localhost:16686](http://localhost:16686)
- **Prometheus Metrics & Alerts:** [http://localhost:9090](http://localhost:9090)

---

## Prerequisites

- Go 1.27.3 (the version recorded in `go.mod`).
- Docker Engine or Docker Desktop with Linux containers and Compose v2 supporting
  `up --wait` (v2.20+ recommended). Allocate at least 4 GB of memory.
- GNU Make, Bash, curl, and a C compiler matching Go's architecture for `-race`.
- Network access for Go tooling and container images on the first run.

On Windows, use WSL2 or Git Bash with GNU Make and a **64-bit MinGW-w64** compiler.
The older 32-bit MinGW compiler cannot run race tests with `windows/amd64` Go.
If GNU Make is installed as `mingw32-make`, substitute that command for `make`.
Docker Desktop must be running in Linux-container mode.

## Local infrastructure

Run from the repository root:

```bash
test -f .env || cp .env.example .env
make up
make topics
make migrate-up
make lint test build
```

`make up` waits for every default service to become healthy. `make topics` creates
`pulsebid.events.v1` (6 partitions) and `pulsebid.events.dlq.v1` (1 partition),
both with seven-day retention and one local replica. It can be rerun safely.
Make explicitly selects the root `.env` (or `.env.example` if absent). It is a
template for native service environment variables; infrastructure
uses explicit development settings in Compose, with an optional `POSTGRES_PORT`
override. On this machine, the local ignored `.env` uses port **15432** because
an existing PostgreSQL server owns 5432; its `DATABASE_URL` matches. The committed
template keeps the default port 5432. Preserve an existing `.env` when rerunning
the quickstart.

| Service | Local address | Credentials / notes |
| --- | --- | --- |
| PostgreSQL 16 | `localhost:5432` | user/password/database: `pulsebid` |
| Redis 7 | `localhost:6379` | append-only persistence |
| Kafka (Redpanda) | `localhost:9092` | containers use `redpanda:29092` |
| Prometheus | <http://localhost:9090> | app targets become UP when native services run |
| Grafana | <http://localhost:3000> | `admin` / `admin`; Prometheus preconfigured |
| Jaeger | <http://localhost:16686> | OTLP gRPC on `localhost:4317` |
| Traefik | <http://localhost:8088> | optional `scale` profile; routes added later |

All published infrastructure ports bind to loopback. Grafana dashboards are
added in Phase 11; Jaeger can already receive sampled admin HTTP traces.

```bash
bash scripts/wait-for.sh http://localhost:3000/api/health 30
bash scripts/wait-for.sh http://localhost:16686/ 30
docker compose --env-file .env -f deploy/docker-compose.yml ps
make down                 # preserves data
make reset CONFIRM=1       # explicitly deletes local named-volume data
```

Use `docker compose --env-file .env -f deploy/docker-compose.yml --profile scale up -d --wait`
to also start Traefik. The `apps` profile includes campaign-service and the
gateway/aggregator operational shells. Gateway routing is added in later phases.

## Development

```bash
make build                # go build ./...
make test                 # go test -race -cover ./...
make test-int             # uncached PostgreSQL testcontainer tests with race/coverage
make lint                 # pinned golangci-lint, formatting check + mandated linters
make fmt                  # apply gofmt through golangci-lint
go run ./cmd/bid-gateway   # admin server on :8090; Ctrl+C drains and stops it
make test-platform        # real admin/SIGTERM smoke checks on Linux/macOS
make docker-build SERVICE=bid-gateway
```

The linter is run through `go run` at a pinned version, so no global linter
installation is needed. `make help` lists the available workflow;
future targets fail with the phase that introduces their implementation.
Dependencies are pinned in `go.mod` and verified by `go.sum`. CI runs lint,
race tests, real PostgreSQL integration tests, build, admin/SIGTERM smoke checks,
infrastructure checks, and Newman reports. `coverage-integration.out` includes
the database-backed service tests; unit-only campaign coverage does not.

If Windows has only a 32-bit C compiler, run race tests in Linux instead. From
PowerShell with Docker available:

```powershell
docker run --rm --network pulsebid_default --mount "type=bind,source=$PWD,target=/src" --mount type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock --mount type=volume,source=pulsebid-go-build-cache,target=/root/.cache/go-build --mount type=volume,source=pulsebid-go-module-cache,target=/go/pkg/mod -e TESTCONTAINERS_HOST_OVERRIDE=host.docker.internal -e DATABASE_URL=postgres://pulsebid:pulsebid@postgres:5432/pulsebid?sslmode=disable --workdir /src golang:1.27.3-alpine sh -c 'apk add --no-cache bash make gcc musl-dev curl && make test-int test build test-platform'
```

Run `make up migrate-up` first so the shared network and smoke-test database
exist. On native Linux/macOS, use `make lint test test-int build test-platform`.
Operational routes are described in [the admin OpenAPI contract](api/openapi/admin.yaml).

## Run the platform servers

Each command reads its process environment, not `.env` automatically. With no
overrides, run each in a separate terminal:

| Command | Admin listener |
| --- | --- |
| `go run ./cmd/bid-gateway` | `:8090` |
| `make run-campaign` | `:8091` (REST `:8081`, internal gRPC `:8082`) |
| `go run ./cmd/event-aggregator` | `:8093` |
| `make bidders` | `:8094` (admin), `:9101`–`:9108` (bidders 1..8) |
| `make seed` | Idempotently populates 8 bidders and 40 campaigns |

Override `ADMIN_ADDR` and `SERVICE_NAME` per process if needed. Keep admin
listeners on trusted networks; use `ADMIN_ADDR=127.0.0.1:8090` to restrict a
native process to loopback (Docker-based Prometheus then needs a reachable
admin address instead).

```bash
curl -i http://localhost:8090/healthz
curl -i http://localhost:8090/readyz
curl http://localhost:8090/metrics
```

Campaign readiness also pings PostgreSQL; other shells still check startup and
draining only. SIGINT/SIGTERM
turns readiness into 503, leaves liveness at 200 for `SHUTDOWN_DELAY` (5s), then
drains HTTP and flushes tracing within `SHUTDOWN_TIMEOUT` (15s).
For guaranteed trace visibility in local Jaeger, set
`OTEL_TRACES_SAMPLER_ARG=1` before starting a service; normal sampling is 0.01.

See [architecture](docs/ARCHITECTURE.md), [decisions](docs/DECISIONS.md), and
[the local runbook](docs/RUNBOOK.md). Next: **Phase 9 — Reporting API**.

## Event Aggregator & Rollups

The event aggregator consumes real-time ad events (`impression`, `click`, `auction`) from Kafka (`pulsebid.events.v1`),
performs single-transaction deduplication in PostgreSQL (`processed_events`), and aggregates additive hourly metrics
into `hourly_stats`:

```bash
# Aggregator Admin probes
curl -i http://localhost:8093/healthz
curl -i http://localhost:8093/readyz
curl -s http://localhost:8093/metrics | grep aggregator_
```

## Campaign API and Postman

For native development, run `make up migrate-up`, then `make run-campaign`.
In another terminal:

```bash
curl -H 'X-API-Key: dev-key' http://localhost:8081/v1/advertisers
curl -H 'X-API-Key: dev-key' -H 'Content-Type: application/json' \
  -d '{"name":"Acme Shoes","contact_email":"ads@acme.test"}' \
  http://localhost:8081/v1/advertisers
```

`make run-campaign` and `make migrate-up/down` read only `DATABASE_URL` from the
root environment file if it is not already exported. They do not apply the
gateway-specific `SERVICE_NAME`, `HTTP_ADDR`, or `ADMIN_ADDR` in that template.
Direct `go run ./cmd/campaign-service` uses process environment and the default
database port 5432, so export the correct URL when using port 15432.

For the complete API test setup:

```bash
make test-api
make test-report
```

This starts infrastructure, applies embedded migrations, builds app containers,
and runs **00 Health**, **01 Setup (Campaign service)**, and **02 Validation and
Auth negatives**. No host Node/Newman installation is needed. Import
[the collection](postman/PulseBid.postman_collection.json) and
[local environment](postman/local.postman_environment.json) to run the same
requests in Postman. Each run creates uniquely named fixtures; primary campaigns
remain available for inspection. HTML/JUnit/JSON reports are written to
`docs/test-reports/<date>/phase-3/`. Apps remain running; `make down` stops them
and preserves PostgreSQL data.

See [the campaign OpenAPI contract](api/openapi/campaign-service.yaml) for all
routes and schemas. Campaigns start as drafts, activation requires an active
creative, and archived status is terminal. Lists use descending `(created_at,id)`
keyset cursors. PATCH preserves omitted fields; null clears only nullable fields.
The campaign body limit is 512 KiB and creative markup is limited to 64 KiB.
Budget routes remain deferred to Phase 6.

`make sqlc` regenerates query code using pinned sqlc 1.31.1.
`make migrate-up` applies pending migrations. `make migrate-down` reverses one
version and **deletes the tables/data defined by that version**. Verification
tests reversal only against disposable PostgreSQL testcontainers.

## Catalog snapshots

The campaign process hosts `pulsebid.catalog.v1.CatalogService/GetSnapshot` on
`GRPC_ADDR` (default `:8082`), alongside REST and admin. The contract is in
[catalog.proto](api/proto/pulsebid/catalog/v1/catalog.proto); generated Go files
are in [the catalog package](gen/pulsebid/catalog/v1). `make proto` builds pinned
local Buf/Go plugins, formats/lints the contract, and regenerates code. No host
protoc or Buf installation is required.

```bash
make test-api              # start/migrate apps and test REST snapshots
make test-grpc             # pinned grpcurl snapshot + standard health check
# Or, with grpcurl installed:
grpcurl -plaintext -max-time 5 localhost:8082 pulsebid.catalog.v1.CatalogService/GetSnapshot
curl -i -H 'X-API-Key: dev-key' http://localhost:8081/v1/snapshot
```

gRPC is a trusted internal listener with plaintext local transport and reflection
for grpcurl; it does not use REST API-key authentication. Compose publishes it
only on loopback. Standard gRPC health reflects DB readiness (refreshed every
second) and becomes NOT_SERVING when draining. All listeners close before the
database pool during shutdown.

Each call checks the catalog version. Full data is rebuilt in one SQL statement
only when the cached version differs. Only active campaigns with active
advertisers, enabled bidders, and active creatives are included. The bidder list
contains only referenced bidders; creatives are ordered by ID. Start/end windows
remain in the snapshot for the gateway to evaluate at request time. Missing
targeting is unrestricted UTC targeting.

Use `known_version` in gRPC for a version-only `unchanged=true` response. REST
uses protobuf JSON with snake_case names, **int64 strings** (version and money
micros), symbolic pacing enum names, and UTC timestamps. Unset start/end times
are omitted. The quoted decimal `ETag` can be sent back in `If-None-Match` for a
bodyless 304. Weak validators, tag lists, and `*` are supported. Authentication
and the DB version check still run for conditional reads.

Folder **01 Setup (Campaign service)** tests active/paused/reactivated membership
and ETags; folder **02** checks snapshot authentication. DB errors return HTTP
503 / gRPC Unavailable rather than asserting cached data is current. Gateway
stale-cache behavior is separate Phase 5 work.

Stop the Compose campaign container before starting a native campaign process,
or override all three listener addresses. In PowerShell,
`$env:GRPC_ADDR='127.0.0.1:8182'` selects an alternate gRPC listener.

## Mock bidders and seed data

Mock bidders simulate downstream DSPs responding to bid requests on `:9101` through `:9108`.

```bash
make bidders                # run 8 mock bidders on 9101-9108
make seed                   # idempotently seed 8 bidders and 40 active campaigns
```

Each bidder supports runtime simulation tuning without restarts:
```bash
# Check current configuration
curl http://localhost:9101/admin/config

# Add 50ms latency and 10% errors
curl -X POST http://localhost:9101/admin/config \
  -H 'Content-Type: application/json' \
  -d '{"latency_ms": 50, "error_rate": 0.1}'

# Probe a bid directly
curl -X POST http://localhost:9101/bid \
  -H 'Content-Type: application/json' \
  -d '{"id":"req-1","imp":{"id":"imp-1","banner":{"w":300,"h":250},"bidfloor":0.50},"candidates":[{"campaign_id":"c1","creative_id":"cr1","max_bid_cpm":3.00}]}'
```

See [the mock bidder OpenAPI specification](api/openapi/mock-bidder.yaml) and [the runbook](docs/RUNBOOK.md) for full endpoint and parameter documentation.
