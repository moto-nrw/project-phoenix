# Contributing to Project Phoenix

Thank you for your interest in contributing to Project Phoenix! This document provides guidelines for contributing to the project.

## Contributor License Agreement (CLA)

Before your first contribution can be accepted, you must agree to our [Contributor License Agreement](CLA.md). By submitting a pull request, you indicate your acceptance of the CLA terms.

## Getting Started

1. **Fork the repository** and clone it locally
2. **Set up your development environment** following [docs/getting-started.md](docs/getting-started.md)
3. **Create a branch** for your changes: `git checkout -b feature/your-feature-name`

## Development Workflow

### Prerequisites

- Docker and Docker Compose
- [Devbox](https://www.jetify.com/devbox/docs/installing_devbox/) + [direnv](https://direnv.net/docs/installation.html) (pin Go 1.27.0, Node 24+, and all CLI tools)

The pinned Devbox environment supports Apple Silicon macOS and Linux on arm64
or amd64. Intel macOS is not supported; current Nixpkgs releases no longer
provide `x86_64-darwin` packages.

### Quick Setup

```bash
git clone https://github.com/moto-nrw/project-phoenix.git
cd project-phoenix
direnv allow              # activates the devbox environment
devbox run bootstrap      # installs frontend and browser dependencies
./scripts/setup-dev.sh    # creates config files, SSL certs, and credentials
docker compose up -d      # starts everything; migrations run automatically
```

See [docs/getting-started.md](docs/getting-started.md) for seeding demo data and troubleshooting.

### Running Quality Checks

**Backend (Go):**
```bash
cd backend
../scripts/run-go-toolchain.sh golangci-lint run --timeout 10m  # Linting
../scripts/run-go-toolchain.sh go test ./...                     # Tests
../scripts/run-go-toolchain.sh go fmt ./...                      # Formatting
```

**Frontend (Next.js):**
```bash
cd frontend
pnpm run check    # Lint + TypeScript (MUST pass before PR)
pnpm run test     # Run tests
```

## Submitting Changes

### Pull Request Process

1. Ensure all quality checks pass (`pnpm run check` for frontend, `../scripts/run-go-toolchain.sh golangci-lint run` from `backend/`)
2. Update documentation if you changed APIs or behavior
3. Write clear commit messages following [Conventional Commits](https://www.conventionalcommits.org/)
4. Open a PR against the `development` branch (NOT `main`)
5. Fill out the PR template with a clear description

### Commit Message Format

```
type(scope): subject

body (optional)

footer (optional)
```

**Types:** `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`

**Examples:**
- `feat(auth): add password reset flow`
- `fix(iot): handle missing device gracefully`
- `docs: update API documentation`

## Code Style

### Go
- Follow standard Go conventions
- Use the pinned formatter: `../scripts/run-go-toolchain.sh go tool goimports -w .`
- Run `../scripts/run-go-toolchain.sh golangci-lint run` before committing

### TypeScript/React
- Use TypeScript strict mode
- Follow oxlint rules (zero warnings policy)
- Use Prettier for formatting

## Reporting Issues

- Use GitHub Issues for bug reports and feature requests
- Include steps to reproduce for bugs
- Check existing issues before creating duplicates

## Code of Conduct

This project follows the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md). By participating, you agree to uphold this code. Please report unacceptable behavior through the channels described in the Code of Conduct.

## Questions?

Open an issue or reach out to the maintainers.

---

Thank you for contributing! 🎉
