# PulseBid console

The console provides advertiser, campaign, creative, and bidder management, an auction simulator, reporting, and system health views. It uses Next.js 15, React 19, TypeScript, TanStack Query, and Tailwind CSS.

## Local development

Start the backend using the [repository quickstart](../README.md). From this directory:

```sh
npm ci
```

Copy [.env.example](./.env.example) to a local environment file named `.env.local` (PowerShell: `Copy-Item .env.example .env.local`; Bash: `cp .env.example .env.local`). Set `ADMIN_PASSWORD`, a random `SESSION_SECRET` of at least 32 characters, and the API key matching the backend. The configured URLs must be reachable from the Next.js server.

```sh
npm run dev
```

Open http://localhost:3001. Sign in with the password in your local configuration. Restart the development server after changing credentials. Root Compose reads the root environment file; it does not read the standalone frontend environment file.

Keep `AUTH_DISABLED=false`, `TEST_MODE_UI=false`, and `NEXT_PUBLIC_API_MOCKING=disabled` when using the real backend. `APP_ORIGIN` must match the browser origin for mutation checks. Never put server secrets in `NEXT_PUBLIC_*` variables.

## Build

```sh
npm run build
npm start
```

The container uses standalone Next.js output. See [Dockerfile](./Dockerfile) and the [root Compose configuration](../docker-compose.yml). Browser-visible Grafana and Jaeger URLs are public build-time configuration; use addresses reachable by the browser.

## Checks

| Command | Purpose |
| --- | --- |
| `npm run check:lock` | Check native-platform lockfile entries |
| `npm run check:api` | Check generated API declarations against backend contracts |
| `npm run gen:api` | Regenerate API declarations after contract changes |
| `npm run lint` | ESLint checks |
| `npm run typecheck` | TypeScript checks |
| `npm test` | Unit and component tests |
| `npm run test:coverage` | Tests with coverage output |
| `npm run e2e` | Playwright browser checks |
| `npm run report` | Open the latest Playwright report |
| `npm run analyze` | Build with bundle analysis |
| `npm run check:bundles` | Enforce bundle budgets after a build |
| `npm run check:secrets` | Inspect public build output for secret leakage |
| `npm run lighthouse` | Measure production pages locally |
| `npm run report:archive` | Package generated verification evidence |

Install the test browser once with `npx playwright install chromium`. See [playwright.config.ts](./playwright.config.ts) and the [acceptance workflow](../.github/workflows/frontend.yml) for test-server and real-backend settings. Real-backend tests and the Jaeger check require their services to be running. Use a dedicated test database because acceptance checks create and mutate records.

Generated coverage, screenshots, performance captures, browser reports, build output, and installed dependencies are ignored. Reinstall dependencies with `npm ci` after a clean checkout.

## Troubleshooting

- **401 on login:** check the administrator password for the running server, restart after environment changes, and clear stale session cookies if the session secret changed.
- **403 on mutations:** verify `APP_ORIGIN` matches the browser URL, including port.
- **Backend unavailable:** check service readiness, API key alignment, and server-side endpoint URLs. Container endpoints use service hostnames; native development normally uses localhost.
- **Directory listing at port 3001:** stop the unrelated static file server and run the Next.js development server or the web container.
- **Jaeger placeholder page:** follow the [Jaeger repair instructions](../Backend/docs/RUNBOOK.md#jaeger-ui).

Deployment constraints and authentication limitations are described in the [root README](../README.md#deployment).
