# Claude Code Configuration

Shared [Claude Code](https://claude.ai/code) configuration for this repo. Everything loads automatically when you start `claude` in the project root. (Codex users get the same rules/skills via the `.codex/` symlinks.)

## What's in here

```
.claude/
├── settings.json               # Shared team config (env limits, permissions, hooks)
├── settings.local.json.example # Template for personal overrides (gitignored copy)
├── hooks/                      # Automation scripts wired in settings.json:
│   ├── format-go.sh            #   gofmt + goimports after Go edits
│   ├── format-typescript.sh    #   prettier after TS edits
│   ├── check-commit-message.sh #   conventional-commit validation
│   ├── check-env-files.sh      #   env file security check on session start
│   ├── skill-reminder.sh       #   nudges active skill usage
│   └── subagent-reminder.sh
├── scripts/                    # Helper scripts (context-monitor.py)
├── skills/                     # Project skills: settings, help-guide-sync,
│   │                           #   env-docker-sync, creating-team-skills, find-skills
└── rules/                      # Always-on + path-scoped guidance (see below)
```

## Rules

Files in `.claude/rules/` without frontmatter are loaded into **every** session; files with `paths:` frontmatter attach when matching files are touched.

| Rule | Covers |
|------|--------|
| `backend-conventions.md` | Layer discipline, repository generics, the CI ratchet tests |
| `calendar-dates.md` | `timezone.Date` for DATE columns — the UTC-shift bug class |
| `settings-system.md` | Tenant-scoped settings registry and workflows |
| `frontend-ui-kit.md` | UI kit components, brand colors, design checklist |
| `help-guide-sync.md` | Keeping the in-app manual in sync with features |
| `env-docker-sync.md` | Env var + docker-compose file sync checklists |
| `github-labels.md` | Only existing labels; one Type + one Priority |
| `no-test-modifications.md` | Never change existing tests to make new code pass |
| `no-production-requests.md` | Never hit moto-app.de / moto.nrw domains |
| `security/hardcoded-credentials.md` | Never hardcode secrets |
| `security/project-security.md` | Crypto, certificates, uploads, security invariant index (path-scoped) |

## Personal customization

```bash
cp .claude/settings.local.json.example .claude/settings.local.json   # gitignored
```

A gitignored `CLAUDE.local.md` in the project root is included into the root context for personal notes.

## Contributing to this config

1. Keep hooks executable (`chmod +x .claude/hooks/*.sh`) and JSON valid (`jq . .claude/settings.json`)
2. When a rule's facts drift from the code, fix the rule in the same PR
3. PRs target `development`
