# Architecture

The complete architecture and contracts are in
[Sections 4–14 of the build plan](../PulseBid%20Build%20Plan.md).

Planned flow: client → bid-gateway → eligible mock bidders → second-price
auction → Redis budget reservation → response. Campaign-service supplies an
in-memory catalog over gRPC. Impression, click, and auction events flow through
Redpanda to the event-aggregator, which writes PostgreSQL hourly rollups and
hosts the Reporting API.

Phase 1 provides a shared operational shell in `internal/platform/app`, wired by
each command. Configuration, logging, metrics, tracing, health, HTTP/gRPC,
lifecycle, clocks, IDs, and exact money are independent platform packages.
Prometheus scrapes native admin ports 8090, 8091, and 8093. Grafana uses
Prometheus; services send OTLP traces to Jaeger on port 4317.

HTTP request flow is request ID → tracing → logging → metrics → recovery → mux.
The metrics/logging layers observe the mux's route template instead of raw
paths or query strings. HTTP and gRPC providers and metric registries are injected
per service; no default global mux/registry/provider is used.

The lifecycle group owns listener workers. Shutdown closes readiness, waits for
load-balancer draining, stops listeners under a fresh deadline, then runs ordered
flush/close hooks. The gRPC helper force-stops after its graceful deadline and
joins its helper goroutine. Domain flush hooks will be added alongside their
Kafka/database components.

See [the decisions](DECISIONS.md) for the eight required ADRs. Detailed diagrams,
operational flows, and dashboards are completed in Phase 11.

## Campaign administration (Phase 2)

The campaign HTTP listener authenticates all routes with the shared API-key
middleware. Handlers decode bounded JSON and delegate to the campaign service.
The service validates inputs and transitions, then uses a pgx repository with
sqlc-generated parameterized queries. Writes run in transactions; mutations
lock and merge the current row, preventing lost updates between PATCH requests.
Activation and creative changes synchronize on the campaign row. Foreign keys,
the partial advertiser-name unique index, and database constraints provide the
final persistence checks. Every catalog mutation bumps the version through the
schema triggers, ready for the Phase 3 snapshot implementation.

Pagination reads one extra row in descending `(created_at,id)` order; opaque
cursors include the resource/filter scope. Monetary fields remain integer micros
through storage and service code and are rendered by the shared exact JSON type.
Campaign runtime owns both listeners, DB-aware readiness, the pool, and telemetry;
shutdown drains listeners before closing the database pool.

## Catalog serving (Phase 3)

REST and gRPC share one snapshot service. A cheap version query serves unchanged
gRPC responses and detects cache hits. Cache misses enter a context-aware gate,
recheck the version, then load the catalog in one SQL statement. Its MVCC view
covers catalog_version and every related record, so concurrent commits cannot
mix generations. Published protobuf objects are immutable and callers receive
deep copies. There is no background snapshot refresher on campaign-service.
Generated timestamps stay stable for a cached version.

The query excludes inactive campaigns/parents and campaigns without active
creatives, supplies unrestricted UTC targeting when absent, and selects only
referenced bidders. Request-time dates are carried through rather than filtered.
Integer micros are decoded directly into int64, then sent as protobuf integers
or protobuf-JSON strings. Conditional REST reads use the same version and retain
request-ID/authentication behavior.

## Mock Bidders & Seeding (Phase 4)

Simulated demand partners (DSPs) run within one process (`cmd/mock-bidder`)
serving N independent HTTP listeners on consecutive ports (9101..910N). Each bidder
handles `POST /bid` by evaluating candidates against the impression floor and pricing
under the active bid mode (`fixed`, `shade`, or seeded `random`). Knobs for latency,
jitter, simulated error rate, timeout rate, and no-bid rate are tunable at runtime via
`GET/POST /admin/config` without process restart.

The `cmd/seed` utility populates the campaign database via REST, idempotently registering
bidders (`bidder-1..N`), creating a test advertiser, and configuring 40 active campaigns
with varied targeting constraints across countries, device types, placements (`slot-0..9`),
and creative dimensions (300x250).

## Bid Gateway Core (Phase 5)

The bid gateway (`cmd/bid-gateway`) receives incoming client bid requests on `POST /v1/bid`.
The end-to-end request pipeline executes strictly within the latency budget (default 100 ms hard
bidder deadline, target p99 < 120 ms):

1. **Decode & Validate**: The JSON request payload is decoded with a 16 KiB limit (`REQUEST_MAX_BYTES`).
   Fields (placement, dimensions, floor up to 6 decimals, device type, ISO 3166-1 alpha-2 country)
   are validated.
2. **In-Memory Snapshot Cache**: Catalog data is retrieved via gRPC polling from `campaign-service`
   every 2 seconds (`SNAPSHOT_REFRESH_INTERVAL`) and held in an `atomic.Pointer[SnapshotCache]`.
   Zero database calls are performed on the hot bidding path. If campaign-service experiences an
   outage, the gateway continues serving from the last known snapshot (stale-while-error).
3. **Targeting Filter**: Incoming requests are evaluated in memory against compiled campaign targeting
   rules (date windows, country inclusion, device type, placement ID, hour of day in campaign timezone,
   and creative size match). Matching campaigns are grouped by `bidder_id`.
4. **Resilient Parallel Fan-out**: The gateway dispatches parallel HTTP requests to all eligible bidders
   using a buffered result channel sized to the group count. Each bidder interaction is guarded by:
   - A circuit breaker (`gobreaker`) that trips if consecutive failure ratios exceed threshold.
   - A concurrency bulkhead (`semaphore.Weighted`, default 512 inflight per bidder).
   - Per-bidder deadline (`min(bidder.timeout_ms, remaining_request_deadline)`).
   - Limit reader on responses (64 KiB cap).
   - Post-validation verifying candidate ownership and price bounds.
5. **Deterministic Second-Price Auction & Re-Auction Loop**: Bids are filtered against the impression floor.
   Valid bids are sorted by CPM descending, then CampaignID ascending, then BidderID ascending. The winning
   campaign pays the second-highest bid plus 1 cent ($0.01 = 10,000 micros), clamped by its own max bid.
   If the winner fails atomic budget reservation in Redis (due to daily cap exhaustion or pacing), the winner is
   dropped and the auction is re-evaluated with the remaining bids (up to `MAX_AUCTION_ATTEMPTS=3`).
6. **Atomic Redis Budget Reservation (`internal/budget`)**:
   - Spend is reserved in integer micros using an embedded Lua script (`reserve.lua`) executed via `redis.NewScript` (`EVALSHA`).
   - Keys are formatted as `pb:budget:{campaign_id}:{YYYYMMDD}` with 48h TTL set on first write.
   - Pacing modes: `asap` (full daily budget) and `even` (time-proportional daily fraction with burst fraction).
   - Local in-memory `ExhaustedCache` (5s TTL) suppresses dead campaigns from targeting on subsequent requests.
   - Compensation refunds (`release.lua`) clamp at 0 via atomic Lua script.
   - Fail-closed: Redis timeouts or connectivity failures return HTTP 204 with `X-PulseBid-Reason: error`.
7. **Response & Diagnostics**: Returns HTTP 200 with winning campaign details, clearing price in USD,
   creative markup with populated event URL tokens, `Server-Timing` headers, and optional `X-PulseBid-Debug`
   diagnostics in test mode. HTTP 204 responses carry explicit reasons (`no_eligible`, `below_floor`,
   `no_bids`, `budget`, `error`).

## Event Tracking & Kafka Publishing (Phase 7)

The bid gateway exposes high-throughput event tracking endpoints and publishes durable events to
Redpanda Kafka for reporting and reconciliation:

1. **Signed HMAC-SHA256 Event Tokens (`internal/events/token`)**:
   - When an auction is won, signed tracking URLs (`impression_url`, `click_url`) are constructed and
     substituted into the winning creative markup (`{impression_url}`, `{click_url}`).
   - Compact JSON payload keys (`b`, `r`, `c`, `cr`, `a`, `bd`, `p`, `u`, `iat`, `exp`) minimize URL length.
   - Cryptographic HMAC-SHA256 signature with constant-time verification (`hmac.Equal`) prevents token tampering.
   - Tokens carry a 24-hour expiration (`EVENT_TOKEN_TTL`), returning HTTP 410 on expiration.
2. **Deterministic Event IDs (ADR-5)**:
   - Generated via UUIDv5 (`NamespacePulseBid = 0193f000-0000-7000-8000-000000000000`) over `"<type>:<bid_id>"`.
   - Duplicate pixel fires or redelivered Kafka messages generate identical event IDs, guaranteeing exactly-once
     effect in Phase 8 deduplication.
3. **High-Throughput Durable Publisher (`internal/events`)**:
   - Built with `github.com/twmb/franz-go` (`kgo`), publishing to topic `pulsebid.events.v1` across 6 partitions.
   - Partition key is `campaign_id` ensuring strict FIFO ordering per campaign.
   - Critical events (`impression`, `click`) publish synchronously with `RequiredAcks(AllISRAcks)` and Snappy compression.
   - Best-effort `auction` events emit asynchronously over a bounded channel (10,000 slots); drops increment metrics without slowing down the bidding engine.
   - Every Kafka message carries W3C OpenTelemetry `traceparent`, `event_type`, and `schema_version` headers.
4. **Tracking Endpoints**:
   - `GET /v1/events/impression?t=<token>`: Validates token, emits `impression` event (`spend_micros = PriceMicros`), returns 200 transparent 1x1 GIF (`image/gif`) with `Cache-Control: no-store`.
   - `GET /v1/events/click?t=<token>`: Validates token, emits `click` event (`spend_micros = 0`), returns HTTP 302 Found redirect to destination landing URL with `Cache-Control: no-store`.

## Event Aggregation & Deduplication (Phase 8)

The event aggregator (`cmd/event-aggregator`) consumes durable event records from Kafka topic `pulsebid.events.v1`
and maintains real-time deduplicated hourly analytics rollups in PostgreSQL:

1. **High-Throughput Consumer Loop (`internal/aggregator`)**:
   - Consumer group `pulsebid-aggregator` configured with `github.com/twmb/franz-go` (`kgo`).
   - Manual offset commit (`kgo.DisableAutoCommit()`) commits offsets back to Kafka only after the database transaction commits.
   - Batch chunking: accumulates records up to `BATCH_SIZE` (default 500) or flushes on `FLUSH_INTERVAL_MS` (default 200ms).
   - Distributed tracing: extracts W3C OpenTelemetry trace context from incoming Kafka headers and creates child spans.
   - Resiliency: exponential backoff on transient database failures (capped at 5s) before retrying the batch.

2. **Atomic Single-Transaction Deduplication & Rollup Store (`store.go`)**:
   - Executes inside a single atomic PostgreSQL transaction:
     - `INSERT INTO processed_events (event_id, occurred_at) VALUES (...) ON CONFLICT DO NOTHING RETURNING event_id`.
     - Newly inserted IDs determine which events are aggregate-eligible; duplicates are discarded.
     - Deterministic sorting: rollup keys `(campaign_id, hour_start)` are sorted lexicographically before issuing SQL upserts,
       completely eliminating database lock deadlocks across concurrent partition workers.
     - Additive upsert into `hourly_stats` (`impressions = hourly_stats.impressions + EXCLUDED.impressions`, `clicks`, `spend_micros`, `bids`, `wins`).

3. **Dead Letter Queue (`dlq.go`)**:
   - Malformed records (unparseable JSON, invalid UUID formats) are isolated from the batch before database interaction.
   - Published immediately to `pulsebid.events.dlq.v1` with diagnostic headers: `error`, `original_topic`, `original_partition`, and `original_offset`.

4. **Retention Pruning Worker (`retention.go`)**:
   - Periodic background worker (hourly interval) purges entries in `processed_events` older than `DEDUP_RETENTION` (default 168h).
   - Batched delete queries prevent vacuum stalls and row lock escalation.

## Reporting API (Phase 9)

The reporting subsystem (`internal/reporting`) runs inside `cmd/event-aggregator` on HTTP port `:8083` and serves
aggregated campaign performance statistics:

1. **Endpoints & Routing**:
   - `GET /v1/reports/campaigns`: Multi-campaign or account-wide performance metrics filtered by time range and campaign list.
   - `GET /v1/reports/campaigns/{id}`: Single-campaign performance metrics over a time range.
   - Protected by API key authentication (`X-API-Key` header verified against configured keys).

2. **Query Granularities**:
   - `hour`: Binned by `hour_start` directly or `date_trunc('hour', hour_start)`. Maximum time range: 31 days.
   - `day`: Binned by `date_trunc('day', hour_start)`. Maximum time range: 366 days.
   - `total`: Aggregated across the entire requested range, grouped strictly by `campaign_id`. Maximum time range: 366 days.

3. **Metrics & Safe Math**:
   - Computes:
     - `CTR = clicks / impressions` (safeguarded against divide-by-zero: 0.0 when impressions == 0).
     - `win_rate = wins / bids` (safeguarded against divide-by-zero: 0.0 when bids == 0).
     - `spend = spend_micros / 1,000,000.0`.
   - Computes overall response `totals` across all matching rows server-side.
   - Enforces a 50,000 row cap (`LIMIT 50001` with HTTP 400 `too_many_rows` if exceeded).
   - Unknown campaigns return HTTP 200 with `data: []` and 0 totals.

## Resilience & Fault Isolation (Phase 10)

The exchange guarantees that external dependency failures never cascade or degrade overall latency:

1. **Bidder Isolation & Circuit Breakers (`internal/gateway/fanout.go`)**:
   - Each bidder has a dedicated bulkhead semaphore (`BIDDER_MAX_INFLIGHT=512`) to prevent connection pool starvation.
   - Each bidder has a dedicated `gobreaker.TwoStepCircuitBreaker`:
     - Trips to `Open` when failure ratio exceeds 50% across at least 20 requests.
     - While `Open`, returns instant `circuit_open` outcome (< 1ms, zero network I/O).
     - After `CBOpenFor` (5s), transitions to `Half-Open` and allows 1 probe request to test bidder health.
   - Per-request context deadline (`min(bidder.timeout_ms, remaining deadline)`) drops slow bidders at deadline.
   - Response body reading is capped at 64 KiB (`io.LimitReader`) and always drained to reuse TCP connections.

2. **Dependency Outage Behavior**:
   - **Slow or Failing Bidder**: Isolated at deadline; healthy bidders still participate in auction; status 200 returned within budget.
   - **All Bidders Down**: Gateway returns HTTP 204 with `X-PulseBid-Reason: no_bids`.
   - **Redis Down/Slow**: Budget reservation fails closed; gateway returns HTTP 204 with `X-PulseBid-Reason: error`; `/readyz` fails; zero overspend possible.
   - **Kafka Down**: Bidding remains operational; auction events are dropped best-effort; impression/click endpoints return HTTP 503.
   - **Campaign Service Down**: Gateway continues serving bids from in-memory snapshot cache; `/readyz` stays healthy; `snapshot_age_seconds` increments.
   - **PostgreSQL Down**: Campaign API and Reporting API return HTTP 503; aggregator pauses Kafka offset commits and retries with backoff.

3. **Graceful Draining & Zero Dropped Requests**:
   - On `SIGTERM`/`SIGINT`, `ready.Drain()` marks `/readyz` 503 immediately, giving load balancers time to stop routing traffic.
   - HTTP server drains active requests up to `SHUTDOWN_TIMEOUT` (15s); Kafka publisher flushes buffered events; database/Redis connections close cleanly.

## Observability & Telemetry (Phase 11)

PulseBid features end-to-end operational visibility across metrics, tracing, structured logs, and dashboards:

1. **Metrics Collection (Prometheus)**:
   - Exported across dedicated admin listeners:
     - `bid-gateway`: `:8090/metrics` (`bid_requests_total`, `bid_duration_seconds`, `bidder_requests_total`, `bidder_latency_seconds`, `bidder_inflight`, `bids_invalid_total`, `auction_clearing_price_micros`, `snapshot_version`, `snapshot_age_seconds`, `snapshot_campaigns`, `events_published_total`, `event_buffer_depth`, `budget_reserve_total`, `redis_pool_acquired`, `redis_pool_idle`).
     - `campaign-service`: `:8091/metrics` (`db_pool_acquired`, `db_pool_idle`, `redis_pool_acquired`, `redis_pool_idle`).
     - `event-aggregator`: `:8093/metrics` (`aggregator_batch_size`, `aggregator_batch_duration_seconds`, `aggregator_events_total`, `aggregator_consumer_lag`, `db_pool_acquired`).
     - `mock-bidders`: `:9101..9108/metrics` and `:8094/metrics` (`http_server_requests_total`, `http_server_request_duration_seconds`, `bidder_decisions_total`).
   - Cardinality is strictly bounded: no metric labels ever record `campaign_id`, `request_id`, or `user_id`.

2. **Distributed Tracing (OpenTelemetry -> Jaeger)**:
   - Root span `POST /v1/bid` on gateway contains detailed child spans:
     - `bid.validate`: JSON decode and request bounds validation.
     - `bid.targeting`: in-memory candidate matching against snapshot catalog.
     - `bid.fanout`: parallel outbound dispatcher.
       - `bidder.call`: one child per candidate bidder with attributes `bidder.id` and `outcome`.
         - Propagates W3C `traceparent` HTTP header to mock bidders, producing downstream `POST /bid` server spans.
     - `bid.auction`: second-price auction resolution.
     - `bid.budget_reserve`: Redis atomic budget reservation attempt.
     - `bid.respond`: HMAC-SHA256 token signing and JSON response construction.
   - Kafka event publishing injects `traceparent` into Kafka record headers; `event-aggregator` consumes records and links the batch processing span via `trace.WithLinks(links...)`.

3. **Structured Logging (`log/slog`)**:
   - JSON output to stdout with standardized correlation fields: `time`, `level`, `msg`, `service`, `version`, `request_id`, `trace_id`, `span_id`.
   - Normal bidding requests log at `Debug`; requests with latency > 110ms or outcome `error` log at `Warn`.
   - Zero sensitive PII logged: creatives, user IDs, and raw IPs are excluded.

4. **Grafana Dashboards & Alert Rules**:
   - Four automated dashboards provisioned from `deploy/grafana/dashboards/`:
     - `exchange-overview.json`: Bid RPS, fill rate, p50/p95/p99 latency vs 120ms threshold, stacked outcomes, clearing CPM.
     - `bidders.json`: per-bidder p99 latency, timeout rate, error rate, circuit breaker state, inflight requests.
     - `budget-and-events.json`: reserve results, p99 reserve latency, publish rate, buffer depth, consumer lag, duplicate ratio.
     - `go-runtime.json`: goroutines, heap allocation, GC pause, CPU usage, Postgres/Redis pool utilization.
   - Prometheus alert rules mounted in `/etc/prometheus/alerts.yml`: `BidLatencyHigh`, `FillRateLow`, `SnapshotStale`, `ConsumerLagHigh`.




