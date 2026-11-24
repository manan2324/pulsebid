# Verification — 2026-11-24

The implementation and test repairs from the audit are in this working tree.
Phase 12 remains open until the required sustained throughput and latency proofs
pass. The older “100% complete” statements are superseded by this report.

## Repairs

- Dead-letter publication failures return before committing source Kafka offsets.
- Async auction events retain originating trace headers. Flush waits for the
  application queue before flushing Kafka; close and enqueue are synchronized.
- Outbound bidder request bytes are owned by each request, including transports
  that close request bodies asynchronously after returning an error.
- Bidder transport failures are classified before cancellation; caller
  cancellation does not trip breakers, while genuine timeouts do.
- Response-body read errors and oversized bidder responses are rejected and
  count toward the breaker; latency includes reading the body. Cached tracer
  handles remove repeated provider lookups from handler/fan-out execution.
- Redis operations now apply their configured deadline and clients enable
  context deadlines. Re-auction reservations share a bounded response deadline.
- Bid warning logs are sampled one in 100, including the HTTP middleware.
- Scale tests use the existing Compose override, dynamic admin ports, and a
  Docker-29-compatible Traefik pin. The test checks traffic on every replica.
- Seed pagination reads `data`, so repeated seeding finds existing resources.
- `make fuzz` runs request, money, and token fuzzers. k6 source is no longer
  accidentally excluded from version control.
- Integration tests fail on unavailable services when `REQUIRE_INTEGRATION=1`.
  Kafka tests use private topics, preventing application consumers or old
  fixtures from masking crash/deduplication failures. CI starts services first.
- Lint errors, stale test assertions, incomplete load runners, and outdated
  quickstart instructions have been corrected. Load failures propagate to exit
  status; no proof accepts zero spend or merely “at least one” impression.

## Historical November 23 results

| Check | Result | Evidence |
| --- | --- | --- |
| Full API regression | 199 requests, 497 assertions, zero failures | [Newman report](./test-reports/2026-11-23/phase-12/postman-report.html) |
| Linux lint/build | Passed; zero lint issues | [Go verification](./test-reports/2026-11-23/verification/go-verification.txt) |
| Race + required integration suite | Passed against PostgreSQL, Redis, and Kafka | [Go verification](./test-reports/2026-11-23/verification/go-verification.txt) |
| Concurrency/crash repetition | Budget, gateway, aggregator passed `-race -count=5` | [Go verification](./test-reports/2026-11-23/verification/go-verification.txt) |
| Active fuzzing | Request, money, token: 10 seconds each; passed | [Go verification](./test-reports/2026-11-23/verification/go-verification.txt) |
| Three-replica overspend | 2,000 requests split 667/666/667; 49 fills; 49,490 micros spent; 510 remaining | [Exact proof](./test-reports/2026-11-23/verification/overspend-proof.json) |
| Duplicate replay/restart | Three fires per event; exactly one impression/click; spend 1,010 micros | [Exact proof](./test-reports/2026-11-23/verification/duplicate-proof.json) |
| Performance acceptance | Not yet passed; see raw results | [k6 5k](./test-reports/2026-11-23/verification/k6-5000rps.txt) |

## November 24 revalidation

- Full Linux lint/build, required race/integration suite, five repetitions of
  budget/gateway/aggregator tests, three active fuzzers, benchmarks, and five
  real SIGTERM-under-load checks passed. [Full log](./test-reports/2026-11-24/verification/go-verification.txt).
- Newman passed all 199 requests and 497 assertions using a migrated private
  database. Repository/retention tests now use private databases too. An earlier
  API run exposed interference from old fixtures; that failed run is retained.
  [HTML report](./test-reports/2026-11-24/phase-12/postman-report.html).
- gRPC snapshot and health checks passed (`SERVING`).
  [Health](./test-reports/2026-11-24/phase-3/grpc-health.json).
- A clean source clone with empty project volumes completed the documented
  quickstart in 83 seconds, produced 40 active campaigns, added none on repeat
  seed, and returned a real bid. Images/modules were cached; this is not a cold
  download timing. [Log](./test-reports/2026-11-24/verification/quickstart.txt).
- Four provisioned dashboard definitions, four alert rules, a distributed bid
  trace with bidder child spans, and a structured request log were captured.
  Grafana's existing password differs from the development default, so dashboard
  JSON came from provisioning files; no screenshots or authenticated API export
  are claimed. [Provenance](./test-reports/2026-11-24/verification/observability.json).
- Jaeger now starts against its existing Badger volume after repairing repeated
  service-index scans. The image build runs the pinned upstream spanstore tests;
  existing traces were preserved. See [decision](./DECISIONS.md).
- Replay/restart again produced exactly one impression, one click, and 1,010
  micros spend after three sends of each event.
  [Proof](./test-reports/2026-11-24/verification/duplicate-proof.json).
- **The fresh three-replica exact-fill proof failed twice.** The first run
  returned 41 fills, the warm repeat 37, rather than 49. The first campaign spent
  exactly 49,490 of its 50,000 micros, but Redis response deadlines can leave
  uncertain reservations charged without a successful bid response. This is
  consistent with fail-closed/no-refund semantics; it does not satisfy the
  stronger exact-fill assertion. These failures are preserved:
  [cold](./test-reports/2026-11-24/verification/failed-cold-proof/overspend-proof.txt),
  [warm](./test-reports/2026-11-24/verification/failed-warm-proof/overspend-proof.txt).
- The corrected proof checks the plan's zero-overspend guarantee while reporting
  uncertain reservations instead of asserting they became delivered fills.
  **Passed on the final gateway image:** 2,000 requests split 667/666/667,
  49 reservations, 35 delivered fills, 14 unconfirmed reservations, 93 error
  no-fills, three bidder no-fills, exact spend 49,490 and remaining 510 micros.
  Winner/price checks and positive fills remain mandatory. This is a budget
  safety pass, not proof of fault-free delivery or performance.
  [Proof](./test-reports/2026-11-24/verification/overspend-proof.json).
- **Performance acceptance still failed.** The 5k, two-minute native k6 run
  completed 402,001 requests (~3,346/s), dropped 197,544 iterations, and measured
  p99 about 1.21s. Its sampled server-duration p99 was about 446ms. Only 264
  requests filled; 315,161 reported no bids. HTTP 200/204 success alone therefore
  provides no acceptable exchange-performance proof.
  [k6](./test-reports/2026-11-24/verification/k6-5000rps.txt).
  Vegeta generated 300,000 requests at 5,000/s, successful throughput ~4,985/s,
  p99 1.35s, and 924 timeouts.
  [Cross-check](./test-reports/2026-11-24/verification/vegeta.txt).
- CPU pressure was high with zero measured memory pressure. The CPU profile
  was distributed across syscalls, runtime/allocation, and instrumentation;
  contention implicated repeated tracer-provider lookups. A diagnostic two-CPU
  k6 run did not fix latency. Cached handler/fan-out tracer handles passed five
  race-enabled gateway runs and final lint/build (zero issues). The post-change
  1k run still failed: p99 ~1.10s and 30 dropped iterations. Disabling private
  profiling also failed: p99 ~683ms and 197 drops. No performance pass or proven
  speedup is inferred from these noisy comparisons. The full 5k measurements
  above precede the tracer caching; acceptance is still open on final code.
  [CPU](./test-reports/2026-11-24/verification/cpu-profile.txt),
  [mutex](./test-reports/2026-11-24/verification/mutex-profile.txt),
  [post-change race tests](./test-reports/2026-11-24/verification/cached-tracers-race.txt),
  [final lint/build](./test-reports/2026-11-24/verification/final-lint-build.txt),
  [post-change load](./test-reports/2026-11-24/verification/cached-tracers/k6-1000rps.txt),
  [profiling off](./test-reports/2026-11-24/verification/profiling-off/k6-1000rps.txt).
- A final one-CPU-per-service diagnostic still failed: 1k p99 ~524ms and 40
  drops. No CPU-allocation experiment established acceptance. The later
  response-body correctness repair passed five race-enabled gateway runs and
  its valid-JSON overflow regression passed separately. Final image budget,
  replay, and observability checks were rerun after this repair.
  [Race log](./test-reports/2026-11-24/verification/final-gateway-race.txt),
  [CPU-allocation trial](./test-reports/2026-11-24/verification/single-cpu-services/k6-1000rps.txt).

## Performance setup and limits

Hardware: Intel Core i5-1335U, 12 logical processors, approximately 16GB host RAM;
Ubuntu under WSL2 has about 7.6GB RAM. Core services and the load generator share
this laptop. Native trials use gateway/bidders/campaign-service executables and
a private Redis 7.4.2 process on 16379; PostgreSQL, Kafka, and ingestion run in
Docker. Gateway and bidders use `GOMAXPROCS=4`; campaign-service uses 2. Mock DSP
latency remains 10ms plus 0–15ms jitter, fixed bidding, no injected errors.
Tracing samples 1%; private contention profiles use sampled collection.

Failed baselines are retained. One 1k run achieved p99 92.54ms, but subsequent
runs were not consistently below 120ms. A completed earlier two-minute 5k run
processed 554,886 requests (~4,613 rps), dropped 45,079 scheduled iterations,
and had p99 501.31ms despite zero HTTP status failures. These results do **not**
meet acceptance. Higher GC settings and a smaller load-generator pool worsened
the result and are not recommended defaults. CPU and memory pressure affect
both services and client; no claim that hardware alone explains the failure is
made without further evidence.

All performance thresholds remain enforced. Discarding unused response bodies
in k6 reduces client memory use while still downloading complete HTTP responses;
API payload correctness is independently covered by Newman. See
[Grafana’s load-generation guidance](https://grafana.com/docs/k6/latest/testing-guides/running-large-tests/).

## Reproduce

Follow the [quickstart](../README.md), then run:

```bash
DATABASE_URL='postgres://pulsebid:pulsebid@localhost:15432/pulsebid?sslmode=disable' bash scripts/verify-linux.sh
bash scripts/test-api.sh
bash scripts/test-multi-replica-overspend.sh
bash scripts/test-duplicate-replay.sh
bash scripts/run-phase12-load-and-profile.sh
```

Use port 5432 instead of 15432 if that is your published PostgreSQL port.
Run load proofs separately from compilation/integration workloads. Generated raw
reports remain local under `docs/test-reports/`; this checked-in summary describes
the working-tree verification and does not claim a remote CI execution.
