<div align="center">

# Project Phoenix

![moto Logo](frontend/public/images/moto_transparent.png)

**Digitale Ganztagsbetreuung, die begeistert.**

NFC-based student attendance and room management for schools.

[![GitHub Stars](https://img.shields.io/github/stars/moto-nrw/project-phoenix?style=flat-square)](https://github.com/moto-nrw/project-phoenix/stargazers)
[![GitHub Issues](https://img.shields.io/github/issues/moto-nrw/project-phoenix?style=flat-square)](https://github.com/moto-nrw/project-phoenix/issues)
[![GitHub Pull Requests](https://img.shields.io/github/issues-pr/moto-nrw/project-phoenix?style=flat-square)](https://github.com/moto-nrw/project-phoenix/pulls)
[![License](https://img.shields.io/badge/License-Source--Available-blue?style=flat-square)](LICENSE)
[![GDPR](https://img.shields.io/badge/GDPR-Compliant-success?style=flat-square)](SECURITY.md#gdpr-compliance)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen?style=flat-square)](CONTRIBUTING.md)
[![Contributor Covenant](https://img.shields.io/badge/Contributor%20Covenant-3.0-4baaaa?style=flat-square)](CODE_OF_CONDUCT.md)

[![Go](https://img.shields.io/badge/Go-1.27.0-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![Next.js](https://img.shields.io/badge/Next.js-16-black?style=flat-square&logo=next.js)](https://nextjs.org)
[![React](https://img.shields.io/badge/React-19-61DAFB?style=flat-square&logo=react&logoColor=black)](https://react.dev)
[![TypeScript](https://img.shields.io/badge/TypeScript-6-3178C6?style=flat-square&logo=typescript&logoColor=white)](https://www.typescriptlang.org)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-17-4169E1?style=flat-square&logo=postgresql&logoColor=white)](https://www.postgresql.org)
[![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?style=flat-square&logo=docker&logoColor=white)](https://www.docker.com)

[Features](#features) •
[Quick Start](#quick-start) •
[Architecture](#architecture) •
[Contributing](#contributing) •
[License](#license)

</div>

---

## About

Project Phoenix is the software behind [**moto**](https://moto-ogs.de), the digital platform for all-day childcare (OGS & Hort) at German schools. Children check in and out with NFC wristbands, staff see who is where in real time, and paper lists disappear. Built hand in hand with a real OGS, backed by research at the Universität Münster, and designed from day one around German data protection law.

**Why schools pick it:**

- **Privacy first**: GDPR by design, with configurable retention, audit logging, right to erasure, role-scoped access to student data, and hosting in Germany
- **Real-time**: live attendance, room occupancy, and movement updates pushed to supervisors
- **Built for the Ganztag**: supports the workflows of real after-school care, from check-in to pickup rules to parent communication
- **Self-hostable**: your data on your infrastructure, deployed with Docker Compose

## Features

| | |
|---|---|
| **NFC check-in/out** | Kids scan themselves in and out at tablet kiosks |
| **Rooms & groups** | Occupancy, supervision, combined groups, activities with schedules |
| **Parents portal** | Cross-school guardian accounts with multi-language UI (de/en/ru/sq) |
| **Online enrollment** | Public, configurable enrollment forms with phases, care offerings, and consent blocks |
| **Staff time tracking** | Work sessions, balances, and vacation workflows (ArbZG-aware) |
| **Per-school settings** | Tenant admins configure behavior at runtime, no redeploys |
| **Built-in manual** | German in-app guides with printable PDFs, generated in CI |
| **Multi-tenant** | Many Träger and schools on one deployment, isolated by PostgreSQL Row-Level Security, each school on its own subdomain with a separate operator portal |

## Hardware

The NFC scanning happens on Raspberry Pi kiosks running [**PyrePortal**](https://github.com/moto-nrw/PyrePortal) (Tauri + React), deployed via [**moto-balenaOS**](https://github.com/moto-nrw/moto-balenaOS). The kiosks talk to this backend through the device-authenticated `/api/iot/*` API.

## Quick Start

**Prerequisites:** [Docker](https://docs.docker.com/get-docker/), [Devbox](https://www.jetify.com/devbox/docs/installing_devbox/), and [direnv](https://direnv.net/docs/installation.html). Devbox pins every tool version, so the whole team gets an identical environment.

```bash
git clone https://github.com/moto-nrw/project-phoenix.git
cd project-phoenix

direnv allow              # one-time: activates the devbox environment
devbox run bootstrap      # installs development tools and frontend dependencies
./scripts/setup-dev.sh    # creates configs, SSL certs, and your operator credentials
docker compose up -d      # starts everything; migrations run automatically

# seed demo data (the setup script prints this command with your credentials)
docker compose run server go run . seed --email <op-email> --password '<pw>' --pin 1234 --url http://server:8080

# create attendance and room data for the statistics demo
docker compose run server go run . simulate full-day --close
```

Then log in at **http://localhost:3000** (staff account from the seeder output) or **http://operator.localhost:3000** (your operator credentials).

Full walkthrough, common commands, and troubleshooting: [docs/getting-started.md](docs/getting-started.md).

## Architecture

| Layer | Technology |
|-------|-----------|
| **Backend** | Go 1.27.0, Chi Router, Bun ORM |
| **Frontend** | Next.js 16, React 19, TypeScript, Tailwind CSS 4 |
| **Database** | PostgreSQL 17 with 15 domain schemas, SSL, Row-Level Security |
| **Auth** | JWT with refresh tokens + MFA, NextAuth.js, three isolated portals (staff / operator / parents) |
| **Deployment** | Docker Compose, SOPS-encrypted environments, GitHub Actions CI/CD |

```
project-phoenix/
├── backend/        # Go API: handlers, services, repositories, models, migrations
├── frontend/       # Next.js app: staff, operator, and parents portals
├── environments/   # SOPS-encrypted staging/production configs
└── docs/           # Documentation
```

API documentation is generated from the code: `docker compose run server go run . gendoc` produces `backend/routes.md` and an OpenAPI spec.

## Security & Privacy

This project handles sensitive student data and treats that responsibility as a feature:

- **SSL/TLS everywhere**: encrypted database connections, least-privilege DB roles
- **GDPR compliance**: configurable data retention, append-only audit logs, right to erasure
- **Tenant isolation**: PostgreSQL Row-Level Security enforced at the database, not just the app
- **Role-based access**: staff only see the student data their role permits

> **Reporting vulnerabilities:** see [SECURITY.md](SECURITY.md) for our security policy and responsible disclosure process.

## Roadmap

- [x] NFC student tracking with real-time updates
- [x] Multi-tenancy (Träger → school isolation via RLS)
- [x] Parents portal with multi-language UI
- [x] Online enrollment
- [x] Staff time tracking & vacation workflows
- [ ] Timetable & scheduled activities (in progress)
- [ ] Träger dashboard with cross-school analytics
- [ ] Push notifications

See the [open issues](https://github.com/moto-nrw/project-phoenix/issues) for everything in flight.

## Contributing

Contributions are welcome.

1. Fork the repository and create a feature branch
2. Make your changes (`pnpm run check` and `scripts/run-go-toolchain.sh scripts/test-backend.sh` must pass)
3. Open a Pull Request against **`development`** (never `main`)

Please read [CONTRIBUTING.md](CONTRIBUTING.md) for the full workflow and conventions.

> **Note:** By contributing, you agree to our [Contributor License Agreement](CLA.md).

## License

Distributed under a Source-Available License. See [LICENSE](LICENSE) for more information.

## Contact

- **Website:** [moto-ogs.de](https://moto-ogs.de)
- **Issues:** [Report a bug or request a feature](https://github.com/moto-nrw/project-phoenix/issues)

---

<div align="center">

Made with ❤️ in Münster, Germany by [moto](https://moto-ogs.de)

[⬆ Back to top](#project-phoenix)

</div>
