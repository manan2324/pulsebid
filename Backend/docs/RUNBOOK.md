# Operations runbook

Use the [root quickstart](../../README.md) for startup. Commands here run from the repository root unless stated otherwise. Use a dedicated test environment for seed data, fault injection, and load testing.

## Startup and diagnostics

```sh
docker compose config --quiet
docker compose --profile apps ps -a
docker compose --profile apps logs --tail 100
```

Start Docker with Linux containers. Check port conflicts and available memory when dependencies fail readiness. `POSTGRES_PORT` can override the published database port; native applications must use the same port in `DATABASE_URL`. Apply migrations before using the catalog: database liveness alone does not prove the schema exists.

## Health and readiness

The gateway admin listener is http://localhost:8090, campaign service http://localhost:8091, and aggregator http://localhost:8093. Probe `/healthz`, `/readyz`, and `/metrics`. Liveness does not imply dependency readiness. Inspect structured logs for startup configuration errors, unavailable dependencies, stale snapshots, or shutdown drain events.

Campaign REST requests require `X-API-Key`. Missing or incorrect keys produce 401. Validation failures produce 400; conflicting catalog operations can produce 409. Database outages can produce 503. Restore the dependency and inspect logs before considering destructive recovery actions.

## Catalog and no-fill investigation

Inspect advertisers, active campaigns, active creatives, dates, targeting, and bidder endpoints. Confirm the requested placement, country, device, and floor match the configured campaign. Review daily budget and pacing before raising traffic. The gateway snapshot must contain eligible campaigns and refresh successfully.

`GET /v1/snapshot` on the campaign API supports version ETags. Budget state is available at `GET /v1/campaigns/{id}/budget`. Both require an API key. Test-only time overrides and budget resets must remain disabled on public deployments.

## Mock bidder tuning

The local bidders expose `/admin/config` on ports 9101 through 9108. Inspect the current configuration before changing latency, jitter, errors, timeouts, no-bid rate, or pricing. Restore the original configuration after fault injection. The console and [load scenarios](../loadtest/) provide tooling for controlled testing.

## Events and reporting

An auction response alone does not create a tracked impression or click. Follow the response's signed tracking URLs to exercise those paths. Invalid or expired tokens are rejected. Events flow through `pulsebid.events.v1`; failed records can reach `pulsebid.events.dlq.v1`.

```sh
docker compose exec -T redpanda rpk topic list
docker compose exec -T redpanda rpk topic consume pulsebid.events.v1 -n 5
```

If reporting is delayed, check consumer lag, aggregator readiness and logs, PostgreSQL write latency, and dead-letter records. Event processing is deduplicated; repeated tracking delivery should not inflate reporting counters.

## Metrics and alerts

Open Grafana at http://localhost:3000 and Prometheus at http://localhost:9090. The local Grafana default credentials are `admin` / `admin` on first startup; replace them before deployment.

- High bid latency: inspect bidder latency/timeouts, Redis reservation latency, CPU, and garbage collection.
- Low fill rate: inspect outcome labels, targeting, floors, available budget, and bidder failures.
- Stale snapshots: check campaign-service connectivity, gRPC health, and gateway refresh logs.
- Growing consumer lag: inspect broker availability, database locks, aggregator throughput, and failed records.

Treat alert thresholds as operational signals. Establish capacity and latency baselines on the actual deployment hardware using the [load-testing guide](../loadtest/README.md).

## Jaeger UI

Open http://localhost:16686 and select the gateway service to inspect auction traces. HTTP and event messages carry trace context.

If the page displays a UI placeholder, rebuild the custom image. It embeds the checksum-pinned official Jaeger 1.67.0 UI assets and retains the Badger storage preload patch. A plain Go build from the module archive can omit those assets while still passing HTTP health checks.

```sh
docker compose build jaeger
docker compose up -d --no-deps --no-build --wait jaeger
```

Confirm **Find Traces** appears and assets load. The `jaeger-data` volume is retained; deleting it is unnecessary for this repair. The [browser regression check](../../frontend/e2e/jaeger.spec.ts) verifies the trace-search interface and services API.

## Shutdown and data retention

```sh
docker compose --profile apps --profile tools --profile scale down
```

This preserves named volumes. Use graceful shutdown to let services drain outstanding work. Removing volumes or using backend `make reset CONFIRM=1` intentionally deletes local data and is not an outage-repair step.
