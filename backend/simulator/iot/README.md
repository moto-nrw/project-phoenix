# IoT Simulator Quickstart

This simulator fakes device traffic (check-ins, attendance, supervisor updates) against the Phoenix backend. Use it to populate dashboards and SSE streams with realistic activity.

## Prerequisites

- Docker & Docker Compose
- `yq` v4 available locally (for the API-key sync script)

## Workflow Overview

0. **(First time)** copy the template config
   ```bash
   cp backend/simulator/iot/simulator.example.yaml backend/simulator/iot/simulator.yaml
   ```

1. **Build images** (first run or after Dockerfile changes)
   ```bash
   docker compose build
   ```

2. **Start Postgres** (daemon mode keeps the DB running for commands)
   ```bash
   docker compose up -d
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

## Engine Capabilities

- Weighted event loop (5s default) that randomly mixes actions per configured weights.
- Student check-ins/checkouts with rotation phases (Heimatraum → AG → Schulhof) and visit cooldowns.
- Schulhof hops that move students outside and back in, including handling stale visit cleanup.
- Attendance toggles once home-room supervisors are present.
- Supervisor swaps that rotate non-lead staff assignments in active sessions.

## Tips

- If you ran the simulator previously, make sure no stale open visits remain before reseeding. Either run the seed against a fresh volume (`docker compose down -v` before step 2) or close them manually:
  ```sql
  UPDATE active.visits
  SET exit_time = NOW()
  WHERE exit_time IS NULL;
  ```
- Rerun steps 4–6 whenever you need fresh data (the seed resets tables, so pull again before simulating).
- If you change `simulator.yaml` (e.g. add devices), re-run step 5 so the API keys stay in sync with the DB.
- Logs show weighted action mix (`tick summary`) and any API failures; tail them to verify the traffic you expect.

## simulator.yaml Structure

```yaml
base_url: http://server:8080
refresh_interval: 30s

event:
  interval: 5s
  max_events_per_tick: 3
  rotation:
    order: [heimatraum, ag, schulhof, heimatraum]
    min_ag_hops: 1
    max_ag_hops: 2
  actions:
    - type: checkin
      weight: 1.0
    - type: checkout
      weight: 0.8
    - type: schulhof_hop
      weight: 0.4
      device_ids: [RFID-LIB-001, RFID-OGS-001]
    - type: attendance_toggle
      weight: 0.4
      device_ids: [RFID-LIB-001, TEMP-CLASS-001]
    - type: supervisor_swap
      weight: 0.3

devices:
  - device_id: RFID-LIB-001
    api_key: <updated via script>
    teacher_ids: [1, 2, 5]
    default_session:
      activity_id: 13
      room_id: 10
      supervisor_ids: [1, 5]
  - device_id: RFID-MAIN-001
    api_key: <updated via script>
    teacher_ids: [1, 2]
    default_session:
      activity_id: 14
      room_id: 9
      supervisor_ids: [1, 2]
  - device_id: RFID-MENSA-001
    api_key: <updated via script>
    teacher_ids: [1, 2]
    default_session:
      activity_id: 16
      room_id: 22
      supervisor_ids: [2, 3]
  - device_id: RFID-OGS-001
    api_key: <updated via script>
    teacher_ids: [3]
    default_session:
      activity_id: 15
      room_id: 21
      supervisor_ids: [3]
  - device_id: RFID-SPORT-001
    api_key: <updated via script>
    teacher_ids: [4]
    default_session:
      activity_id: 1
      room_id: 19
      supervisor_ids: [4]
  - device_id: TEMP-CLASS-001
    api_key: <updated via script>
    default_session:
      activity_id: 12
      room_id: 11
      supervisor_ids: [5, 6]
  - device_id: TEMP-MENSA-001
    api_key: <updated via script>
    default_session:
      activity_id: 17
      room_id: 23
      supervisor_ids: [3, 7]
```
