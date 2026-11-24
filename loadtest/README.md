# Load testing

The binding targets are in [Sections 16–18 of the build plan](../PulseBid%20Build%20Plan.md).
Use [current verification](../docs/VERIFICATION.md) for measured results.

Run the gateway and eight mock bidders natively; leave PostgreSQL, Redis, Kafka,
and campaign-service running. Seed 40 campaigns with bidder endpoints reachable
from the native gateway (`BIDDER_HOST=http://127.0.0.1 make seed`). Normal mock
latency is 10ms plus up to 15ms of jitter; fixed pricing keeps auction outcomes
reproducible. Set `OTEL_TRACES_SAMPLER_ARG=0.01` and record CPU/RAM, Go version,
`GOMAXPROCS`, process layout, and any configuration changes with the results.

```bash
K6_BIN=/path/to/k6 VEGETA_BIN=/path/to/vegeta bash scripts/run-phase12-load-and-profile.sh
# Or individual runs:
make load RATE=1000 DURATION=30s
make load RATE=3000 DURATION=30s
make load RATE=5000 DURATION=2m
```

`k6/bid.js` uses a constant arrival rate and varied seeded targeting. Thresholds
require p99 below 120ms, HTTP failures below 1%, checks above 99%, zero dropped
iterations, and actual fills. p99 appears in both text and JSON summaries.
`k6/slow-bidder.js` injects 500ms latency into bidder-1 and restores its complete
original configuration afterward. `k6/overspend.js` is a separate exact-cap
scenario launched by `scripts/test-multi-replica-overspend.sh`.

The full driver collects CPU, heap, goroutine, block, and mutex profiles during
the two-minute 5k run. Enable `ENABLE_TEST_ENDPOINTS=true` on the local gateway
to collect sampled block/mutex contention. It preserves k6's nonzero exit code;
latency or scheduling failures must never be reported as successful load proof.
The Vegeta cross-check stores raw, JSON, and text output and independently checks
success, p99 latency, and achieved throughput.

Reports go to `docs/test-reports/<date>/verification/`; override `REPORT_DIR`
when comparing setups. Keep failed baselines as well as passing runs. Postman
proves API behavior; its sequential execution cannot establish concurrent load
or replica safety.
