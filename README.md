# PulseBid

A real-time bidding exchange in Go: campaign administration, cached gRPC
catalogs, concurrent DSP fan-out, second-price auctions, atomic Redis budgets,
signed impression/click tracking, Kafka ingestion, deduplicated PostgreSQL
rollups, and reporting. Prometheus, Grafana, and Jaeger provide observability.

The [build plan](./PulseBid%20Build%20Plan.md) defines the contracts.
See [current verification](./docs/VERIFICATION.md) for actual results and evidence;
historical completion claims are superseded by that report.

## Quickstart

Requirements: Docker Engine/Desktop with Linux containers and Compose **2.24.4+**,
Bash, curl, and Python 3 for the system proof scripts. Windows users can run the
commands in an Ubuntu WSL terminal. Go **1.27.3** is needed for native development;
the container quickstart does not need a host Go or Node installation.

From the repository root:

```bash
test -f .env || cp .env.example .env
docker compose --env-file .env -f deploy/docker-compose.yml up -d --wait
bash scripts/create-topics.sh
docker compose --env-file .env -f deploy/docker-compose.yml --profile tools run --build --rm migrate up
docker compose --env-file .env -f deploy/docker-compose.yml --profile apps up --build -d --wait
docker compose --env-file .env -f deploy/docker-compose.yml --profile tools run --build --rm seed
```

If another PostgreSQL server uses 5432, set `POSTGRES_PORT=15432` in `.env`.
For native database clients, update `DATABASE_URL` to the same port. Preserve an
existing `.env`; native commands do not automatically load all of its variables.

The seed command can be repeated: it registers eight bidders and maintains
40 active campaigns targeting `slot-0` through `slot-9`.

```bash
curl -i http://localhost:8090/readyz
curl -H 'X-API-Key: dev-key' http://localhost:8081/v1/campaigns
curl -H 'Content-Type: application/json' -d '{"id":"example","imp":{"id":"1","placement_id":"slot-0","w":300,"h":250,"bidfloor":0.5},"device":{"type":"mobile"},"geo":{"country":"IN"},"tmax":100}' http://localhost:8080/v1/bid
```

| Service | Address |
| --- | --- |
| Gateway / admin | `localhost:8080` / `localhost:8090` |
| Campaign REST / gRPC / admin | `localhost:8081` / `localhost:8082` / `localhost:8091` |
| Reporting / aggregator admin | `localhost:8083` / `localhost:8093` |
| Mock bidders / admin | `localhost:9101`â€“`9108` / `localhost:8094` |
| PostgreSQL / Redis / Kafka | `localhost:5432` / `localhost:6379` / `localhost:9092` |
| Grafana / Prometheus / Jaeger | `localhost:3000` / `localhost:9090` / `localhost:16686` |

Grafana uses local development credentials `admin` / `admin`. Campaign and
reporting APIs use `X-API-Key: dev-key`. Configuration defaults are in
[.env.example](./.env.example). Admin listeners expose health, readiness, metrics,
and pprof. Test endpoints are enabled in the local Compose apps profile.

## Run the checks

```bash
# Linux/macOS with Go, Make, a 64-bit C compiler, and running backing services:
make lint build
DATABASE_URL='postgres://pulsebid:pulsebid@localhost:5432/pulsebid?sslmode=disable' make test-int
REQUIRE_INTEGRATION=1 go test -race -count=5 ./internal/budget ./internal/gateway ./internal/aggregator
make fuzz FUZZTIME=10s
make bench

# Windows/WSL alternative: container supplies Go, Make, and a 64-bit compiler.
# Set DATABASE_URL to the host-published PostgreSQL port (15432 on this machine).
DATABASE_URL='postgres://pulsebid:pulsebid@localhost:15432/pulsebid?sslmode=disable' bash scripts/verify-linux.sh

# Full API regression; builds apps, migrates, seeds, and runs all Postman folders:
bash scripts/test-api.sh
```

`make test-int` runs **all** packages, enables campaign testcontainers, and sets
`REQUIRE_INTEGRATION=1`: an unavailable PostgreSQL, Redis, or Kafka service is a
failure, not a skipped safety test. Kafka tests create private topics and remove
them afterward. CI starts and migrates infrastructure before running this suite.
Without that flag, ordinary `go test ./...` may skip unavailable external services.
Repository and retention tests create migrated private databases on the configured
test server and drop them afterward; that test account needs `CREATE DATABASE`.
Application tables are not used for those tests.

The API runner also creates a private migrated database. Its exit cleanup restores
the normal application configuration and drops only that test database. Run API
and load checks separately from integration tests to avoid competing workloads.

The native Windows GCC installed on this machine is 32-bit and cannot build
Go's race detector; use the Linux verification script. Docker host networking
must be enabled for that script on Docker Desktop. Newman runs in a Node
container, so no host Newman installation is needed. Reports are under
`docs/test-reports/<date>/phase-12/` and `docs/test-reports/<date>/verification/`.

## System and performance proofs

```bash
# Starts three gateways behind Traefik and checks an isolated $0.05 budget.
bash scripts/test-multi-replica-overspend.sh
# Public gateway URLs now use localhost:8088; each admin port is assigned dynamically.
bash scripts/test-duplicate-replay.sh
bash scripts/verify-no-overspend.sh
```

The budget proof requires 2,000 completed k6 requests, positive fills capped at
49 at a clearing CPM of 1.01, spend of exactly 49,490 micros, and positive request
counts on all three replicas. It reports error/no-bid responses and unconfirmed
reservations separately: a Redis response timeout can leave a charged reservation
under the documented fail-closed/no-refund behavior. It fails if no money was
spent or any cap is exceeded. Replay uses a fresh campaign,
fires each impression/click three times, restarts ingestion with queued records,
and checks exactly one of each plus the exact spend. Go integration tests inject
crashes before DB commit and after DB commit/before offset commit.

To return to a single gateway on 8080 after scale tests:

```bash
docker compose --env-file .env -f deploy/docker-compose.yml --profile apps up -d --wait --scale bid-gateway=1 bid-gateway
```

For headline performance, run the gateway and mock bidders natively on Linux
or WSL, with the backing services running. Seed bidder endpoints for the gateway's
network namespace (`BIDDER_HOST=http://127.0.0.1 make seed` for native bidders).
Install k6 and Vegeta, or set `K6_BIN` and `VEGETA_BIN` to their executable paths.
Enable test mode on the gateway to collect block/mutex profiles.

```bash
make load RATE=1000 DURATION=30s
make load RATE=3000 DURATION=30s
make load RATE=5000 DURATION=2m
bash scripts/run-phase12-load-and-profile.sh
```

The full driver checks 1k/3k/5k rps, collects five pprof profiles during the
two-minute 5k run, runs slow-bidder isolation, and cross-checks with Vegeta.
Threshold failures propagate as command failures. k6 requires p99 below 120ms,
failure rate below 1%, zero dropped iterations, and actual fills. Do not equate
passing Postman assertions with proof of concurrent throughput.

## Native development and operations

```bash
make run-campaign             # reads DATABASE_URL from the selected env file
make bidders N=8
make run-gateway
make run-aggregator
make seed                    # requires the campaign service to be running
make proto sqlc              # regenerate protobuf and query code
make down                    # stop the stack, preserve data
make reset CONFIRM=1          # explicitly delete local infrastructure volumes
```

SIGTERM/SIGINT first marks services unready, waits the drain delay, stops
listeners, flushes owned work, and closes resources. Kafka outages leave bidding
available while critical impression/click publishing returns 503. Redis outages
fail closed. Gateway catalogs remain available during campaign-service outages
once an initial snapshot has loaded.

See [architecture](./docs/ARCHITECTURE.md), [decisions](./docs/DECISIONS.md),
[runbook](./docs/RUNBOOK.md), [load-test details](./loadtest/README.md),
[OpenAPI contracts](./api/openapi), and the
[Postman collection](./postman/PulseBid.postman_collection.json).
