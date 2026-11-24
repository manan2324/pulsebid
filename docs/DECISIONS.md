# Architecture decisions

The [build plan](../PulseBid%20Build%20Plan.md), especially Section 4.4, is the
source of truth. ADR-1 through ADR-8 below are planned contracts, not Phase 0
implementations.

## ADR-1: Snapshot cache

The gateway replaces an `atomic.Pointer[Snapshot]` wholesale. It polls
`GetSnapshot(known_version)` every `SNAPSHOT_REFRESH_INTERVAL` (2s by default).
Unchanged responses avoid rebuilding; outages keep the last snapshot and expose
`snapshot_age_seconds`. This avoids locks and database calls on the hot path.

## ADR-2: Fan-out

Use one goroutine per eligible bidder and a results channel buffered to the
bidder count. The collector selects between results and the deadline. Do not use
errgroup cancellation on the first bidder error: failures must remain isolated,
and late results must not block or leak goroutines.

## ADR-3: Budget reservation

Reserve the clearing price after selecting a winner. On exhausted or paced
budget, remove that bid and rerun the auction because the price may change.
Allow at most three attempts before returning no fill.

**Phase 6 implementation details:**
- **Atomic Lua scripts:** Budget reservation runs via `internal/budget/reserve.lua` executed with `redis.NewScript` (`EVALSHA`). Because Redis executes Lua scripts atomically on a single thread, race conditions across multiple gateway replicas are impossible.
- **Keys and TTL:** Spend is tracked under `pb:budget:{campaign_id}:{YYYYMMDD}` (UTC) with a 48-hour TTL set on first write (`TTL < 0`), enabling automatic clean rollover without midnight locking.
- **Integer micros:** All spend math uses `int64` micros (`INCRBY`). Floating-point money arithmetic is strictly forbidden on the server side.
- **Pacing calculations:**
  - `asap`: `pace_cap = hard_cap`.
  - `even`: `pace_cap = min(hard_cap, floor(daily_budget * min(1, secondsIntoUTCDay/86400 + PACING_BURST_FRACTION)))` (default burst fraction 0.05).
- **Compensation refunds:** `internal/budget/release.lua` executes `DECRBY` clamped to 0, preventing negative spend under any refund scenario while preserving key TTL.
- **Local exhausted cache:** Bid gateways maintain an in-memory `ExhaustedCache` with a 5-second TTL to skip dead campaigns during targeting, avoiding unnecessary bidder calls.
- **Fail-closed:** Any Redis timeout or error immediately returns HTTP `204` with `X-PulseBid-Reason: error`, protecting advertiser budgets against infrastructure degradation.

## ADR-4: Charging

Charge spend on auction win. Impression events drive reported spend. Unrendered
wins are not refunded; reservation expiry and refunds are a stretch goal.

## ADR-5: Event IDs

Use `UUIDv5(NS_PULSEBID, "<type>:<bid_id>")`. Pixel retries and Kafka redelivery
therefore share an ID and can be deduplicated exactly.

## ADR-6: Delivery semantics

Use an idempotent producer with `acks=all` and retries. Commit consumer offsets
only after the database transaction commits: at-least-once delivery with
exactly-once effect.

## ADR-7: Event priority

Auction events use a bounded, nonblocking, best-effort channel; dropped events
are counted. Impression/click events are critical and return HTTP 503 if they
cannot be enqueued.

## ADR-8: Reporting placement

The event-aggregator hosts the Reporting API in a separate `internal/reporting`
package, keeping deployment simple while allowing a later split.

## Bootstrap choices (2026-11-18)

- The supplied plan is named `PulseBid Build Plan.md`. Keep that original as the
  single source of truth rather than creating a duplicate with the requested
  uppercase spelling.
- The first phase means Phase 0, the first unchecked entry. Keep future-phase
  directories with `.gitkeep`; future Make targets fail explicitly with the
  required phase. Proto/sqlc configuration is introduced alongside its inputs
  in Phases 2–3, avoiding invalid empty generation configuration.
- Use the installed Go 1.27.3 and golangci-lint v2.15.0 (resolved from the Go
  module proxy). Invoke the linter at that pinned version through `go run` so
  developers and CI use the same tool without modifying application dependencies.
- Keep local infra images versioned, publish ports on loopback, and use named
  volumes. Kafka advertises distinct native and container listeners so both
  future execution modes receive reachable broker metadata.
- Add a static BusyBox binary to the Jaeger all-in-one image solely for an HTTP
  health check. Persist its Badger storage in a named volume. The minimal base
  image does not otherwise guarantee an HTTP probe executable.
- Do not initialize Git or create commits implicitly. All files are ready for
  version control; the starting workspace was not a Git repository.
- Docker was hidden from the sandbox but available to approved commands. Use
  the installed Docker Desktop runtime for infrastructure verification. Native
  Go race tests cannot use the installed 32-bit GCC; run `make test build` in
  the Linux Go 1.27.3 container with GCC, Bash, and Make instead.
- An existing PostgreSQL process occupies host port 5432. Add optional
  Compose-only `POSTGRES_PORT` (default 5432), documented in plan Section 7.6.
  Local `.env` selects 15432 and the matching `DATABASE_URL`, preserving the
  existing server and all default contracts.

## Phase 1 platform choices (2026-11-18)

- Phase 1 explicitly requires admin endpoints for every command, including mock
  bidders and seed. Use otherwise unused defaults 8094/8095 and document them
  in plan Section 7.1. Seed temporarily runs an operational shell, then becomes
  the finite REST seeding command in Phase 4. No public business routes or
  dependency readiness checks are implemented ahead of their assigned phases.
- Keep logger, Prometheus registry, tracer provider, and W3C propagator explicit.
  `net/http/pprof` handlers are registered only on the private admin mux; its
  standard-library default-mux registrations are never served. Pprof errors are
  normalized into the same JSON envelope without buffering successful profiles.
- Readiness checks and HTTP timeout middleware are cooperative and receive bounded
  contexts; callbacks must honor cancellation. This avoids spawning abandoned
  goroutines to enforce timeouts. HTTP/gRPC listener shutdown has explicit forced
  cleanup; hooks run in listener/flush/pool/telemetry order under a fresh deadline.
- Log only route patterns and bounded metadata. Request IDs accept printable ASCII
  of 1–128 bytes; invalid incoming IDs are replaced with UUIDv7. API-key comparison
  uses fixed-size SHA-256 digests and constant-time comparison across every key.
- Money accepts JSON decimal and exponent syntax with exact string arithmetic.
  Both the written fractional component and the expanded decimal must fit six
  places; exponent forms never introduce rounding. Reject null, overflow, and
  excess precision. Round per-impression cost by quotient/remainder so even the
  maximum int64 CPM cannot overflow during rounding.
- The shared HTTP error mapper accepts wrapped typed public errors and hides
  unknown/private causes. Later service packages keep their own domain sentinels
  and wrap them with the corresponding public code at the HTTP boundary.
- The Phase 1 plan says Postman: none. Document operational routes in the admin
  OpenAPI file and cover them with Go and real-process smoke tests. Postman's
  Health/Setup/Validation folders remain Phase 2 work.

## Phase 2 campaign decisions (2026-11-18)

- Implement every Section 11.2 route except snapshot and budget, which remain
  Phases 3 and 6. Use the full Section 8 schema, including the future reporting
  tables, as required by Phase 2. Migrations are embedded read-only assets,
  applied explicitly; API startup never silently changes the schema.
- Generate pgx/v5 queries with sqlc 1.31.1 into `gen/campaigndb`. Reuse generated
  records whose shapes match the domain; map campaign storage micros into the
  separate decimal-JSON domain model. Keep SQL parameterized and generated code
  reproducible via `make sqlc`.
- Mutations lock the current row inside a transaction before merging PATCH
  fields. Creative changes and activation also lock their campaign. Enforce the
  same activation precondition through PATCH and action routes. Draft can
  activate, active can pause, paused can reactivate, and any state can archive;
  archived cannot leave that state. Repeating a current state is allowed, with
  the active-creative requirement still enforced for active status.
- POST campaign always starts draft; supplied status is rejected. IDs and
  timestamps are read-only, except the client-supplied bidder slug on creation.
  Creative campaign_id is immutable. Unknown fields are rejected for all admin
  writes. Null clears only nullable fields (email, timeout, dates, dimensions).
- Targeting PUT is full replacement: omitted arrays become empty and timezone
  defaults to UTC. Missing targeting GET for an existing campaign returns the
  unrestricted defaults with its creation timestamp. Normalize country codes,
  sort/deduplicate restrictions, reject the machine-dependent `Local` timezone,
  and embed Go's IANA timezone data for distroless deployments.
- Use 512 KiB as the campaign transport body limit so JSON-escaped 64 KiB
  creative markup fits. This does not change the gateway's 16 KiB contract.
  Validate names at 1–200 characters, positive optional dimensions, and http(s)
  URLs without embedded credentials. Lists include archived records unless the
  campaign status filter excludes them. Cursors bind to resource and filters.
- pgx reads/writes have five-second deadlines; rollback uses a separate bounded
  cleanup context. Unique/FK violations become conflict, missing rows become
  not_found, and connection failures (including closed-connection/EOF races)
  become unavailable. Internal SQL errors are not exposed in HTTP messages.
- The golang-migrate driver lacks contextual Up/Steps methods. Bound its connect,
  statement, and lock work using connection/server/driver timeouts. Down reverses
  one version. Tests verify up/down/up only in disposable testcontainers.
- Integration tests are opt-in with the integration build tag and explicitly
  uncached; CI runs them. The ordinary unit suite needs no Docker. The platform
  process smoke test now requires a migrated DB for campaign readiness.
- Add app-profile containers for campaign and the existing gateway/aggregator
  admin shells, plus a tools-profile migration job. A tiny loopback-only health
  probe supports distroless health checks. Auxiliary migrate/healthcheck commands
  are finite tools, not additional business-service processes.
- Newman runs 78 requests in folders 00–02 with unique run-scoped fixtures and
  HTML/JUnit/JSON reports. Snapshot assertions and bidder runtime tuning remain
  deferred to their phases. Docker runs pinned Newman/reporter packages, so no
  host Node installation is required. Git Bash path conversion is disabled only
  for Docker run, with the Windows bind path normalized through cygpath.

## Phase 3 catalog decisions (2026-11-19)

- Keep Section 11.5's protobuf fields/numbers exactly as specified. Generate
  locally with pinned Buf 1.75.0, protoc-gen-go 1.36.12, and
  protoc-gen-go-grpc 1.6.2 in a Docker tool image. Reflection supports the planned
  grpcurl command without supplying a separate descriptor file.
- Build the entire catalog with one SQL statement using JSON aggregation. Its
  MVCC view includes both version and records, including empty snapshots. Use
  exact typed int64 decoding, not generic JSON numbers/floats. Include only
  bidders referenced by included campaigns, and only active creatives sorted
  by ID. Missing targeting becomes empty arrays plus UTC; null dimensions and
  bidder timeout become protobuf zero defaults. Carry date windows unchanged.
- Share a cache between both transports. Query version on every call; matching
  known_version returns version/unchanged only, even on a cold server. A
  context-aware gate serializes cache rebuilds and rechecks version after waiting.
  Atomically publish an immutable message and return deep copies. Rebuild failure
  does not replace a good cached snapshot, but the server returns an error rather
  than claiming its old cache is current. Inject the clock for generated_at.
- REST follows protobuf JSON with UseProtoNames and emitted scalar/list defaults:
  snake_case keys, int64 strings, enum names, UTC timestamp strings, and omitted
  unset message timestamps. This is documented in the plan/OpenAPI. ETag is the
  quoted decimal version; GET uses weak If-None-Match comparison, supports tag
  lists and wildcard, and returns a bodyless 304 after auth and version checks.
- The gRPC listener is trusted internal plaintext in local development, matching
  the specified unauthenticated grpcurl command. API-key auth remains on REST.
  Health reports startup/draining plus DB readiness via an owned one-second
  health worker; snapshot calls themselves always use the bounded DB check.
  Mark gRPC NOT_SERVING before the drain delay, then stop HTTP/gRPC before pools.
- No background catalog polling is added to campaign-service. Gateway polling,
  stale-while-error handling, and targeting evaluation remain Phase 5 work.

## Phase 4 mock bidder decisions (2026-11-19)

- Implement each mock bidder as an independent HTTP server on consecutive ports
  `base-port + i` (default 9101-9108), serving `/bid`, `/healthz`, `/readyz`,
  `/admin/config`, and `/metrics`.
- Use `atomic.Pointer[Config]` and atomic compare-and-swap for thread-safe runtime
  configuration updates via `POST /admin/config` without server reboots.
- Random mode pricing uses deterministic seeding (`seed + fnv64(request_id + campaign_id)`)
  so tests and simulation runs are completely reproducible.
- Enforce strict bidding boundaries: price is always `>= floor` and `<= max_bid_cpm`,
  and rounded to at most 6 decimal places. Empty candidates or all candidates below
  floor return HTTP 204 No Content.
- Start an auxiliary admin listener on `ADMIN_ADDR` (default `:8094`) to integrate
  with platform smoke tests and centralized readiness probes.
- Implement `cmd/seed` as an idempotent CLI command that checks existing bidders,
  advertisers, and campaigns before creation. Support fallback to the admin shell if
  `ADMIN_ADDR` is provided without `RUN_SEED=1`, maintaining test-platform compatibility.

## Phase 5 bid gateway core decisions (2026-11-19)

- Model and validation: Implement OpenRTB-like bid request model in `internal/gateway/model.go`.
  Strictly validate positive dimensions, valid device types (`mobile|desktop|tablet|ctv`),
  uppercase ISO alpha-2 country codes, and exact 6-decimal-place CPM bid floors (<= $1000).
- Snapshot polling & cache: Cache protobuf catalog snapshot wholesale in `atomic.Pointer[Snapshot]`
  in `internal/gateway/snapshot.go`. Refresher polls campaign-service gRPC `GetSnapshot(known_version)`
  every `SNAPSHOT_REFRESH_INTERVAL` (default 2s) with `unchanged=true` support. On gRPC failure,
  continue serving the existing snapshot (stale-while-error) and increment `snapshot_age_seconds`.
  Readiness probe (`/readyz`) requires at least one snapshot successfully loaded.
- Targeting evaluation: Compile incoming campaigns into immutable `CompiledCampaign` structs
  using hash sets (`map[string]struct{}`) for O(1) checks across countries, device types,
  placements, and hours of day with preloaded `*time.Location`. Check time windows, timezone hour
  (including half-hour offsets like `Asia/Kolkata`), and creative size match (exact width/height
  or first creative if request size is omitted). Group nominated candidates by `bidder_id`.
- Fan-out concurrency: Execute outbound HTTP calls to candidate bidders in parallel goroutines
  reporting to a buffered channel sized to `len(groups)` so late or hanging bidders never block
  or leak goroutines. Protect each bidder with `gobreaker` circuit breaker and `semaphore.Weighted`
  bulkhead (`BIDDER_MAX_INFLIGHT=512`). Cap per-bidder timeout to `min(bidder.timeout_ms, remaining_deadline)`.
  Enforce 64 KiB response limit with `io.LimitReader` and post-validate that returned bids match
  candidate campaigns and creative IDs, and `0 < price <= max_bid_cpm`.
- Deterministic 2nd-price auction: Implement pure `RunAuction` function filtering bids by `price >= floor`,
  sorting by CPM descending, CampaignID ascending, and BidderID ascending. Calculate clearing CPM as
  `min(winner.CPM, max(second.CPM, floor) + 10_000)` micros ($0.01 increment). Provide `Without()`
  helper for budget re-auction loops in Phase 6.
- Outcome categorization: On HTTP 204 no-fill, emit header `X-PulseBid-Reason` with distinct values:
  `no_eligible` (no campaigns matched targeting), `below_floor` (either bids submitted were below floor
  or all candidate campaigns have max bid below floor), `no_bids` (bidders returned no bids or errors),
  `budget` (budget exhausted/paced), and `error` (transport/system failure).
- Stubs behind final interfaces: Provide `Reserver` interface with `StubReserver` (always reserved)
  and `EventPublisher` with `NoopEventPublisher`, allowing `POST /v1/bid` to function end-to-end with
  server timing and unsigned placeholder URLs until Phases 6 and 7.
- Test mode enhancements: Under `ENABLE_TEST_ENDPOINTS=true`, support `X-PulseBid-Test-Time` to override
  request clock time for targeting tests, and emit `X-PulseBid-Debug` header detailing eligible,
  responded, timed out, error, and valid bid counts.

## Phase 7 event tracking decisions (2026-11-19)

- Schema and versioning: Implemented `internal/events/model.go` with `schema_version = 1`.
  Fixed namespace constant `NamespacePulseBid = uuid.MustParse("0193f000-0000-7000-8000-000000000000")`.
  `EventID` generated via deterministic UUIDv5 over `"<type>:<bid_id>"`, guaranteeing that retry
  fires of tracking pixels and Kafka redeliveries share identical event IDs for atomic deduplication (ADR-5).
- Compact HMAC-SHA256 event tokens: Built `internal/events/token` package using `base64.RawURLEncoding`
  with constant-time verification (`hmac.Equal`). Tokens use short single-letter JSON keys (`b`, `r`,
  `c`, `cr`, `a`, `bd`, `p`, `u`, `iat`, `exp`) and are signed with `EVENT_TOKEN_SECRET` with configurable
  `EVENT_TOKEN_TTL` (default 24h). Honors `X-PulseBid-Test-Time` in test mode for expiry validation (410 token_expired).
- Priority event publishing: Critical events (`impression`, `click`) publish synchronously via `franz-go`
  with `AllISRAcks` and return HTTP 503 on buffer saturation. Best-effort `auction` events emit asynchronously
  over a bounded buffer (10,000 slots) with non-blocking drops recorded in Prometheus metrics (ADR-7).
- Kafka topic routing and ordering: Messages are published to `pulsebid.events.v1` with `campaign_id` as
  partition key to ensure strict per-campaign partition ordering across all 6 partitions (Section 10.4).
  Headers inject W3C OpenTelemetry `traceparent`, `event_type`, and `schema_version`.
- Gateway responses: `GET /v1/events/impression` returns HTTP 200 with 43-byte valid transparent GIF (`image/gif`)
  and `Cache-Control: no-store`. `GET /v1/events/click` returns HTTP 302 Found redirect to the token's `landing_url`
  with `Cache-Control: no-store`. Winning bids substitute `{impression_url}` and `{click_url}` into creative `adm`.

## Phase 8 event aggregation decisions (2026-11-19)

- Single Database Transaction for Atomic Deduplication & Rollup: Deduplication against `processed_events`
  (`INSERT INTO processed_events ... ON CONFLICT DO NOTHING RETURNING event_id`) and additive rollup into
  `hourly_stats` occur within the exact same database transaction. If the transaction fails, neither the
  ledger nor the rollup changes; if it commits, both change atomically (ADR-6).
- Deadlock-Free Sorted Upserts: Rollup updates for `(campaign_id, hour_start)` are deterministically sorted
  prior to issuing SQL upserts within the transaction. This completely avoids PostgreSQL row-lock deadlocks
  across concurrent partition consumers or multiple aggregator instances.
- Manual Offset Commits: Auto-commit is disabled (`kgo.DisableAutoCommit()`). Kafka offsets are committed
  only after the database transaction successfully commits. On crash before DB commit, events are re-processed
  with zero loss. On crash after DB commit before offset commit, events are safely deduplicated on restart
  with zero double counting.
- Poison Pill DLQ Routing: Malformed JSON payloads or non-UUID campaign/participant identifiers are detected
  during batch decoding and forwarded to `pulsebid.events.dlq.v1` with diagnostic headers (`error`,
  `original_topic`, `original_partition`, `original_offset`), preventing poison pills from blocking consumer
  progress.
- Bounded Retention Pruning: A background worker runs on a periodic schedule (default 1h) purging
  `processed_events` records older than `DEDUP_RETENTION` (default 168h / 7 days) using batched deletes
  to prevent vacuum and lock contention on high-volume tables.

## Phase 9 reporting API decisions (2026-11-19)

- Server-Side Aggregation & Safe Division: Granularities (`hour`, `day`, `total`) are computed server-side
  directly in PostgreSQL via `date_trunc` and grouping to minimize database-to-application traffic. CTR,
  win_rate, and spend decimals are computed in the service layer with strict zero-division safeguards:
  `CTR = clicks / impressions` (0.0 if impressions == 0), `win_rate = wins / bids` (0.0 if bids == 0),
  and `spend = spend_micros / 1,000,000.0`.
- Deterministic Ordering & Hard Row Cap: Queries order rows deterministically by `bucket ASC, campaign_id ASC`.
  To protect against memory exhaustion without complex cursor pagination, queries issue `LIMIT 50001`; if
  50,001 rows return, the service immediately returns HTTP 400 (`too_many_rows`) prompting the client to
  narrow the query range.
- Empty Result Semantics for Unknown Campaigns: When querying a campaign ID that does not exist or has zero
  traffic in the requested range, the API returns HTTP 200 with an empty data array (`data: []`) and zeroed
  totals (`totals: { impressions: 0, clicks: 0, spend: 0, bids: 0, wins: 0, ctr: 0, win_rate: 0 }`),
  rather than 404, matching analytics API conventions.
- Standardized Error Handling: Error codes are mapped via `httpx.Status`, standardizing on `"invalid_request"`
  for bad query parameters and `"too_many_rows"` for oversized result sets.
- API Key Authentication: All reporting routes (`/v1/reports/*`) are authenticated via `httpx.APIKey(apiKeys)`
  requiring an `X-API-Key` header matching configured keys, consistent with the Campaign Service administrative
  and write routes.

## Phase 10 resilience hardening decisions (2026-11-19)

- Circuit Breaker Tuning and Logging: Configured per-bidder `gobreaker.TwoStepCircuitBreaker` with parameters
  from configuration (`CBMinRequests: 20`, `CBFailureRatio: 0.5`, `CBOpenFor: 5s`). Added `OnStateChange` hook
  logging transitions with slog Warn logs. When open, requests return instant `circuit_open` outcome without
  initiating outbound HTTP connections.
- Straggler Accounting in Debug Header: When fan-out deadline expires before all candidate bidders deliver
  results, uncompleted bidders are accounted for as `timed_out` in `X-PulseBid-Debug` so that
  `eligible == responded + timed_out + errors` holds consistently under timeouts.
- Fail-Closed Budget Policy: When Redis is down or encounters an operation timeout, `reserver.Reserve()` returns
  an error. The gateway fails closed, returning HTTP 204 with `X-PulseBid-Reason: error` and incrementing
  `budget_reserve_total{result="error"}`. This guarantees that Redis network partitions never cause runaway overspend.
- Stale-While-Error Snapshot Serving: If `campaign-service` is unreachable, `SnapshotCache.Poll()` logs a warning
  and preserves the in-memory snapshot. `/readyz` continues to report healthy (200), ensuring the gateway continues
  serving bids even if the campaign control plane is down.
- Graceful Draining: On `SIGTERM`/`SIGINT`, `ready.Drain()` flips `/readyz` to 503 immediately while allowing
  in-flight HTTP requests up to `SHUTDOWN_TIMEOUT` (15s) to complete, preventing connection resets on upstream proxies.

## Phase 11 observability decisions (2026-11-20)

- OpenTelemetry Trace Hierarchy & Global Registration: Registered service `TracerProvider` and `Propagator` globally
  at service bootstrap via `otel.SetTracerProvider()` and `otel.SetTextMapPropagator()`. Instrumented `POST /v1/bid`
  handler with consecutive sibling child spans (`bid.validate`, `bid.targeting`, `bid.fanout`, `bid.auction`,
  `bid.budget_reserve`, `bid.respond`) and parallel nested child spans (`bidder.call` with attributes `bidder.id` and
  `outcome`). Outbound HTTP calls propagate `traceparent` headers to downstream mock DSPs, which construct linked
  server spans.
- Low-Cardinality Metric Labeling: Adhered strictly to Section 13.1 guidelines by never using high-cardinality values
  such as `campaign_id` or `request_id` as Prometheus metric labels. Partitioned `bids_invalid_total` by `reason`
  (`not_in_candidate_list`, `price_non_positive`, `price_invalid_or_exceeds_max`).
- Connection Pool Observation: Registered non-blocking connection pool collectors (`db_pool_acquired`, `db_pool_idle`,
  `redis_pool_acquired`, `redis_pool_idle`) across `bid-gateway`, `campaign-service`, and `event-aggregator` to observe
  pool utilization without executing database or cache ping operations on Prometheus scrape passes.
- Provisioned Dashboards & Alert Rules: Implemented declarative dashboard JSON specifications matching the 4 required
  dashboards (`exchange-overview.json`, `bidders.json`, `budget-and-events.json`, `go-runtime.json`) mounted in
  `/var/lib/grafana/dashboards`. Mounted declarative Prometheus alert rules in `/etc/prometheus/alerts.yml` covering
  high latency (`BidLatencyHigh`), low fill rate (`FillRateLow`), stale snapshot cache (`SnapshotStale`), and consumer
  lag growth (`ConsumerLagHigh`).






## Audit repairs (2026-11-23)

- Preserve the JSON/event wire contracts. Change only the internal publisher API
  to `TryPublishAsync(ctx, event)`: serialize at enqueue time so Kafka headers
  preserve the server trace without retaining request contexts or mutable events.
  A queue barrier makes `Flush` include application-buffered auctions; the worker
  uses nonblocking Kafka enqueue and bounded delivery. Close synchronizes with
  enqueue so no event is accepted after draining has completed.
- Leave source offsets uncommitted on a failed DLQ write. The consumer exits with
  an error, allowing its owner/restart to retry instead of silently losing the
  poison record. DLQ publication itself has a bounded context.
- Apply Redis operation timeouts inside the reserver and enable
  `ContextTimeoutEnabled` on service clients. Keep at most 20ms after the fan-out
  deadline for reserve/response work, clamped to an earlier caller deadline;
  re-auction attempts share that deadline. Redis failures remain fail-closed.
- Each bidder request owns its serialized body. A pooled buffer cannot be reused
  on `Do` return because `RoundTripper` may close a body asynchronously. Classify
  transport errors before cancelling their contexts to distinguish real errors
  from timeouts. Caller cancellation is excluded from breaker failures.
- Require external-service tests in CI and `make test-int`; optional local unit
  runs may still skip. Give Kafka tests private six-partition topics and cleanup
  so existing messages or the application consumer cannot satisfy test assertions.
- Use `deploy/docker-compose.scale.yml` for replicas, with ephemeral admin host
  ports and public event URLs through Traefik on 8088. Pin Traefik v3.6.16 for
  the Docker API used by Docker 29; see the official
  [migration documentation](https://doc.traefik.io/traefik/v3.6/migrate/v3/).
- System proofs use isolated campaigns and exact counts/spend, never a database-
  wide Redis flush. Seed through the tools-profile container after starting the
  campaign service. Python 3 supplies JSON parsing, pagination, polling, and
  buffered Vegeta output; shell scripts remain the entry points.
- Keep load thresholds and exit codes binding. k6 source is tracked; only its
  downloaded executable is ignored. Failed profiles/results are retained and
  Phase 12 stays open until the actual latency/throughput targets pass.

## Persistent Jaeger startup repair (2026-11-24)

- A restart after load testing stalled inside Badger `preloadOperations`. The
  v1.67 reader returned a service name for each indexed span, causing its caller
  to rescan operation indexes repeatedly. The local image now builds the same
  pinned Jaeger source with `jaeger-prefill.patch`: collect each service once,
  retain its latest expiration (including non-expiring entries), and avoid
  fetching values during the key-only scan. The database format, volume, and
  existing traces are preserved. Upstream spanstore tests run in the image build;
  startup against the existing database now reaches healthy status.
- Source: [Jaeger v1.67 Badger reader](https://github.com/jaegertracing/jaeger/blob/v1.67.0/internal/storage/v1/badger/spanstore/reader.go).

## Test isolation and scheduling (2026-11-24)

- Repository/retention tests create migrated private PostgreSQL databases and
  drop only those databases afterward. The Newman runner uses a separate
  database and restores the normal app configuration on exit. Old API fixtures
  can no longer win auctions belonging to a later run.
- The budget concurrency test allows two seconds per operation while retaining
  exact accepted/rejected/spend assertions. A fake-transport re-auction test
  allows one second for race-instrumented scheduling. Production deadlines and
  dedicated deadline/resilience assertions are unchanged; these tests establish
  correctness rather than machine-speed limits.

## Performance profiling and budget proof scope (2026-11-24)

- Cache gateway handler and fan-out tracer handles at construction. Repeated
  `otel.Tracer` lookups contended on the SDK provider during load. Span names,
  context propagation, and sampling remain the same. Five race-enabled gateway
  runs passed; the subsequent 1k comparison still failed latency acceptance.
- The safety proof checks the planned zero-overspend guarantee: exactly 49
  reservations of 1,010 micros fit in a 50,000-micro budget, positive successful
  fills cannot exceed those reservations, and all three replicas receive traffic.
  It reports unconfirmed reservations and error/no-bid response counts. Requiring
  exactly 49 successful HTTP responses conflicts with the documented no-refund
  behavior when a Redis response times out after executing its mutation. The two
  earlier failures of that stronger assertion are preserved; performance failure
  remains independently enforced and disclosed.

- Reject bidder response-body read failures before interpreting HTTP 200/204.
  An empty broken body previously became a successful no-bid; even valid partial
  JSON must fail if the read failed. Record latency through body consumption,
  classify deadlines before cancellation, and count these failures toward the
  breaker. Detect 64 KiB overflow using one extra byte and close without an
  unbounded drain. Regression tests cover empty/valid JSON read failures,
  deadline errors, oversize bodies, closure, and breaker opening.
