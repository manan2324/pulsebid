# PulseBid backend

Go services implement campaign management, catalog snapshots, bid auctions, distributed budget reservation, event tracking, and reporting. PostgreSQL stores catalog and reporting data, Redis coordinates budget reservations, and Redpanda carries events.

Use the [repository quickstart](../README.md) to run the complete stack, including the console. Commands below run from this directory.

## Services and contracts

| Component | Public listener | Admin listener |
| --- | --- | --- |
| Bid gateway | 8080 HTTP | 8090 |
| Campaign service | 8081 REST, 8082 gRPC | 8091 |
| Event aggregator/reporting | 8083 HTTP | 8093 |
| Mock bidders | 9101-9108 HTTP | Service configuration dependent |

Campaign and reporting requests require `X-API-Key`. Refer to the [API definitions](./api/), [Postman collection](./postman/), and [architecture](./docs/ARCHITECTURE.md). Admin listeners provide health, readiness, and metrics endpoints and belong on trusted networks.

## Native development

Install Go matching [go.mod](./go.mod), Bash, GNU Make, Python 3, and Docker with Compose v2. Copy [.env.example](./.env.example) to `.env` for backend Compose commands. Native commands read process environment variables; they do not automatically load that file.

```sh
make up
make topics
make migrate-up
make build
```

Set service-specific environment variables before running `make run-campaign`, `make run-gateway`, or `make run-aggregator` in separate terminals. Use `make bidders N=8` to run local mock bidders and `make seed` to create sample data. Ensure bidder addresses are reachable from the gateway: container hostnames and native localhost addresses differ.

## Verification

```sh
make lint build
make test
make test-int
make test-api
make test-grpc
make test-platform
make fuzz FUZZTIME=10s
```

Race tests require a supported C toolchain. Linux CI exercises the full integration workflow; Windows users can use WSL for Bash-based checks. Integration and API tests require Docker and may start application containers or create test records. Use dedicated test services rather than production data.

Use `make generate` after editing protobuf or SQL generation inputs, and commit the resulting [generated bindings](./gen/). Frontend declarations are regenerated with `npm --prefix ../frontend run gen:api`. Keep generated source under version control; execution reports and profiling captures are ignored.

For throughput, latency, failure injection, and budget checks, see [load testing](./loadtest/README.md). Passing sequential API checks does not establish concurrent capacity.

## Operations

See the [runbook](./docs/RUNBOOK.md) for startup, readiness, bidder tuning, reporting, and tracing. `make down` preserves volumes. `make reset CONFIRM=1` deletes local data and is only appropriate when an intentional reset is required.

The bundled deployment uses development credentials and mock services. Review [deployment requirements](../README.md#deployment) before exposing a deployment.
