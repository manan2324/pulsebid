# Local infrastructure runbook

Start with the [quickstart](../README.md). Phase 2 exposes campaign REST on 8081
with `X-API-Key`, plus private admin listeners.

## Startup or health failures

```bash
docker compose --env-file .env -f deploy/docker-compose.yml config --quiet
docker compose --env-file .env -f deploy/docker-compose.yml ps -a
docker compose --env-file .env -f deploy/docker-compose.yml logs --tail=100
```

If Docker is unavailable, install/start Docker Desktop with Linux containers or
Docker Engine and Compose v2. If a published port is already taken, stop the
conflicting local process, or for PostgreSQL use `POSTGRES_PORT=15432 make up`
and set native services' `DATABASE_URL` to use port 15432. In PowerShell use
`$env:POSTGRES_PORT='15432'` before `mingw32-make up`. If Redpanda is unhealthy, check its logs and Docker's
memory allocation. `make up` fails if the stack does not become healthy within
180 seconds.

## Kafka topics

```bash
make topics
docker compose -f deploy/docker-compose.yml exec -T redpanda rpk topic list -X brokers=redpanda:29092
docker compose -f deploy/docker-compose.yml exec -T redpanda rpk topic describe pulsebid.events.v1 -X brokers=redpanda:29092
```

Expect six event partitions and one DLQ partition. Topic creation preserves
existing topics and displays their metadata; it does not silently repartition
or delete data. The event topic's retention is 604800000 milliseconds.

## Observability

Grafana: <http://localhost:3000> (`admin` / `admin` on first startup).
Prometheus: <http://localhost:9090>. Jaeger: <http://localhost:16686>.
Run the three main commands in separate terminals to enable Prometheus's app
targets. `/metrics` includes Go/process collectors and HTTP route counters after
requests have completed. Set `OTEL_TRACES_SAMPLER_ARG=1` to see every admin request
in Jaeger. Grafana dashboards are added in Phase 11.

## Platform health and shutdown

`/healthz` stays 200 while the process is running. `/readyz` is 200 after listener
startup and 503 during shutdown or when a dependency check fails. Campaign
readiness pings PostgreSQL under a one-second deadline. The remaining shells do
not yet check snapshot or Redis availability. Errors use the JSON envelope and every response
carries `X-Request-Id`.

Send SIGTERM to the compiled binary (or Ctrl+C to the foreground native command).
Expect a `service draining` log, a 5s drain delay, then `service stopped` and exit
0. Do not signal the `go run` wrapper when validating Unix signals; use
`make test-platform`, which builds and signals all five binaries directly.
The HTTP and gRPC shutdown helpers force-close at the shutdown deadline.

If startup fails, check the JSON `service failed` log for invalid environment or
occupied admin ports. Windows race tests require a 64-bit C compiler; use the
Docker command in README if only 32-bit MinGW is installed.

## Campaign API and database

Run `make migrate-up` before exercising CRUD. A healthy DB ping alone does not
prove the schema is migrated. `make test-api` applies migrations automatically
and leaves app containers running for manual Postman use. Check
`http://localhost:8091/readyz`, then `GET http://localhost:8081/v1/advertisers`
with `X-API-Key: dev-key`.

Missing/wrong keys produce 401. Invalid fields, UUIDs, money precision, timezones,
or pagination produce 400. Unique-name and foreign-key conflicts produce 409,
as does activation without an active creative. Advertiser/campaign DELETE
archives; creative DELETE is hard deletion; bidder DELETE fails while any
campaign (including archived) references it.

PostgreSQL outages produce API 503 and readiness 503 while liveness stays 200.
The pool reconnects when the database returns. Inspect campaign/container logs
and database health; do not use `make reset` to repair a transient outage.
Queries/transactions have five-second contexts and server statement/lock
timeouts. Migration statements have a 30s cap and 5s lock timeout.

`make test-int` creates an isolated PostgreSQL container, exercises CRUD,
concurrent patches, pagination, up/down/up migrations, and a database outage,
then removes the container. It never runs down on the persistent local DB.
`make migrate-down` is an explicit destructive tool for reversing one local
migration, not an outage-recovery step.

## Catalog snapshots

Campaign-service listens for gRPC on 8082 (`GRPC_ADDR`) and REST on 8081.
`make test-grpc` runs the pinned grpcurl client and verifies standard gRPC health.
Reflection is enabled on the trusted internal gRPC listener; REST snapshot reads
still require `X-API-Key`. In Compose, published listeners bind to loopback.

`GET /v1/snapshot` returns protobuf JSON using snake_case fields, int64 strings,
symbolic pacing enums, and UTC timestamps. Retain its quoted version ETag and
send `If-None-Match` to get a bodyless 304 for the current version. A pause or
other successful catalog mutation advances the version; the next read rebuilds
the snapshot. Inactive parents or missing active creatives exclude a campaign.
Future/expired start/end dates alone do not exclude it; the gateway checks them
at bid time. Missing targeting means unrestricted rules in UTC.

Each request checks PostgreSQL even when a snapshot is cached. On DB failure,
REST returns 503 and gRPC returns Unavailable; no stale 304 or unchanged result
is returned. gRPC health follows database readiness within one second and flips
to NOT_SERVING immediately when draining. Consumers keep their own last snapshot
according to the Phase 5 gateway policy.

Regenerate with `make proto` (Buf 1.75.0, Go plugin 1.36.12, gRPC plugin 1.6.2) and
`make sqlc` after editing their inputs. Generated files are part of the source
tree. `make test-api` now writes Phase 4 reports; `make test-grpc` writes snapshot
and health JSON beside them. No proto or schema field numbers changed in Phase 3.

## Mock Bidders & Runtime Tuning (Phase 4)

The mock bidder service simulates external DSPs during local development and testing.
Run 8 bidders natively:
```bash
make bidders N=8 BASE_PORT=9101
```
Or start all platform services in Docker:
```bash
docker compose --env-file .env -f deploy/docker-compose.yml --profile apps up -d --wait
```

Mock bidders listen on consecutive ports starting from `BASE_PORT` (default 9101 through 9108):
- `POST /bid`: accepts `BidRequest`, filters candidates below `floor`, and returns bids according to the active pricing mode. Returns HTTP 204 if no candidates qualify.
- `GET /healthz`: liveness probe.
- `GET /readyz`: readiness probe (returns 503 during graceful shutdown).
- `GET /metrics`: Prometheus metrics (`bids_received_total`, `bids_submitted_total`, `bids_no_bid_total`, `bid_price_dollars`, `bid_duration_seconds`).
- `GET /admin/config`: inspect active simulation knobs.
- `POST /admin/config`: atomic runtime tuning without restarting the process.

### Runtime tuning examples

Simulate high latency on bidder 1:
```bash
curl -X POST http://localhost:9101/admin/config \
  -H 'Content-Type: application/json' \
  -d '{"latency_ms": 120, "jitter_ms": 20}'
```

Inject errors or timeouts:
```bash
# 20% internal server error rate (HTTP 500)
curl -X POST http://localhost:9102/admin/config \
  -H 'Content-Type: application/json' \
  -d '{"error_rate": 0.2}'

# 10% timeout rate (hangs for 2 seconds to trigger gateway timeout)
curl -X POST http://localhost:9103/admin/config \
  -H 'Content-Type: application/json' \
  -d '{"timeout_rate": 0.1}'
```

Change pricing mode:
```bash
# Mode: "fixed" (max_bid_cpm), "shade" (max_bid_cpm * bid_shade), or "random" (seeded by seed + fnv64(req_id+campaign_id))
curl -X POST http://localhost:9104/admin/config \
  -H 'Content-Type: application/json' \
  -d '{"bid_mode": "shade", "bid_shade": 0.85}'
```

Reset knobs to defaults:
```bash
curl -X POST http://localhost:9101/admin/config \
  -H 'Content-Type: application/json' \
  -d '{"latency_ms": 0, "jitter_ms": 0, "error_rate": 0, "timeout_rate": 0, "no_bid_rate": 0, "bid_mode": "fixed"}'
```

### Seeding test data

Populate the database with 8 registered bidders, a test advertiser ("Seed Advertiser"), and 40 active campaigns across targeting tiers (countries, devices, placements):
```bash
make seed
```
`make seed` is idempotent: running it repeatedly updates existing bidders and leaves active campaigns intact without creating duplicates.

## Bid Gateway Operations & Debugging (Phase 5)

The gateway listens for bid requests on `:8080` (`HTTP_ADDR`) and operational probes on `:8090` (`ADMIN_ADDR`).

### Running the Gateway
Natively:
```bash
make run-gateway
```
Or via Docker Compose:
```bash
docker compose --env-file .env -f deploy/docker-compose.yml --profile apps up -d bid-gateway
```

### Probing the Gateway
- **Liveness**: `curl http://localhost:8090/healthz` (200 OK once process starts)
- **Readiness**: `curl http://localhost:8090/readyz` (200 OK only after a catalog snapshot is cached; returns 503 during startup or graceful drain)
- **Metrics**: `curl http://localhost:8090/metrics`
  - `bid_requests_total{outcome="..."}`
  - `bidder_requests_total{bidder_id="...", outcome="..."}`
  - `snapshot_version`, `snapshot_age_seconds`, `snapshot_campaigns`

### Testing Bids via cURL
Send a sample bid request:
```bash
curl -X POST http://localhost:8080/v1/bid \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "req-manual-1",
    "imp": { "id": "1", "placement_id": "slot-0", "bidfloor": 0.50 },
    "device": { "type": "mobile" },
    "geo": { "country": "IN" }
  }'
```

### Test Mode & Diagnostics
When running with `ENABLE_TEST_ENDPOINTS=true`:
- Inspect debug header: `X-PulseBid-Debug` reports `eligible`, `responded`, `timed_out`, `errors`, `valid_bids`, `attempts`.
- Override evaluation time: Send `X-PulseBid-Test-Time: 2026-11-18T10:00:00Z` to test specific time-of-day targeting hours.
- Inspect latency: Header `Server-Timing: total;dur=<ms>` is returned on all bid responses.

## Budget Pacing & Overspend Verification (Phase 6)

Budget is managed atomically across gateway replicas using Redis and embedded Lua scripts (`internal/budget`).

### Inspecting Campaign Spend
Query current spend status for any campaign:
```bash
curl -X GET http://localhost:8081/v1/campaigns/<campaign_id>/budget \
  -H 'X-API-Key: dev-key'
```
Response:
```json
{
  "campaign_id": "01a1f31d-cd1d-7e1c-aa9b-6014f4faf055",
  "date": "2026-11-19",
  "daily_budget": 0.01,
  "daily_budget_micros": 10000,
  "spent": 0.00909,
  "spent_micros": 9090,
  "remaining": 0.00091,
  "remaining_micros": 910
}
```

### Resetting Campaign Spend (Test Mode Only)
When `ENABLE_TEST_ENDPOINTS=true`, reset a campaign's daily spend in Redis:
```bash
curl -X POST http://localhost:8081/v1/campaigns/<campaign_id>/budget/reset \
  -H 'X-API-Key: dev-key'
```

### Verifying Zero Overspend
Run the automated verification script against all campaigns:
```bash
bash scripts/verify-no-overspend.sh
```
The script queries `/v1/campaigns/{id}/budget` across all active campaigns and exits with code 1 if any campaign has `spent > daily_budget`.

## Event Tracking & Verification (Phase 7)

Tracked ad events (`impression`, `click`, `auction`) are published to Kafka topic `pulsebid.events.v1`.

### Firing Tracking Pixels & Clicks
1. **Fire Impression Tracking Pixel**:
   ```bash
   curl -i "http://localhost:8080/v1/events/impression?t=<token>"
   ```
   Expect HTTP `200 OK`, `Content-Type: image/gif` (43-byte transparent 1x1 GIF), and `Cache-Control: no-store`.

2. **Follow Click Tracking URL**:
   ```bash
   curl -i --max-redirs 0 "http://localhost:8080/v1/events/click?t=<token>"
   ```
   Expect HTTP `302 Found`, `Location: <destination_landing_url>`, and `Cache-Control: no-store`.

3. **Tampered Token Check**:
   ```bash
   curl -i "http://localhost:8080/v1/events/impression?t=tampered-token"
   ```
   Expect HTTP `400 Bad Request` with `code: "invalid_token"`.

4. **Token Expiry Check (Test Mode)**:
   ```bash
   curl -i -H "X-PulseBid-Test-Time: 2028-11-18T10:00:00Z" "http://localhost:8080/v1/events/impression?t=<token>"
   ```
   Expect HTTP `410 Gone` with `code: "token_expired"`.

### Inspecting Live Kafka Events
Consume raw events from Redpanda:
```bash
docker compose -f deploy/docker-compose.yml exec -T redpanda rpk topic consume pulsebid.events.v1 -n 5
```
Each message carries:
- Partition key = `campaign_id` (ensures FIFO ordering per campaign).
- Value = JSON event payload with deterministic UUIDv5 `event_id` (`<type>:<bid_id>`).
- Headers = `traceparent`, `event_type`, `schema_version`.

## Event Aggregator & Deduplication Operations (Phase 8)

The event aggregator runs under `cmd/event-aggregator`, consuming from `pulsebid.events.v1` and updating
`hourly_stats` and `processed_events` in PostgreSQL.

### Health and Readiness Checks
```bash
# Liveness
curl -i http://localhost:8093/healthz

# Readiness (verifies both PostgreSQL and Kafka broker connectivity)
curl -i http://localhost:8093/readyz

# Prometheus Metrics
curl -s http://localhost:8093/metrics | grep aggregator_
```

### Inspecting Aggregation State in PostgreSQL
1. **Check Deduplicated Processed Events Count**:
   ```bash
   docker compose -f deploy/docker-compose.yml exec -T postgres psql -U pulsebid -d pulsebid -c "SELECT count(*) FROM processed_events;"
   ```

2. **Check Hourly Rollup Stats**:
   ```bash
   docker compose -f deploy/docker-compose.yml exec -T postgres psql -U pulsebid -d pulsebid -c "SELECT campaign_id, hour_start, impressions, clicks, spend_micros, bids, wins FROM hourly_stats ORDER BY hour_start DESC LIMIT 20;"
   ```

3. **Inspect Dead Letter Queue (DLQ)**:
   ```bash
   docker compose -f deploy/docker-compose.yml exec -T redpanda rpk topic consume pulsebid.events.dlq.v1 -n 5
   ```

### Querying Reporting API
1. **Query Campaign Performance (Total Granularity)**:
   ```bash
   curl -s -H "X-API-Key: dev-key" \
     "http://localhost:8083/v1/reports/campaigns?from=2026-11-19T00:00:00Z&to=2026-11-20T00:00:00Z&granularity=total" | jq .
   ```

2. **Query Single Campaign Performance (Hourly Granularity)**:
   ```bash
   curl -s -H "X-API-Key: dev-key" \
     "http://localhost:8083/v1/reports/campaigns/<campaign-id>?from=2026-11-19T00:00:00Z&to=2026-11-20T00:00:00Z&granularity=hour" | jq .
   ```

3. **Query Multiple Campaigns (Daily Granularity)**:
   ```bash
   curl -s -H "X-API-Key: dev-key" \
     "http://localhost:8083/v1/reports/campaigns?from=2026-11-01T00:00:00Z&to=2026-11-20T00:00:00Z&granularity=day&campaign_ids=<id1>,<id2>" | jq .
   ```

## Resilience & Failure Drills (Phase 10)

This section documents operational symptoms, dashboards, and remediation for every failure scenario in Section 14.1:

| Failure Scenario | Symptoms & Metrics | Dashboard / Verification | Remediation |
|---|---|---|---|
| **Bidder Slow (> deadline)** | p99 bid latency stays <= 120ms; `bidder_requests_total{outcome="timeout"}` increments; `X-PulseBid-Debug` shows `timed_out=1`. | Bidders dashboard: per-bidder timeout rate spike. | Check bidder latency; increase timeout in campaign service if intentional, or contact DSP partner. |
| **Bidder Returns 5xx / Bad JSON** | `bidder_requests_total{outcome="error"}` increments; other bidders fill requests normally (status 200). | Bidders dashboard: error rate graph for affected bidder. | Inspect bidder logs; DSP partner must fix endpoint error. |
| **Bidder Down** | Connection refused; circuit breaker trips to `Open`; `bidder_requests_total{outcome="circuit_open"}` increments; gateway calls skip bidder instantly (< 1ms). | Bidders dashboard: circuit state shows `open`. | Restart bidder service or disable bidder via `PATCH /v1/bidders/{id}`. |
| **All Bidders Down** | Gateway returns HTTP 204 with `X-PulseBid-Reason: no_bids`; fill rate drops to 0%. | Overview dashboard: fill rate drops to 0%, `bid_requests_total{outcome="no_bids"}` rises. | Investigate downstream bidder network or mock-bidder container. |
| **Redis Down / Slow** | Gateway returns HTTP 204 with `X-PulseBid-Reason: error` (fail closed); `/readyz` returns 503; `budget_reserve_total{result="error"}` increments. | Overview dashboard: 204 error outcome; Prometheus alert `RedisDown`. | Restart Redis (`docker compose up -d redis`); inspect Redis memory and CPU. |
| **Kafka Down** | Bidding unaffected (status 200); `events_published_total{result="dropped"}` increments; `GET /v1/events/impression` and `click` return HTTP 503. | Overview dashboard: event publish errors; Aggregator dashboard: lag freeze. | Restart Redpanda broker (`docker compose up -d redpanda`); check disk space. |
| **Campaign Service Down** | Gateway serves bids normally from cached snapshot; `snapshot_age_seconds` steadily increases; `/readyz` remains 200. | Overview dashboard: `snapshot_age_seconds` gauge climbing. | Restart campaign service (`docker compose up -d campaign-service`); check DB connectivity. |
| **PostgreSQL Down** | Campaign REST API returns HTTP 503; Reporting API returns HTTP 503; Aggregator retries batches without committing Kafka offsets (zero data loss). | Overview dashboard: DB connection pool exhausted. | Restart PostgreSQL (`docker compose up -d postgres`); inspect disk volume. |

### Failure Drill Verification Commands

1. **Simulate Slow Bidder**:
   ```bash
   curl -X POST http://localhost:9101/admin/config -H "Content-Type: application/json" -d '{"latency_ms": 500}'
   # Verify bid still returns 200 from healthy bidders under 120ms:
   curl -i -X POST http://localhost:8080/v1/bid -H "Content-Type: application/json" -d '{"id":"test-slow","imp":{"id":"i1","placement_id":"pm-auction","w":300,"h":250,"bidfloor":0.5},"device":{"type":"mobile"},"geo":{"country":"IN"}}'
   # Reset bidder:
   curl -X POST http://localhost:9101/admin/config -H "Content-Type: application/json" -d '{"latency_ms": 0}'
   ```

2. **Simulate Redis Outage (Documented Manual Drill)**:
   ```bash
   docker compose -f deploy/docker-compose.yml stop redis
   # Verify /readyz fails:
   curl -i http://localhost:8090/readyz
   # Verify bid fails closed with 204 error:
   curl -i -X POST http://localhost:8080/v1/bid -H "Content-Type: application/json" -d '{"id":"test-redis","imp":{"id":"i1","placement_id":"pm-auction","w":300,"h":250,"bidfloor":0.5},"device":{"type":"mobile"},"geo":{"country":"IN"}}'
   # Restore Redis:
   docker compose -f deploy/docker-compose.yml start redis
   ```

3. **Simulate Campaign Service Outage**:
   ```bash
   docker compose -f deploy/docker-compose.yml stop campaign-service
   # Verify gateway continues serving bids from memory:
   curl -i -X POST http://localhost:8080/v1/bid -H "Content-Type: application/json" -d '{"id":"test-snap","imp":{"id":"i1","placement_id":"pm-auction","w":300,"h":250,"bidfloor":0.5},"device":{"type":"mobile"},"geo":{"country":"IN"}}'
   # Restore campaign service:
   docker compose -f deploy/docker-compose.yml start campaign-service
   ```

## Observability & Dashboards (Phase 11)

PulseBid provides four provisioned Grafana dashboards (<http://localhost:3000>) and automated Prometheus alert rules (<http://localhost:9090>).

### Dashboard Reading Guide

1. **PulseBid - Exchange Overview** (`/d/pulsebid-exchange-overview`):
   - **Bid RPS**: Total inbound bid requests per second on gateway. Normal: tracking traffic pattern.
   - **Fill Rate**: Ratio of `filled` responses to total bids. Normal: > 40%. Alert if < 20% for 10m.
   - **Bid Latency (p50 / p95 / p99)**: Measured against the 120ms red threshold line. Alert if p99 > 120ms for 5m.
   - **Bid Outcomes Stacked**: Visual breakdown of `filled`, `no_eligible`, `no_bids`, `below_floor`, `budget`, `error`, `invalid`.
   - **Clearing CPM**: p50 and p95 clearing price in USD.
   - **Active Campaigns**: Total count of active campaigns loaded in the memory snapshot.

2. **PulseBid - Bidders** (`/d/pulsebid-bidders`):
   - **Per-Bidder p99 Latency**: Outbound latency to each DSP. Spikes above 100ms indicate impending bidder drops.
   - **Per-Bidder Timeout Rate**: Percentage of requests where candidate bidder exceeded its deadline.
   - **Per-Bidder Error Rate**: Percentage of 5xx responses or transport rejections.
   - **Circuit Breaker Open Rate**: Non-zero rate indicates tripped breaker skipping bidder calls instantly.
   - **Bidder Inflight**: Active concurrent calls against the 512 bulkhead limit.
   - **Invalid Bids Dropped**: Invalid bid responses partitioned by validation reason (`not_in_candidate_list`, `price_non_positive`, `price_invalid_or_exceeds_max`).

3. **PulseBid - Budget and Events** (`/d/pulsebid-budget-and-events`):
   - **Budget Reserve Outcomes**: Breakdown of `reserved`, `exhausted`, `paced`, `error`. Spikes in `exhausted` indicate campaigns hitting daily caps; spikes in `paced` indicate smoothing burst suppression.
   - **Budget Reserve Latency**: p99 Redis round-trip time. Normal: < 3ms.
   - **Event Publish Rate**: Outbound impression, click, and auction events sent to Kafka.
   - **Event Buffer Depth**: Depth of the async auction event channel (10,000 capacity). Must remain near 0.
   - **Aggregator Consumer Lag**: Unprocessed records in `pulsebid.events.v1` per partition.
   - **Duplicate Event Ratio**: Ratio of deduplicated events to total events processed. Normal: 0–10% under retry/redelivery conditions.
   - **Aggregator Batch Size & Duration**: p95 batch size (up to 500) and transaction commit time (< 50ms).

4. **PulseBid - Go Runtime** (`/d/pulsebid-go-runtime`):
   - **Goroutines by Job**: Baseline ~20–50 goroutines per binary. Watch for unbounded goroutine growth indicating leaks.
   - **Heap Alloc Bytes**: Active memory usage.
   - **GC Duration**: Average pause duration.
   - **Process CPU Usage**: Core utilization across `bid-gateway`, `campaign-service`, `event-aggregator`, and `mock-bidders`.
   - **Connection Pools**: Active (`acquired`) vs idle connections for PostgreSQL (`db_pool_*`) and Redis (`redis_pool_*`).

---

### Alert Response Playbooks

1. **`BidLatencyHigh` (p99 latency > 120ms for 5m)**:
   - *Severity*: Critical
   - *Investigation*:
     1. Open **PulseBid - Bidders** dashboard to identify if a specific bidder's latency is elevated without timing out.
     2. Check **PulseBid - Budget and Events** dashboard for Redis reserve latency spikes (> 5ms).
     3. Check **PulseBid - Go Runtime** dashboard for GC pause spikes or CPU saturation (> 90%).
   - *Remediation*:
     - If one bidder is slow: lower its `timeout_ms` in `campaign-service` or trip breaker by setting `error_rate: 1.0` temporarily.
     - If Redis is overloaded: scale Redis memory or inspect slow logs (`redis-cli slowlog get 10`).

2. **`FillRateLow` (fill rate < 20% for 10m)**:
   - *Severity*: Warning
   - *Investigation*:
     1. Open **PulseBid - Exchange Overview** dashboard: inspect the **Bid Outcomes Stacked** chart.
     2. If `no_eligible` dominates: catalog targeting criteria (geo, placement, time-of-day) do not match incoming traffic.
     3. If `below_floor` dominates: bid floor in requests is set higher than campaign `max_bid_cpm`.
     4. If `budget` dominates: campaigns have exhausted their daily budgets or pacing mode is restrictive.
     5. If `no_bids` dominates: bidders are returning 204 or connection errors.
   - *Remediation*: Adjust campaign targeting, lower floor prices, or increase daily budgets.

3. **`SnapshotStale` (snapshot age > 30s)**:
   - *Severity*: Warning
   - *Investigation*:
     1. Check if `campaign-service` is running and reachable over gRPC port 8082:
        `docker compose -f deploy/docker-compose.yml exec -T bid-gateway /healthcheck`
     2. Inspect gateway logs for `failed to poll catalog snapshot; serving stale`.
   - *Remediation*: Restart `campaign-service` (`docker compose restart campaign-service`). Gateway continues serving uninterrupted from the cached snapshot.

4. **`ConsumerLagHigh` (consumer lag > 100 for 5m)**:
   - *Severity*: Warning
   - *Investigation*:
     1. Open **PulseBid - Budget and Events** dashboard: verify whether lag is growing on all partitions or a single partition.
     2. Check PostgreSQL write latency and lock contention in `processed_events` or `hourly_stats`.
   - *Remediation*: Scale up aggregator instances or increase `BATCH_SIZE` / reduce flush interval.

---

### Jaeger Distributed Tracing Inspection

Open Jaeger UI (<http://localhost:16686>) and select service `bid-gateway`:
- Each bid trace shows the full hierarchy: `POST /v1/bid` -> `bid.validate` -> `bid.targeting` -> `bid.fanout` -> `bidder.call` (with child `POST /bid` on mock bidder) -> `bid.auction` -> `bid.budget_reserve` -> `bid.respond`.
- Trace propagation across HTTP uses W3C header `traceparent`.
- Kafka event messages carry `traceparent` in record headers, allowing the event aggregator to link event processing spans.

## Stopping and resetting

`make down` stops infrastructure and preserves its named volumes.
`make reset CONFIRM=1` deletes local data. Docker images remain cached.
