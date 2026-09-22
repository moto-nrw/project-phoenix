# Demo process: one demo school per demo access

The `demo` command keeps the demo schools of the public demo alive. Since
#3463 every demo access gets a demo school of its own; the one standing demo
school of #3461 stays selectable as the fallback (see below).

## One demo school per demo access (default)

1. `POST /demo/access-requests` stores the demo access and queues an order in
   `platform.demo_school_states` (`status = preparing`). The slug is the OGS
   name as a DNS label plus six random characters. The link it mails (#3465)
   points at the waiting room `FRONTEND_URL/demo`, because `https://<slug>.<TENANT_DOMAIN>`
   answers only once the school exists; the waiting room sends the visitor
   there when the status turns `ready`. The serving backend may
   only insert an order, read `name`, `status`, `tenant_id` and
   `visitor_account_id`, and stamp `last_used_at`; `seed_state` with its credentials and `status` stay
   out of its reach (column grants, `phoenix_admin` bypasses row security).
2. The demo process claims orders oldest first. At most **3** seeds run at a
   time (`demoSeedWorkers`); further orders wait. The seed workers share one
   operator login, because every operator login drives the second factor.
3. A seed names the school after the OGS and renames one caregiver with a
   group and shifts (seed person Julia Klein) and one parent with a child in
   that group (Sabine Schneider) to the prospect. Only the name is taken: the
   prospect's address never reaches the queue, so it cannot become an account
   or guardian address. Until the demo roles exist the visitor's caregiver has
   the administrator role, and the demo access signs in as that account.
4. After the seed the process runs the school's first tick, which rebuilds
   rooms and attendance at any hour and on any weekday, and only then sets
   `status = ready`. It names the visitor's caregiver (`visitor_account_id`)
   and the visitor's parent (`visitor_parent_account_id`, #3468): the
   renamed Sabine Schneider, a primary guardian with full parents portal
   rights for exactly one child.
5. Only schools in use get ticks (#3464). Redeeming the token stamps
   `last_used_at` on the school's row (the serving role may write that one
   column), and the process keeps one ticker per ready school entered in the
   last 30 minutes (`demoInUseWindow`). A new ticker ticks at once, so an
   entered school moves within about a second; one that falls out of the
   window stops. A returning visitor redeems the link again and the
   simulation resumes; the first tick rebuilds attendance when the daily close
   ended it. Tokens redeemed only once cover a single visit: a visitor who
   stays longer than the window without entering again sees a still school.
6. A failed order is repeated once. The repetition renames and soft-deletes
   the school the broken attempt left under the slug (schools are never
   hard-deleted and keep their unique subdomain) and seeds with a fresh
   account scope. Account emails and usernames carry the slug's random
   suffix (`demo11.k3m9xp@mail.de`), the repetition `k3m9xp-2`: usernames end
   at 30 characters, and the abandoned school keeps its accounts. A seed that
   fails because the operator cannot sign in (the second factor allows three
   codes in 15 minutes) is not counted: the order returns to the queue and
   the process waits a minute before it tries to sign in again. After the second failure the order is `failed`;
   the entry page tells the prospect to request a new link, and that address
   no longer counts as having an active demo access.
7. A restart releases the claims of the stopped process. An order whose seed
   state was already stored is not seeded again; it only gets its first tick.
8. „Demo neu anfangen" (#3470, `POST /demo/access/reset`) soft-deletes the
   visitor's school, revokes its sessions and queues a new order with the
   same OGS and visitor names; every access of the old school now enters the
   new one. The old order keeps its row and its `ready` status, but the
   process skips it: `ReadyDemoSchools` and `ActiveDemoSchools` join
   `platform.schools` and leave hidden schools out, so a running ticker
   stops at the next poll. The standing school cannot be restarted.

An address whose newest unexpired demo access enters a school that did not
fail gets no second school: after the 10-minute cooldown of #3465 a further
request stores an access into that same school and mails its link.

`--once` runs the expiry, empties the queue, ticks every ready school once, in use or not, and exits.

### Expiry

A demo access ends 14 days after its last use (#3470): every redemption moves
`expires_at` forward by the full lifetime. Once an hour (`demoExpiryInterval`,
first run at the first poll) the process deletes every access past its end
and soft-deletes the demo schools no remaining access enters; the standing
school is never hidden. A hidden school holds no place against the capacity,
gets no ticker and cannot be entered. A school that could not be hidden
(database away) is remembered and hidden at the next poll. The process's role
sees only `id`, `school_slug` and `expires_at` of `auth.demo_accesses`, never
an address or a name, and may set `deleted_at` on `platform.schools`
(migration 1.15.412). Hidden schools accumulate; the ADR allows rebuilding
the demo database when that matters.

### Capacity

The server queues at most `--demo-max-active-schools` demo schools at once
(#3466): queued schools and ready ones whose school is not deleted count, a
failed one does not. Beyond that a new address gets `503
demo_capacity_reached`; an address with a school still gets its link. The
value sits on the `server` command in `environments/demo.compose.yml` (300);
check it against the host size before a fair and change it there with a
deploy. `serve` refuses to start under `APP_ENV=demo` without it; locally
pass the flag to `serve`.

## Fallback: the standing demo school

Append `--demo-standing-school` to the `command` of **both** `server` and
`demo-runtime` in `environments/demo.compose.yml` and deploy.
`scripts/check-runtime-env.py` rejects a stack where only one of them carries
the flag. Locally the same flag goes on `serve` and `demo`
(or `DEMO_STANDING_SCHOOL=true` for both processes). Then every demo access
enters `messe-demo` as its administrator, nothing is queued, and the process
behaves as described in the rest of this document. Demo accesses issued in
one mode keep working in the other: each one remembers its school.

The standing school seeds the `vollbetrieb` profile once under the slug
`messe-demo`, stores its credentials and entity IDs in
`platform.demo_school_states`, and runs the existing live simulation every
5–8 seconds. It does not write `.seed-state.json`.

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
- Parent ticks (#3468): every 3 minutes another parent of the school asks
  for a pickup change on a school day from the day after tomorrow on, or
  writes a message about the own child, alternating, so „Offene Anfragen" in
  the OGS app is never empty. One parent acts for 10 minutes on one login,
  then the next takes over. The visitor's parent and every parent who shares
  its child stay out: their messages would appear in the visitor's own
  parents app. A request the school refuses (4xx) is skipped; a failing
  parent never stops the children's ticks of the same round.
- When no visits remain, the runner restores sessions and attendance without
  consulting the weekday timetable. It keeps existing sessions alive and
  recreates those ended by daily close. School schedules themselves are not
  shifted to weekends.
- Weekend day plan (#3471): the device sessions the runner starts are
  mirrored into the timetable as spontaneous blocks (title of the activity,
  window from the session start plus 60 minutes, status running), so home
  page, „Mein Tag" and the day plan show eight running blocks on a Saturday
  as well. That is all the simulation can add on a weekend: the web
  spontaneous start and instance planning reject Saturday and Sunday on the
  server, and a device session can neither be back-dated nor planned ahead.
  Upcoming blocks, pickup and arrival times and the week view stay empty.
  Rotating the sessions to fill the day was tried and rejected: it turns the
  day plan into a log of the same eight activities and changes nothing for a
  per-access school, which only ticks during the visit. Screenshots and the
  proposed follow-up are in the issue.
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
