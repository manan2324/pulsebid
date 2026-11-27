# PulseBid

PulseBid is a real-time bidding exchange with a web console for managing advertisers, campaigns, creatives, and bidders. It includes an auction simulator, budget pacing, event tracking, reporting, and operational dashboards.

The backend uses Go, PostgreSQL, Redis, and Redpanda. The console uses Next.js, React, and TypeScript. Prometheus, Grafana, and Jaeger provide metrics and distributed tracing.

## Repository

| Path | Contents |
| --- | --- |
| [frontend](./frontend/) | Web console, API proxy, authentication, and browser tests |
| [Backend](./Backend/) | Services, migrations, API contracts, and backend tests |
| [Backend/deploy](./Backend/deploy/) | Container builds and observability configuration |
| [.github/workflows](./.github/workflows/) | Backend verification and frontend acceptance workflows |

## Requirements

- Docker Engine or Docker Desktop with Linux containers and Docker Compose v2.24.4 or newer.
- Node.js 22.19 or newer within the Node.js 22 release line and npm for frontend development.
- Go matching [go.mod](./Backend/go.mod), plus Bash, GNU Make, and Python 3 for native backend development and verification. Windows users can use WSL or Git Bash for Bash-based commands.

## Run the complete application

Run these commands from the repository root. Copy the environment template first (PowerShell: `Copy-Item .env.example .env`; Bash: `cp .env.example .env`).

Edit your local environment file and set `ADMIN_PASSWORD` and a random `SESSION_SECRET` of at least 32 characters. Keep `PULSEBID_API_KEY` and `API_KEYS` consistent. Never commit real credentials.

```sh
docker compose up -d --wait postgres redis redpanda prometheus grafana jaeger
docker compose --profile tools run --build --rm migrate up
docker compose exec -T redpanda rpk topic create pulsebid.events.v1 pulsebid.events.dlq.v1 --partitions 3 --replicas 1
docker compose --profile apps up -d --build --wait
docker compose --profile tools run --build --rm seed
```

Topic creation is needed on a fresh installation; already-existing topics do not need recreation. Seeding registers the mock bidders and sample campaigns. It is intended for development and demonstrations.

Open [the console](http://localhost:3001) and sign in using the password you configured. The frontend does not have a fixed built-in administrator password.

| Service | Local address |
| --- | --- |
| Console | http://localhost:3001 |
| Bid gateway | http://localhost:8080 |
| Campaign API | http://localhost:8081 |
| Reporting API | http://localhost:8083 |
| Grafana | http://localhost:3000 |
| Prometheus | http://localhost:9090 |
| Jaeger | http://localhost:16686 |

The standalone frontend development configuration is separate from the root Compose configuration. See the [frontend guide](./frontend/README.md).

## Use the console

1. Review the dashboard and service status.
2. Create or inspect an advertiser, then configure its campaigns, creatives, targeting, budget, and bidder.
3. Activate a campaign with an active creative and a reachable bidder.
4. Open the simulator, send a request matching the campaign targeting, and inspect the auction result. A fill requires an eligible campaign, a valid bid above the floor, and available budget.
5. Fire the returned tracking links to generate impression/click events, then inspect reporting. Auction-only traffic does not imply recorded impressions or clicks.
6. Use the system page to inspect service health and open Grafana or Jaeger.

## Development and verification

```sh
cd frontend
npm ci
npm run dev
```

Run source checks from the frontend directory:

```sh
npm run check:lock
npm run check:api
npm run lint
npm run typecheck
npm test
```

Use `npm run build` followed by `npm start` to test the production frontend build. Browser acceptance, coverage, and performance commands are documented in the [frontend guide](./frontend/README.md).

From the backend directory, use `make lint build`, `make test`, and `make test-int` as appropriate. Integration, API, and load tests require infrastructure and may create test data; use a dedicated test environment. See [backend development](./Backend/README.md) and [load testing](./Backend/loadtest/README.md).

CI recreates verification reports as downloadable artifacts. Reports, screenshots, profiling captures, caches, installed dependencies, and build output are excluded from Git. Test source, fixtures, lockfiles, and generated API bindings are retained so checks remain reproducible.

## Deployment

Container builds and production frontend output are provided, but the bundled Compose stack is a local reference environment. It uses demo infrastructure credentials, mock bidders, and enabled backend test endpoints. Do not expose it publicly unchanged.

For a deployment, configure TLS and public origins, replace API/database/event-token secrets and Grafana credentials, disable test endpoints and API mocking, and restrict database, broker, admin, metrics, and tracing access. Configure durable storage, backups, resource limits, and readiness monitoring for your environment. The console uses a single shared administrator password and process-local login throttling; multi-user access and distributed throttling require additional authentication infrastructure.

Run functional and load verification on the intended deployment hardware before setting capacity targets. This repository does not establish a passing sustained 5,000 requests/second capacity guarantee.

See the [architecture](./Backend/docs/ARCHITECTURE.md), [operations runbook](./Backend/docs/RUNBOOK.md), and environment templates: [root](./.env.example), [backend](./Backend/.env.example), [frontend](./frontend/.env.example).

## Stop services

```sh
docker compose --profile apps --profile tools --profile scale down
```

Named volumes are preserved. Adding `--volumes` deletes stored application and observability data.

## License

[MIT](./LICENSE).
