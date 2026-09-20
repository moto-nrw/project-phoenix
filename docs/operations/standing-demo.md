# Standing demo school

The `demo` command is the shared-school fallback for #3461. It seeds the
`vollbetrieb` profile once under the slug `messe-demo`, stores its credentials
and entity IDs in `platform.demo_school_states`, and runs the existing live
simulation every 5–8 seconds. It does not write `.seed-state.json`.

The school has NFC off, web attendance on, detailed presence and a 23:59 daily
close. Physical devices remain available to the simulator, while the existing
NFC guards hide device and RFID screens from school admins.

## Local native dev loop

1. Prepare the normal worktree configuration with `scripts/setup-dev.sh`.
   Use separate worktree ports and a separate Compose project/database.
2. Start the application with `phx-dev up`. On macOS with the Nix toolchain,
   use `CGO_ENABLED=0 phx-dev up` if the native linker cannot find `libresolv`.
3. In another terminal, run `CGO_ENABLED=0 phx-dev backend go run . demo
   --url http://localhost:<SERVER_HOST_PORT>`, replacing the port with the
   worktree's `.env` value. Keep the command on one line.
4. Open `http://messe-demo.localhost:<FRONTEND_HOST_PORT>`. Retrieve a school
   admin's credentials from `seed_state.profiles.vollbetrieb.credentials` in
   the private local database. Do not paste the JSON into logs or tickets.

The first run needs `OPERATOR_EMAIL`, `OPERATOR_PASSWORD` and `OGS_DEVICE_PIN`
from the normal local environment. Staff/admin passwords are generated for
this school. A restart reads the existing state and does not seed another
school. Ctrl+C stops the runner; it does not delete the school.

For a finite smoke check, append `--once`: the command provisions or reloads
the school, executes one tick and exits. The CI seed smoke runs it twice before
the table-coverage ratchet, exercising both creation and restart.

## Deployed sidecar

`demo-runtime` is a service of `environments/demo.compose.yml`, so every demo
deploy ships and starts it; nothing is started by hand on the host. Staging and
production stacks do not contain the service, and the release scripts name it
only where the current Compose file lists it.

- **Deploy:** `scripts/deploy-remote.sh` pins `demo-runtime.image` to the same
  immutable backend revision as `server`, stops the sidecar with server and
  frontend before the snapshot, and recreates it after server and frontend are
  healthy. Only server and frontend gate the release: the sidecar starts
  without `--wait`, and a sidecar that cannot start produces a deploy-log
  warning, never a rollback of a healthy application. It is always recreated
  because it lives in the server container's network namespace.
- **Rollback:** the snapshot records the sidecar's image digest and Compose
  entry. Automatic and manual rollback stop it before the restore and start it
  from the restored file. A snapshot older than the sidecar restores a stack
  without it.
- **Failure visibility:** the runner rewrites `/tmp/demo-heartbeat` (tmpfs)
  after every successful tick. The healthcheck marks the container `unhealthy`
  when the file is older than two minutes, so failing ticks and a hung process
  show up in `docker compose ps` instead of a silently static demo. A crashed
  process is restarted (`restart: unless-stopped`). Logs go to journald:
  `journalctl CONTAINER_TAG=demo-runtime`. The first start seeds the school,
  which the 15-minute start period covers. Every restart begins a new start
  period, so a crash loop shows as `restarting` with a growing restart count
  and its error in the log, not as `unhealthy`. Nothing pushes these states to
  a person yet; check `docker compose ps` when the demo looks static.

The `:demo` tag is the environment tag, not a separate release stream. The
sidecar shares the server's network, uses only `http://server:8080`, and
publishes no ports. No additional SOPS keys are introduced.

The sidecar receives only its explicit environment allowlist: environment and
timezone, maintenance DB DSN and pool settings, operator bootstrap credentials,
and the device PIN. It receives no JWT signing key, application-role password,
SMTP credentials, frontend secrets or mounted dotenv file. Unlike the serving
backend, it requires a privileged connection for its private state table.
HTTP database roles cannot read that table.

## Recovery and behavior

- A database advisory lock permits only one runner for `messe-demo`. A second
  runner fails before seeding or simulating. The connection is monitored;
  losing it cancels the runner.
- Each tick reads room visits and school attendance in a tenant-scoped
  transaction. A child whose latest arrival used the protected virtual web
  device or whose latest departure was booked by staff without a kiosk is
  left alone for 15 minutes. Automatic closing does not restart that period.
  Failed reads never trigger a rebuild.
- When no visits remain, the runner restores sessions and attendance without
  consulting the weekday timetable. It keeps existing sessions alive and
  recreates those ended by daily close. School schedules themselves are not
  shifted to weekends.
- The initial occupancy is capped at 84 children to respect the seed profile's
  room capacities. All eligible children receive simulator RFID identities,
  including those available to subsequent attendance ticks.
- An interrupted initial seed can leave a partially provisioned school. The
  fixed slug prevents a second school from being silently created. Investigate
  the seed error before rebuilding the **isolated synthetic demo database**;
  never reset a database containing real schools.

## Verification

`scripts/run-go-toolchain.sh go -C backend test ./simulate
./modules/organizationtenancy/compose ./cmd ./seed/api` covers grace-period
boundaries, weekend rebuilds, stored-state restart, competing runners, HTTP-role
credential isolation and environment/host guards. Keep this command on one line.

Local acceptance also checks live SSE updates, web check-in/check-out, restart
without a second school, recovery after daily close, and the admin's hidden
device/RFID screens. Requests to deployed moto domains remain human-only.
