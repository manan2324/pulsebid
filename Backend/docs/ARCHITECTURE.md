# Architecture

PulseBid separates the catalog control plane from the auction request path and asynchronous reporting.

## Request flow

1. The Next.js console authenticates the operator and proxies requests to backend APIs with a server-side API key.
2. The campaign service stores advertisers, bidders, campaigns, targeting, and creatives in PostgreSQL. REST supports management operations; gRPC publishes catalog snapshots for the gateway.
3. The gateway caches the catalog, validates incoming bid requests, applies targeting and date rules, and fans out eligible candidates to bidders under request deadlines.
4. The auction validates returned bids and selects a winner. Redis Lua scripts reserve budget atomically across gateway replicas, enforcing daily caps and pacing.
5. The gateway returns the winning creative and signed tracking URLs. Auction, impression, and click events are published to Redpanda with deterministic event identifiers and trace context.
6. The event aggregator deduplicates events and updates PostgreSQL reporting aggregates. The reporting API serves the console.

## Boundaries

- The campaign catalog is authoritative in PostgreSQL; gateway snapshots reduce database work on the bid path. Readiness and snapshot-age monitoring identify unavailable or stale catalog state.
- Monetary values use integer micros internally. Auction and budget validation must preserve exact conversion and cap enforcement.
- Budget state is coordinated through Redis rather than per-process counters.
- Event delivery can repeat. Deduplication and database transactions protect reporting from duplicate processing.
- Bidder deadlines, concurrency limits, and circuit breakers constrain external failures.
- Admin listeners and internal gRPC endpoints are trusted-network interfaces. Public deployment requires network restrictions and appropriate transport security.
- The console uses a signed session and a shared administrator password. Login throttling is process-local.

## Observability

Services emit structured logs, Prometheus metrics, and OpenTelemetry traces. Grafana dashboards cover auction outcomes, bidders, budgets/events, and Go runtime behavior. Jaeger stores traces in a persistent Badger volume in the local stack.

## Source layout

- [cmd](../cmd/): service entrypoints and seed tooling.
- [internal](../internal/): domain behavior, platform utilities, persistence, and HTTP/gRPC handlers.
- [api](../api/): OpenAPI and protobuf contracts.
- [gen](../gen/): generated contract bindings.
- [migrations](../migrations/): PostgreSQL schema migrations.
- [deploy](../deploy/): container definitions and observability configuration.
- [loadtest](../loadtest/): concurrent workload and failure-injection scenarios.

See the [runbook](./RUNBOOK.md) for operational commands and the [root README](../../README.md) for deployment constraints.
