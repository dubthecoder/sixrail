# Six Rail — Documentation

## Plans

Historical design and implementation documents from the project's development.

| Document | Date | Status | Description |
|---|---|---|---|
| [GoPulse Design](plans/2026-03-03-gopulse-design.md) | 2026-03-03 | Completed (superseded) | Original GoPulse architecture — multi-page GO Transit site with departures, map, alerts, schedules, journey planner, and fares |
| [GoPulse Implementation](plans/2026-03-03-gopulse-implementation.md) | 2026-03-03 | Completed (superseded) | Step-by-step build plan for GoPulse: Go API, GTFS parsing, SvelteKit frontend, Railway deploy |
| [Six Rail Migration Design](plans/2026-03-03-sixrail-migration-design.md) | 2026-03-03 | Completed | Migration from proprietary Metrolinx API proxy to hybrid GTFS architecture, renamed to Six Rail |
| [Six Rail Migration Implementation](plans/2026-03-03-sixrail-implementation.md) | 2026-03-03 | Completed | Task-by-task migration: new models, GTFS static/realtime parsers, simplified frontend |
| [Simulated Positions Design](plans/2026-03-03-simulated-positions-design.md) | 2026-03-03 | Completed | Synthetic vehicle positions from GTFS static schedule when API key is unavailable |
| [Simulated Positions Implementation](plans/2026-03-03-simulated-positions.md) | 2026-03-03 | Completed | Implementation: trip indexing, position interpolation, poller toggle in main.go |
| [Filter Panel Design](plans/2026-03-04-filter-panel-design.md) | 2026-03-04 | Completed | Floating filter chips for train/bus type, route, and status filtering |
| [Filter Panel Implementation](plans/2026-03-04-filter-panel.md) | 2026-03-04 | Completed | Implementation: routeType field, filter store, FilterChips component, map integration |

## Project Timeline

1. **GoPulse** — Initial build as a multi-page GO Transit tracking site with Metrolinx proprietary API proxy
2. **Six Rail migration** — Renamed to Six Rail, switched to hybrid GTFS Static + GTFS-RT architecture, simplified to single fullscreen map
3. **Simulated positions** — Fallback vehicle simulation from static schedule when API key is pending
4. **Filter panel** — Client-side filtering by transport type, route, and status (later redesigned as always-visible train/bus toggles)
