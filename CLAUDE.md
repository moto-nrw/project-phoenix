# moto — Agent entry point

Project Phoenix is the internal name of moto: student attendance and room
management for schools. Go/Chi/BUN backend, Next.js/React frontend, PostgreSQL.
Read versions and available scripts from manifests rather than this file.
All paths below are relative to the repository root.

## Working agreement

- Match the task: implement requested changes, diagnose reported failures,
  review when asked to review. General questions do not require a review ritual.
- Read affected files and applicable guidance before editing. Reuse existing
  helpers, standard libraries, and installed dependencies; choose the smallest
  correct change. Preserve validation, error handling, security, and accessibility.
- Continue authorized, reversible work without asking again. Ask when a missing
  business decision changes the result or an action exceeds the agreed scope.
  Explain the concrete conflict; do not invent an approval gate from a guideline.
- Preserve unrelated work. State unverified assumptions and blocked checks.
  Keep user-facing explanations concise, concrete, and technically precise.
- Review findings need a code location, failing scenario, and consequence.
  Trace affected error paths and security boundaries; distinguish verified
  failures from hypotheses. Use the `code-review` skill for a requested review.

## Safety and cross-stack invariants

- **School ID is the tenant boundary.** Preserve tenant-scoped transactions,
  RLS, and isolation between tenant, operator, parents, and school sessions.
  Parent-child access requires relationship-level permissions, not membership alone.
- **Production data:** use localhost for moto API calls. Read
  [.claude/rules/no-production-requests.md](.claude/rules/no-production-requests.md)
  before constructing requests; the staging/production domain guard stays active.
  The seeder is dev-only; production infrastructure uses migrations or admin UI.
- **Secrets:** keep credentials out of source and output. Edit deployed envs
  only through SOPS, not ciphertext or SSH `.env` edits. Read
  [.claude/rules/security/hardcoded-credentials.md](.claude/rules/security/hardcoded-credentials.md).
- **Configuration:** required infrastructure values fail fast when missing.
  School-admin runtime configuration uses tenant overrides and registry defaults
  only; consumers must not add env fallbacks, including for legacy compatibility.
  Exact exceptions and sync steps: [.claude/rules/env-docker-sync.md](.claude/rules/env-docker-sync.md).
- **Dates and IDs:** calendar dates are `timezone.Date` / `YYYY-MM-DD`, instants
  are timestamps, and clock times use wall-clock normalization. Backend `int64`
  IDs map to frontend strings. Load the date rule before changing date logic.
- **API contracts:** paths use kebab-case; migrate touched legacy snake_case
  paths with their consumers. IoT errors and auth headers are also consumed by
  `../PyrePortal/`; coordinate both sides. `../moto-balenaOS/` runs the kiosk,
  not the backend. Details: [docs/agents/contracts.md](docs/agents/contracts.md).

## Read when the task matches

Load matching references before design, implementation, or review; do not
bulk-load the table. References are required even if the runtime did not attach
path-scoped rules automatically.

| Task or affected surface | Required guidance |
|---|---|
| Any backend change | [backend/CLAUDE.md](backend/CLAUDE.md), including Active Architecture Migration #2580; inspect affected policy entries before choosing a boundary |
| Any frontend change | [frontend/AGENTS.md](frontend/AGENTS.md) and [frontend/CLAUDE.md](frontend/CLAUDE.md) |
| Login, MFA, refresh, sessions, routing, tenant scoping, enrollment, IoT | [docs/agents/contracts.md](docs/agents/contracts.md); check affected session mint/renew paths and wrong-portal rejection |
| Auth, permissions, crypto, certificates, uploads, sensitive data | [.claude/rules/security/project-security.md](.claude/rules/security/project-security.md); parent-child access also requires [.claude/rules/guardian-parent-permissions.md](.claude/rules/guardian-parent-permissions.md) |
| Dates, attendance days, recurrence, clock times | [.claude/rules/calendar-dates.md](.claude/rules/calendar-dates.md); shifts/timetables also require backend Domain Knowledge |
| Tenant settings | [.claude/rules/settings-system.md](.claude/rules/settings-system.md) and the `settings` skill |
| Env vars, Docker, SOPS, deployment | [.claude/rules/env-docker-sync.md](.claude/rules/env-docker-sync.md) and [docs/agents/operations.md](docs/agents/operations.md) |
| User-visible UI, emails, kiosk, help | [.claude/rules/verstaendlichkeit.md](.claude/rules/verstaendlichkeit.md); load `moto-einfache-sprache` before German copy, [.claude/rules/frontend-ui-kit.md](.claude/rules/frontend-ui-kit.md) before frontend UI |
| New/changed tenant feature flow or help content | [.claude/rules/help-guide-sync.md](.claude/rules/help-guide-sync.md); update guide and affected screenshots in the same PR (exemptions in the rule) |
| Failing or changing tests | [.claude/rules/no-test-modifications.md](.claude/rules/no-test-modifications.md); [backend fixture rules](docs/agents/backend-testing.md) |
| Domain terminology or architecture decisions | [CONTEXT.md](CONTEXT.md), relevant `docs/adr/`, [docs/agents/domain.md](docs/agents/domain.md) |
| Issues or labels | [docs/agents/issue-tracker.md](docs/agents/issue-tracker.md), [docs/agents/triage-labels.md](docs/agents/triage-labels.md), [docs/agents/github-labels.md](docs/agents/github-labels.md) |
| PR screenshots or service/test-DB commands | [docs/agents/operations.md](docs/agents/operations.md) |
| Pushed fixes for a quorum review | [docs/agents/quorum-review-loop.md](docs/agents/quorum-review-loop.md); run `scripts/quorum-rerequest.sh` after the push |
| Agent instructions, skills, hooks, or context maintenance | `writing-for-agents` skill and [.claude/README.md](.claude/README.md) |

## Completion and commands

Use Docker Compose for services (`docker compose up -d`, `docker compose logs -f server`).
Host-side quality/test commands below are deliberate exceptions.
Tools belong in Devbox; do not depend on unrecorded global installations.

| Change | Required verification |
|---|---|
| Frontend | `cd frontend && pnpm run check` — zero warnings; relevant behavior tests |
| Backend | Relevant tests via `scripts/run-go-toolchain.sh`; architecture migration checks in backend/CLAUDE.md |
| Code before push | `scripts/test-changed.sh origin/development` without `--fast` |
| User-visible | Verständlichkeit checklist recorded in PR; UI screenshot and help-guide requirements from the matching rules |
| Docs/hooks only | `node scripts/check-agent-context.mjs` and `node --test scripts/check-agent-context.test.mjs`; verify changed hook behavior |

Run required checks once after the last relevant edit. Broaden or repeat checks
when changes, failures, or unresolved risks justify it. Report what actually ran;
a missing tool or failed command is not a passed check.

## Git and skills

PRs target `development` (moto-balenaOS: `main`). Commit types: `feat`, `fix`,
`refactor`, `chore`, `docs`, `test`, `style`. Use descriptive titles and names,
without AI/tool branding or `Co-Authored-By: Claude`.

Use named and task-relevant skills; load their `SKILL.md` through the available
skill tool or read the file directly. The runtime catalog is the discovery
source; the reminder is not an instruction to load every skill. Repository
skills live in `.agents/skills/` / `.claude/skills/`; area skills live under
`backend/` and `frontend/`. Preserve working symlinks instead of copying files.

Personal notes may live in a gitignored `CLAUDE.local.md`; read it if present.
Keep shared project rules here and in the linked references, not in local notes.
