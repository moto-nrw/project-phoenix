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
frontend and browser dependencies. This only happens once. The Devbox lock
supports Apple Silicon macOS and Linux on arm64 or amd64; Intel macOS is not
supported by the current Nixpkgs package set.

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
docker compose up -d
```

Wait for all services to start. First boot takes a minute because the backend runs all database migrations automatically.

## Seeding Test Data

The setup script prints the exact seed command with your credentials. It looks like this:

```bash
docker compose run server go run . seed \
  --email operator@example.com \
  --password 'YOUR_PASSWORD' \
  --pin 1234 \
  --url http://server:8080
```

The server container must be running (`docker compose up -d`) before you seed.

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

The same organization also contains `Demo-Schule Manuell` / `manuell`. Its
school-admin login is `manuell-admin@example.test` / `Manuell1234%`. This
profile uses binary presence, web attendance without NFC terminals, open care,
open rooms, disabled enrollment, and weekly-plan-driven care. It contains 12
children with contacts and weekly plans. Four children are present and four
have already checked out; room-visit history stays empty. The account with the
semantic key `entwickler-admin` can switch between `vollbetrieb` and `manuell`.
Each school's dedicated school-admin account remains isolated to that school.

The simulator uses `vollbetrieb` without a flag. Select it explicitly with:

```bash
docker compose run server go run . simulate full-day --profile vollbetrieb
```

Inspect the manual profile without starting an IoT simulation:

```bash
docker compose run server go run . simulate status --profile manuell
```

Reset the development database before recreating this deterministic profile:

```bash
docker compose run server go run . migrate reset
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
| Start all services | `docker compose up -d` |
| Stop all services | `docker compose down` |
| View logs | `docker compose logs -f server` |
| Rebuild backend after Go changes | `docker compose build server && docker compose up -d server` |
| Reset database | `docker compose run server go run . migrate reset` |
| Run backend tests | `scripts/run-go-toolchain.sh scripts/test-backend.sh` |
| Run frontend checks | `cd frontend && pnpm run check` |

## Troubleshooting

**"account is inactive" on login** -- The legacy admin account (`admin@example.com`) is disabled by design. Use a staff account from the seeder output or the operator dashboard.

**SMTP errors in logs** -- Normal for local development. Email features (invitations, password reset) require SMTP configuration, which is not needed for development.

**"no matching decryption secret"** -- Clear your browser cookies for localhost:3000 and try again. This happens when the NEXTAUTH_SECRET changes between sessions.
