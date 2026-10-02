# Getting Started

## Prerequisites

Install these before cloning:

1. **direnv** -- https://direnv.net/docs/installation.html
2. **devbox** -- https://www.jetpack.io/devbox/docs/installing_devbox/
3. **Docker Desktop** -- https://docs.docker.com/get-docker/

## Setup

```bash
git clone git@github.com:moto-nrw/project-phoenix.git
cd project-phoenix
direnv allow
devbox run bootstrap
```

Wait for Devbox to install the pinned tools, then let bootstrap install the
frontend and browser dependencies. Each worktree gets its own frontend install;
browser tooling is installed only when missing. The Devbox lock
supports Apple Silicon macOS and Linux on arm64 or amd64; Intel macOS is not
supported by the current Nixpkgs package set.

### Worktree setup

`wt add <reference>` runs bootstrap by default, leaving frontend tools ready to
use. For backend or documentation work that does not need host-side frontend
tools, use `wt add --no-bootstrap <reference>`. Run `devbox run bootstrap` from
that worktree when you need the frontend later. Skipping bootstrap does not
disable the other worktree setup steps, including Compose configuration.

### Dependency audits

Run `devbox run -- pnpm --dir frontend exec knip --include dependencies,unlisted`
to check dependencies without starting services or seeding a database. Knip's
Playwright plugin is disabled because it executes performance configurations
that require test credentials. The `entry` list covers those configurations,
E2E tests, performance tests, and the guide PDF generator as source files;
keep it aligned when adding Playwright entry points.

Tests use Node and happy-dom. jsdom is neither a direct dependency nor an
installed optional dependency; the pnpm workspace excludes Vitest's unused
optional jsdom peer.

Both Lucide and Phosphor remain in use. Next.js optimizes Lucide imports by
default, and the frontend config enables Phosphor import optimization. These
optimizations affect compiled modules, not the contents of installed npm
packages. Do not delete unused icon files from `node_modules`.

### Local services

Then run the setup script:

```bash
./scripts/setup-dev.sh
```

The script will:
- Ask for your operator email and password (or generate random ones)
- Create all config files (.env, docker-compose.yml, backend/dev.env, frontend/.env.local)
- Generate SSL certificates for the local database
- Print your credentials at the end

**Save the credentials from the output. They won't be shown again.**

## Starting the App

```bash
scripts/dev-native.sh up
```

This starts only `postgres` and `mailpit` in Docker and runs the backend
(air) and frontend (`next dev`) directly on your machine. It migrates the
database before the backend starts; the first boot takes a minute. Stop with
Ctrl+C. Details: [operations](agents/operations.md#native-dev-loop).

Running the backend and frontend inside Docker as well
(`docker compose --profile full up -d`) still works, but on 16 GB laptops the
Docker VM then competes with the IDE for memory; see
[low-memory machines](development-environment.md#low-memory-machines).

## Seeding Test Data

The setup script prints the exact seed command with your credentials. It looks like this:

```bash
scripts/dev-native.sh backend go run . seed \
  --email operator@example.com \
  --password 'YOUR_PASSWORD' \
  --pin 1234 \
  --url http://localhost:8080
```

The backend must be running (`scripts/dev-native.sh up`) before you seed. In a `wt` worktree use its `SERVER_HOST_PORT` from `.env` instead of 8080.

After seeding, you get 20 staff accounts, 100 students, rooms, groups, and activities.
The default `vollbetrieb` profile has stable credentials and fails with a clear
conflict if it already exists.

### Demo school profile contract

The seeder writes `backend/.seed-state.json`. Contract version 3 stores a
`profiles` map and declares `default_profile: "vollbetrieb"`. Seeder,
simulator, performance tests, and browser tests read this profile contract;
unknown versions are rejected. Each profile records its organization, school,
settings and owner, role-based credentials, physical and virtual devices,
semantic entity maps, expected counts, and scenario metadata.

The stable bootstrap identity is `Demo-Träger Nord` / `demo-traeger-nord` and
`Demo-Schule Vollbetrieb` / `vollbetrieb`. Its school-admin login is
`vollbetrieb-admin@example.test` / `Vollbetrieb1234%`. The profile explicitly
enables detailed presence, NFC and web attendance, fixed groups, a fixed care
schedule, online enrollment, and weekly-plan-driven care. The seeder reads
these settings and the expected children, devices, and planned activities back
through the API before it writes the state file.

The same command also creates `anmeldung-wochenplan` as the first school of
`Demo-Träger Süd` / `demo-traeger-sued`. Its isolated school-admin login is
`anmeldung-wochenplan-admin@example.test` / `Wochenplan1234%` (the shared
`--staff-password` flag overrides all school-admin passwords). The developer
admin `demo1@mail.de` can switch between the two organizations using the normal
tenant switch. The school-admin accounts remain school-local.

This smaller profile has detailed web attendance, no physical terminals or
NFC attendance, fixed groups, an open-room concept, enrollment and care
offerings enabled, and `enrollment.bookings_authoritative=false`. Twelve
children have contacts, parent accounts, and weekly plans for Monday, Tuesday and
Thursday, 12:00–15:00. Their bookings deliberately cover Monday until 16:00.
The seed verifies each child's parent-facing care days and times against the
weekly plan, despite the Monday booking. Sixteen requests cover approval,
submission through a parent account, waiting list, rejection, and withdrawal.
The active phase, schema, offerings, request keys, and parent credentials are
recorded under `profiles["anmeldung-wochenplan"]` in the version 3 state.
The second school in `demo-traeger-sued` is `anmeldung-buchungen`. Its isolated
school-admin login is `anmeldung-buchungen-admin@example.test` / `Buchungen1234%`.
This profile uses binary web and NFC attendance, open care, and open rooms.
The seed approves bookings for all twelve children before enabling
`enrollment.bookings_authoritative=true`. It then closes online enrollment and
checks that the remaining active bookings still determine Monday care days,
not the old Tuesday and Thursday weekly-plan rows. Pickup times remain in the
weekly plan.

Under `profiles["anmeldung-buchungen"]`, the student and enrollment-request keys
`abschluss-geplant`, `abschluss-faellig`, and `abschluss-erledigt` identify the
three complete-withdrawal examples. The profile includes a physical terminal
and the protected virtual web device. The seed checks NFC check-in/check-out,
web attendance, empty room history, and withdrawal states through the API.
There is no separate withdrawal-demo school or special state field.

The `marketing` profile is the product-screenshot school `OGS Sonnenhang` in
its own organization `Demo-Träger Marketing` / `demo-traeger-marketing`. Its
school-admin login is `marketing-admin@example.test` / `Marketing1234%`; this
account is `accounts.admin[0]` of `profiles["marketing"]`, followed by the
developer admin. The profile uses detailed web attendance without NFC (rooms
record where the children are), fixed groups, and disabled enrollment. Two
groups, two caregivers, four rooms, and twelve children with contacts have
weekly plans for all five weekdays, derived from the 10:15 reference clock
(`marketingReferenceClock`): seven children are present, two were picked up
early, and three are expected after 10:15. The present children sit in two
running blocks (`Bauecke` in the Bauraum, `Fußball` in the Turnhalle,
09:30–11:30). The admin finds an unread team message and an unread parent
message, the team has one notice for the day, and the families see one news
item and a staff reply. For the shot list, two families file requests for the
next weekday (an open pickup change for Elif, an excused absence for Mia), the
meal plan covers the current week, the team calendar has a meeting today, and
the autumn festival of the news item waits for the families' reply in their
calendar. Eight children have a logo-figure picture
(`backend/seed/avatar`) uploaded with parental consent, the other four show
initials; the admin and both caregivers upload theirs while signed in. No two
people share a picture. Presence and the running blocks are recorded through
the web API at seed time. Product screenshots using this profile therefore
require a seed from the capture day (Berlin time), on a weekday. Four parent
accounts (`ParentSeed1234%` unless `--staff-password` is set, one with two
children) are listed under `credentials.parents`.

The normal seed always creates and checks all five profiles through production
HTTP endpoints against the local server. The shared developer admin can switch
between all five schools; each school-admin account stays school-local.

`Demo-Träger Nord` also contains `Demo-Schule Manuell` / `manuell`. Its
school-admin login is `manuell-admin@example.test` / `Manuell1234%`. This
profile uses binary presence, web attendance without NFC terminals, open care,
open rooms, disabled enrollment, and weekly-plan-driven care. It contains 12
children with contacts and weekly plans. Four children are present and four
have already checked out; room-visit history stays empty. The account with the
semantic key `entwickler-admin` can switch between `vollbetrieb` and `manuell`.
Each school's dedicated school-admin account remains isolated to that school.

The simulator uses `vollbetrieb` without a flag. Select it explicitly with:

```bash
scripts/dev-native.sh backend go run . simulate full-day --profile vollbetrieb
```

Inspect the manual profile without starting an IoT simulation:

```bash
scripts/dev-native.sh backend go run . simulate status --profile manuell
```

Reset the development database before recreating this deterministic profile:

```bash
scripts/dev-native.sh backend go run . migrate reset
```

To add one more demo school to a database that is already seeded, give the run
its own slug and restrict it to one profile. Every account of that school then
carries the slug in its domain (`anna.mueller@demo-ogs-nord.moto-ogs.de`), and
the school joins the existing Demo-Träger:

```bash
scripts/dev-native.sh backend go run . seed --email op@example.com --password 'Test1234%' --pin 1234 --url http://localhost:8080 \
  --profile vollbetrieb --tenant-slug ogs-nord --school-name 'OGS Nord' --state ogs-nord.seed-state.json
```

Use the seed command's `--randomize` flag only for an intentionally disposable,
uniquely named school. The Go reader can migrate legacy version 2 state files;
new consumers must use version 3 and select profiles through the shared reader.

## Logging In

| URL | Purpose | Credentials |
|-----|---------|-------------|
| http://localhost:3000 | Tenant (school) app | Any staff account from seeder output (e.g. `demo1@mail.de` / `sdlXK26%`) |
| http://operator.localhost:3000 | Operator dashboard | The operator email/password you chose during setup |

## Common Commands

| Task | Command |
|------|---------|
| Start everything | `scripts/dev-native.sh up` |
| Stop backend/frontend | Ctrl+C or `scripts/dev-native.sh down` |
| Stop infrastructure | `docker compose down` |
| View logs | `tmp/dev-native/backend.log`, `tmp/dev-native/frontend.log` |
| After `go.mod` changes | restart `scripts/dev-native.sh up` (air reloads plain Go edits) |
| Reset database | `scripts/dev-native.sh backend go run . migrate reset` |
| Run backend tests | `scripts/run-go-toolchain.sh scripts/test-backend.sh` |
| Run frontend checks | `cd frontend && pnpm run check` |

## Troubleshooting

**"account is inactive" on login** -- The legacy admin account (`admin@example.com`) is disabled by design. Use a staff account from the seeder output or the operator dashboard.

**SMTP errors in logs** -- Normal for local development. Email features (invitations, password reset) require SMTP configuration, which is not needed for development.

**"no matching decryption secret"** -- Clear your browser cookies for localhost:3000 and try again. This happens when the NEXTAUTH_SECRET changes between sessions.
