# Repository Guidelines

## Project Structure & Module Organization
`api/` contains the Go service. Entry point is `api/cmd/server/main.go`; core packages live under `api/internal/` (`config`, `gtfs`, `handlers`, `metrolinx`, `models`). Keep backend tests next to the code as `*_test.go`.

`web/` contains the SvelteKit app. Routes live in `web/src/routes`, shared UI in `web/src/lib/components`, helpers in `web/src/lib`, and static assets in `web/static`. Repo-level docs live in `docs/` and the top-level `README.md`.

## Build, Test, and Development Commands
Backend:

```bash
cd api
go run ./cmd/server/
go test ./... -v
go test ./... -race
go vet ./...
```

Frontend:

```bash
cd web
npm install
npm run dev
npm run dev:tunnel
npm run check
npm run lint
npm run build
```

Use `METROLINX_API_KEY` for the API and `API_BASE_URL=http://localhost:8080` for the web app. The API also loads `.env` and `.env.local` from the repo root or `api/`.

## Coding Style & Naming Conventions
Go code should stay `gofmt`-clean and package-focused; prefer small internal packages over cross-package leakage. Svelte/TypeScript formatting is enforced by Prettier and ESLint in `web/`. Prettier uses tabs, single quotes, no trailing commas, and `printWidth: 100`.

Use PascalCase for Svelte components (`SplitFlapBoard.svelte`), SvelteKit route conventions (`+page.svelte`, `+page.server.ts`), and descriptive lower-case Go package names.

## Testing Guidelines
Backend tests use Go’s `testing` package and already cover handlers, GTFS parsing, config loading, and models. Add or update tests for every behavior change; prefer table-driven tests where they clarify edge cases.

The frontend currently relies on `npm run check`, `npm run lint`, and `npm run build` in CI rather than a dedicated test runner. Treat those as the minimum gate for web changes.

## Commit & Pull Request Guidelines
Recent history follows Conventional Commit style with optional scopes, for example `fix(web): ...`, `style(web): ...`, and `feat: ...`. Keep commits focused by service when possible.

PRs should include a short summary, affected area (`api` or `web`), any env/config changes, and screenshots for visible UI changes. Before opening a PR, run the same checks GitHub Actions runs for the service you touched.
