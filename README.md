<div align="center">

# Project Phoenix

![moto Logo](frontend/public/images/moto_transparent.png)

**A modern RFID-based student attendance and room management system for educational institutions**

[![GitHub Stars](https://img.shields.io/github/stars/moto-nrw/project-phoenix?style=flat-square)](https://github.com/moto-nrw/project-phoenix/stargazers)
[![GitHub Issues](https://img.shields.io/github/issues/moto-nrw/project-phoenix?style=flat-square)](https://github.com/moto-nrw/project-phoenix/issues)
[![GitHub Pull Requests](https://img.shields.io/github/issues-pr/moto-nrw/project-phoenix?style=flat-square)](https://github.com/moto-nrw/project-phoenix/pulls)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue?style=flat-square)](LICENSE)
[![GDPR](https://img.shields.io/badge/GDPR-Compliant-success?style=flat-square)](SECURITY.md#gdpr-compliance)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen?style=flat-square)](CONTRIBUTING.md)
[![Contributor Covenant](https://img.shields.io/badge/Contributor%20Covenant-3.0-4baaaa?style=flat-square)](CODE_OF_CONDUCT.md)

[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![Next.js](https://img.shields.io/badge/Next.js-15-black?style=flat-square&logo=next.js)](https://nextjs.org)
[![React](https://img.shields.io/badge/React-19-61DAFB?style=flat-square&logo=react&logoColor=black)](https://react.dev)
[![TypeScript](https://img.shields.io/badge/TypeScript-5-3178C6?style=flat-square&logo=typescript&logoColor=white)](https://www.typescriptlang.org)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-17-4169E1?style=flat-square&logo=postgresql&logoColor=white)](https://www.postgresql.org)
[![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?style=flat-square&logo=docker&logoColor=white)](https://www.docker.com)

[Features](#-features) •
[Quick Start](#-quick-start) •
[Documentation](#-documentation) •
[Contributing](#-contributing) •
[License](#-license)

</div>

---

## 📖 About

Project Phoenix is a comprehensive room and student management system designed for educational institutions in compliance with European data protection regulations. It leverages RFID technology to track student attendance and location in real-time, providing administrators with powerful tools for monitoring room occupancy, managing activities, and generating detailed analytics.

### Why Project Phoenix?

- **Privacy-First Design** — Built from the ground up with GDPR compliance, featuring configurable data retention, audit logging, and right-to-erasure support
- **Real-Time Visibility** — Know instantly where students are, which rooms are occupied, and how spaces are being utilized
- **Modern Stack** — Go backend with Next.js 15 frontend, designed for performance and developer experience
- **Self-Hosted** — Your data stays on your infrastructure, with full control over security and compliance

---

## ✨ Features

### Core Functionality
- 🏷️ **RFID Student Tracking** — Real-time location tracking using RFID technology
- 🏫 **Room Management** — Monitor room occupancy and usage patterns
- 👥 **Group Management** — Organize students into groups and manage activities
- 👨‍🏫 **Multiple Supervisors** — Assign multiple supervisors to groups and rooms
- 📊 **Analytics Dashboard** — Comprehensive reporting and utilization statistics
- 🗓️ **Schedule Management** — Handle class schedules and time-based activities
- 🎯 **Activity Tracking** — Track student participation in various activities

### Technical Features
- 🔐 **JWT Authentication** — Secure authentication with role-based access control
- ✉️ **Email Workflows** — SMTP-backed invitations with branded templates and rate-limited password reset
- 🚀 **RESTful API** — Well-documented API with OpenAPI specification
- 📱 **Responsive UI** — Modern, mobile-friendly interface
- 🐳 **Docker Support** — Easy deployment with containerization
- 🔄 **Real-time Updates** — Live tracking of student movements and room occupancy
- 🌐 **i18n Ready** — Internationalization support built-in

---

## 🚀 Quick Start

### Prerequisites

- **Docker and Docker Compose** — For running PostgreSQL and optional containerized development
- **Devbox** — Reproducible development environment (installs Go, Node.js, and all CLI tools)
- **direnv** — Automatic environment activation when entering the project directory

> **Why Devbox?** We use Devbox to ensure every developer has identical tool versions. No more "works on my machine" issues — everyone gets the same Go, Node.js, golangci-lint, etc.

### Install Development Tools

<details>
<summary><strong>macOS</strong></summary>

```bash
# Install Devbox
curl -fsSL https://get.jetify.com/devbox | bash

# Install direnv
brew install direnv

# Add to ~/.zshrc (or ~/.bashrc)
eval "$(direnv hook zsh)"
```

</details>

<details>
<summary><strong>Windows (WSL) / Linux</strong></summary>

```bash
# Install Devbox
curl -fsSL https://get.jetify.com/devbox | bash

# Install direnv (Ubuntu/Debian)
sudo apt install direnv

# Add to ~/.bashrc (or ~/.zshrc)
eval "$(direnv hook bash)"
```

</details>

<details>
<summary><strong>Optional: Suppress direnv output</strong></summary>

By default, direnv prints all exported environment variables when entering the project. To silence this output, create a direnv config file:

```bash
mkdir -p ~/.config/direnv
cat > ~/.config/direnv/direnv.toml << 'EOF'
[global]
log_format = "-"
log_filter = "^$"
EOF
```

> **Note:** The `DIRENV_LOG_FORMAT` environment variable no longer works in direnv 2.36.0+ due to a [known regression](https://github.com/direnv/direnv/issues/1418). The TOML config above is the correct solution.

</details>

### One-Command Setup

```bash
# Clone the repository
git clone https://github.com/moto-nrw/project-phoenix.git
cd project-phoenix

# Allow direnv to activate the environment (one-time)
direnv allow

# Run the automated setup script
./scripts/setup-dev.sh

# Start all services
docker compose up -d
```

When you `cd` into the project, direnv automatically activates Devbox and you'll see:
```
phoenix dev ready - go 1.25.5, node 20.20.0
```

All tools (Go, Node, npm, golangci-lint, bruno-cli, etc.) are now available.

The application will be available at:
- **Frontend:** http://localhost:3000
- **Backend API:** http://localhost:8080

### Manual Setup

<details>
<summary>Click to expand manual setup instructions</summary>

1. **Generate SSL certificates** (required for GDPR-compliant database connections):
   ```bash
   cd config/ssl/postgres
   ./create-certs.sh
   cd ../../..
   ```

2. **Configure environment files**:
   ```bash
   cp backend/dev.env.example backend/dev.env
   cp frontend/.env.local.example frontend/.env.local
   # Edit the files with your settings
   ```

3. **Start services**:
   ```bash
   docker compose up -d
   ```

4. **Run database migrations**:
   ```bash
   docker compose run server ./main migrate
   ```

</details>

---

## 🏗️ Architecture

### Tech Stack

| Layer | Technology |
|-------|-----------|
| **Backend** | Go 1.23+, Chi Router, Bun ORM |
| **Frontend** | Next.js 15, React 19, TypeScript 5 |
| **Styling** | Tailwind CSS 4 |
| **Database** | PostgreSQL 17 with SSL encryption |
| **Auth** | JWT with refresh tokens, NextAuth.js |
| **Deployment** | Docker Compose, Caddy (production) |
| **CI/CD** | GitHub Actions |

### Project Structure

```
project-phoenix/
├── backend/                   # Go backend API
│   ├── api/                   # HTTP handlers and routes
│   ├── auth/                  # Authentication logic
│   ├── database/              # Migrations and repositories
│   ├── models/                # Domain models
│   └── services/              # Business logic
├── frontend/                  # Next.js frontend
│   └── src/
│       ├── app/               # Next.js App Router
│       ├── components/        # UI components
│       └── lib/               # Utilities and API clients
├── deployment/                # Production configurations
├── docs/                      # Documentation
└── docker-compose.yml         # Development environment
```

### Database Schema

The database uses PostgreSQL schemas to organize tables by domain:

| Schema | Purpose |
|--------|---------|
| `auth` | Authentication, tokens, permissions |
| `users` | User profiles, students, teachers, staff |
| `education` | Groups and educational structures |
| `facilities` | Rooms and physical locations |
| `activities` | Student activities and enrollments |
| `active` | Real-time session tracking |
| `schedule` | Time and schedule management |
| `iot` | RFID device management |
| `audit` | GDPR compliance logging |

---

## 📚 Documentation

### Development

| Command | Description |
|---------|-------------|
| `go run main.go serve` | Start backend server |
| `go run main.go migrate` | Run database migrations |
| `go run main.go gendoc` | Generate API documentation |
| `npm run dev` | Start frontend dev server |
| `npm run check` | Run lint + typecheck |

### API Documentation

```bash
cd backend
go run main.go gendoc          # Generate routes.md and OpenAPI spec
```

This creates:
- `backend/routes.md` — Complete route documentation
- `backend/docs/openapi.yaml` — OpenAPI 3.0 specification

### Key API Endpoints

| Endpoint | Description |
|----------|-------------|
| `POST /api/auth/login` | Authentication |
| `GET /api/students` | List students |
| `GET /api/rooms` | List rooms |
| `GET /api/active/groups` | Active sessions |
| `POST /iot/checkin` | RFID check-in |

### Testing

```bash
# Backend tests
cd backend && go test ./...

# Frontend checks
cd frontend && npm run check

# API integration tests (Bruno)
cd bruno && bru run --env Local 0*.bru
```

---

## 🛡️ Security & Privacy

This project handles sensitive student data and implements comprehensive security measures:

- **SSL/TLS Encryption** — All database connections use SSL (`sslmode=require`)
- **GDPR Compliance** — Configurable data retention, audit logging, right-to-erasure
- **Role-Based Access** — Teachers only see data for students in their assigned groups
- **Secure Defaults** — No secrets in code, environment-based configuration

> **Reporting Vulnerabilities:** Please see [SECURITY.md](SECURITY.md) for our security policy and responsible disclosure process.

---

## 🗺️ Roadmap

- [x] RFID student tracking
- [x] Multi-supervisor support
- [x] GDPR compliance features (data retention, audit logging)
- [x] Email invitation workflow
- [x] Password reset with rate limiting
- [ ] Mobile companion app
- [ ] Real-time push notifications
- [ ] Advanced analytics and reporting
- [ ] Multi-language UI

See the [open issues](https://github.com/moto-nrw/project-phoenix/issues) for a full list of proposed features and known issues.

---

## 🤝 Contributing

Contributions are what make the open source community amazing! Any contributions you make are **greatly appreciated**.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'feat: add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request against `development`

Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details on our code of conduct, development setup, and the process for submitting pull requests.

> **Note:** By contributing, you agree to our [Contributor License Agreement](CLA.md).

---

## 📄 License

Distributed under the Apache License 2.0. See [LICENSE](LICENSE) for more information.

---

## 📬 Contact

- **Project Website:** [moto.nrw](https://moto.nrw)
- **GitHub:** [github.com/moto-nrw/project-phoenix](https://github.com/moto-nrw/project-phoenix)
- **Issues:** [Report a bug or request a feature](https://github.com/moto-nrw/project-phoenix/issues)

---

## 🙏 Acknowledgments

- [Chi Router](https://github.com/go-chi/chi) — Lightweight, idiomatic Go HTTP router
- [Bun ORM](https://bun.uptrace.dev/) — Fast and simple SQL-first ORM for Go
- [Next.js](https://nextjs.org/) — The React framework for production
- [Tailwind CSS](https://tailwindcss.com/) — Utility-first CSS framework
- [Shields.io](https://shields.io/) — Badges for this README

---

<div align="center">

Made with ❤️ by [moto](https://moto.nrw)

[⬆ Back to top](#project-phoenix)

</div>
