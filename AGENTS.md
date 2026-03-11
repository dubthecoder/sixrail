# Repository Guidelines

## Project Structure & Module Organization
`services/` contains all deployable services. Active Go modules in `services/go.work` live in `services/departures-api/`, `services/gtfs-static/`, `services/realtime-poller/`, and `services/sse-push/`, with shared Go code in `services/shared/`. The previous `api-gateway` service has been removed; the SvelteKit web app now proxies backend calls server-side and can talk directly to `departures-api`.

`services/web/` contains the SvelteKit app. Routes live in `services/web/src/routes`, shared UI in `services/web/src/lib/components`, helpers in `services/web/src/lib`, frontend tests in `services/web/src/**/*.test.ts`, and static assets in `services/web/static`. Repo-level docs live in `docs/`, especially `docs/README.md` and `docs/plans/`, plus the top-level `README.md`.

## Build, Test, and Development Commands
Backend:

```bash
cd services
go vet ./...
go test ./... -v -short
```

Match CI for the Go module(s) you touched before opening a PR:

```bash
cd services
go vet ./departures-api/...
go test ./departures-api/... -v -race -short
```

Swap `departures-api` for `gtfs-static`, `realtime-poller`, `shared`, or `sse-push` as needed.

Frontend:

```bash
cd services/web
npm install
npm run dev
npm run test:ci
npm run check
npm run lint
npm run build
```

Use `METROLINX_API_KEY` for backend services. For local web development, set `API_BASE_URL=http://localhost:8082` when pointing directly at `departures-api`; use `http://localhost:8080` only if you have a separate local proxy configured. Additional web env vars live in `services/web/.env.example`, including `PUBLIC_MAPBOX_TOKEN` and the optional HMR tunnel settings.

## Coding Style & Naming Conventions
Go code should stay `gofmt`-clean and package-focused; prefer small internal packages over cross-package leakage. Svelte/TypeScript formatting is enforced by Prettier and ESLint in `services/web/`. Prettier uses tabs, single quotes, no trailing commas, and `printWidth: 100`.

Use PascalCase for Svelte components (`SplitFlapBoard.svelte`), SvelteKit route conventions (`+page.svelte`, `+page.server.ts`), and descriptive lower-case Go package names.

## Testing Guidelines
Backend tests use Go's `testing` package. Add or update tests for every behavior change; prefer table-driven tests where they clarify edge cases.

Frontend tests use Vitest for `services/web/src/**/*.test.ts`. Add or update tests when changing logic-heavy web helpers, stores, or server utilities. Treat `npm run test:ci`, `npm run check`, `npm run lint`, and `npm run build` as the minimum gate for web changes.

## Commit & Pull Request Guidelines
Recent history follows Conventional Commit style with optional scopes, for example `fix(web): ...`, `style(web): ...`, and `feat: ...`. Keep commits focused by service when possible.

PRs should include a short summary, affected area, any env/config changes, and screenshots for visible UI changes. Before opening a PR, run the same checks GitHub Actions runs for each touched service or app: the matching Go module `go vet` and `go test -v -race -short`, or for web `npm run test:ci`, `npm run check`, `npm run lint`, and `npm run build`.
