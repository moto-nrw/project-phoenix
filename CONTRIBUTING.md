# Contributing to Project Phoenix

Thank you for your interest in contributing to Project Phoenix! This document provides guidelines for contributing to the project.

## Contributor License Agreement (CLA)

Before your first contribution can be accepted, you must agree to our [Contributor License Agreement](CLA.md). By submitting a pull request, you indicate your acceptance of the CLA terms.

## Getting Started

1. **Fork the repository** and clone it locally
2. **Set up your development environment** following the instructions in [CLAUDE.md](CLAUDE.md)
3. **Create a branch** for your changes: `git checkout -b feature/your-feature-name`

## Development Workflow

### Prerequisites

- Go 1.23+
- Node.js 20+
- Docker and Docker Compose
- PostgreSQL 17+ (or use Docker)

### Quick Setup

```bash
# Clone and setup
git clone https://github.com/moto-nrw/project-phoenix.git
cd project-phoenix

# Generate SSL certificates (required)
cd config/ssl/postgres && ./create-certs.sh && cd ../../..

# Copy environment files
cp backend/dev.env.example backend/dev.env
cp frontend/.env.example frontend/.env.local

# Start services
docker compose up -d
```

### Running Quality Checks

**Backend (Go):**
```bash
cd backend
golangci-lint run --timeout 10m  # Linting
go test ./...                     # Tests
go fmt ./...                      # Formatting
```

**Frontend (Next.js):**
```bash
cd frontend
npm run check    # Lint + TypeScript (MUST pass before PR)
npm run test     # Run tests
```

## Submitting Changes

### Pull Request Process

1. Ensure all quality checks pass (`npm run check` for frontend, `golangci-lint run` for backend)
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
- Use `gofmt` and `goimports`
- Run `golangci-lint` before committing

### TypeScript/React
- Use TypeScript strict mode
- Follow ESLint rules (zero warnings policy)
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
