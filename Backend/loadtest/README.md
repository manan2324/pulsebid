# Load testing

Run tests only against an environment you control. Start dependencies, the catalog service, the gateway, and reachable mock bidders, then seed matching campaigns using the [quickstart](../../README.md). Ensure campaigns have sufficient budget and creatives for the requested targeting.

Install k6 on your PATH. From the backend directory:

```sh
k6 run -e RATE=100 -e DURATION=30s loadtest/k6/bid.js
k6 run -e RATE=1000 -e DURATION=30s loadtest/k6/bid.js
k6 run -e RATE=3000 -e DURATION=30s loadtest/k6/bid.js
```

Increase traffic gradually while observing CPU, memory, bidder latency, Redis, consumer lag, and application errors. `GATEWAY` overrides the default http://127.0.0.1:8080. `PRE_VUS` and `MAX_VUS` control load-generator concurrency. The script's default rate is 5,000 requests/second; always set an explicit rate for initial checks.

The bid scenario requires p99 latency below 120ms, HTTP failure rate below 1%, checks above 99%, zero dropped iterations, and at least one fill. A nonzero exit code means a threshold or execution failure. A configured target is not evidence of achieved throughput, and a passing low-rate run does not establish higher capacity.

For scripted reports and profiling:

```sh
make load RATE=1000 DURATION=30s
K6_BIN=/path/to/k6 VEGETA_BIN=/path/to/vegeta bash scripts/run-load-and-profile.sh
```

The profiling driver runs sustained high-load scenarios and collects CPU, heap, goroutine, block, and mutex evidence. It requires the corresponding tooling and profiling endpoints in a trusted test environment. Record the hardware, versions, process layout, sampling rate, and configuration alongside results.

[k6/slow-bidder.js](./k6/slow-bidder.js) injects latency and restores the original bidder configuration. [k6/overspend.js](./k6/overspend.js) is used by [the replica verification script](../scripts/test-multi-replica-overspend.sh) for budget-cap checks. The [Vegeta scenario](./vegeta/) provides an independent traffic generator.

Generated reports normally go under backend `docs/test-reports`, which is ignored by Git. Export evidence separately when needed. The bid load scenario does not fire impression/click tracking links; use simulator or API checks to test tracking and reporting end to end. Sequential Postman checks cannot establish concurrent capacity or replica safety.
