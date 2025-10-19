# IoT Simulator Quickstart

This simulator fakes device traffic (check-ins, attendance, supervisor updates) against the Phoenix backend. Use it to populate dashboards and SSE streams with realistic activity.

## Prerequisites

- Docker & Docker Compose
- `yq` v4 available locally (for the API-key sync script)

## Workflow Overview

1. **Build images** (first run or after Dockerfile changes)
   ```bash
   docker compose build
   ```

2. **Start Postgres** (daemon mode keeps the DB running for commands)
   ```bash
   docker compose up -d postgres
   ```

3. **Apply migrations**
   ```bash
   docker compose exec server ./main migrate
   ```

4. **Seed runtime data** (generates active sessions, visits, etc.)
   ```bash
   docker compose exec server ./main seed
   ```

5. **Sync simulator API keys** (grab latest keys from `iot.devices` and patch `simulator.yaml`)
   ```bash
   ./scripts/update-simulator-keys.sh
   ```

6. **Run the simulator**
   ```bash
   docker compose run --rm \
     -e SIMULATOR_CONFIG=/app/simulator.yaml \
     -v "$PWD/backend/simulator/iot/simulator.yaml:/app/simulator.yaml:ro" \
     server ./main simulate
   ```

The simulator authenticates each configured device, keeps state in sync, and emits traffic on the configured interval.

## Tips

- Rerun steps 4–6 whenever you need fresh data (the seed resets tables, so pull again before simulating).
- If you change `simulator.yaml` (e.g. add devices), re-run step 5 so the API keys stay in sync with the DB.
- Logs show weighted action mix (`tick summary`) and any API failures; tail them to verify the traffic you expect.
