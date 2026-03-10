# Deprecated: Monolithic API

This monolithic Go API has been replaced by the microservices in `services/`.

The monolith is kept temporarily for parallel running during the transition period.
Once the microservices are verified in production, this directory can be removed.

## New Services

| Service | Directory | Description |
|---------|-----------|-------------|
| shared | `services/shared/` | Shared Go module (models, NATS/Redis helpers, Metrolinx client) |
| gtfs-static | `services/gtfs-static/` | GTFS ZIP loader, schedule queries via HTTP |
| realtime-poller | `services/realtime-poller/` | Unified poller for all Metrolinx feeds → Redis + NATS |
| departures-api | `services/departures-api/` | Departure queries, NextService, fares, alerts, network health |
| api-gateway | `services/api-gateway/` | Thin routing layer, CORS, health aggregation |
| sse-push | `services/sse-push/` | NATS → SSE streams to browsers |
