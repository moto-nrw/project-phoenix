# Claude Code Configuration

This directory contains configuration for [Claude Code](https://claude.ai/code),
an AI-powered coding assistant by Anthropic.

## For Contributors Using Claude Code

This configuration provides:

- **Project-specific patterns**: BUN ORM query patterns, Docker workflows,
  multi-schema PostgreSQL
- **Auto-formatting on save**: Go (gofmt + goimports) and TypeScript (prettier)
  automatically formatted
- **Testing shortcuts**: Run Bruno API tests with `/test-api` command
- **Architecture guidance**: Factory pattern, repository/service layers, Next.js
  15 patterns
- **Error prevention**: Critical patterns like BUN ORM quoted aliases, Docker
  rebuild reminders
- **Workflow automation**: Slash commands for common tasks (`/rebuild-backend`,
  `/quality-check`, etc.)

### Getting Started with Claude Code

1. Install Claude Code: https://claude.ai/code
2. Navigate to project root: `cd project-phoenix`
3. Start Claude: `claude`
4. Configuration loads automatically

### Available Slash Commands

- `/test-api [domain|all]` - Run Bruno API tests
- `/rebuild-backend` - Rebuild Docker backend container
- `/quality-check` - Frontend lint + typecheck
- `/migrate-check` - Validate database migrations
- `/gendoc` - Generate API documentation

### Specialized Subagents

The configuration includes domain experts that Claude Code can invoke:

- **go-bun-expert**: Go + BUN ORM + PostgreSQL patterns
- **nextjs-expert**: Next.js 15 + React 19 + TypeScript
- **api-tester**: Bruno API testing workflows

## For Other Contributors

**You can safely ignore this directory** if you're not using Claude Code.

All necessary development information is also available in:

- `CLAUDE.md` (project root)
- `backend/CLAUDE.md` (backend-specific)
- `frontend/CLAUDE.md` (frontend-specific)
- `README.md` (general project info)

The configuration files are specific to Claude Code and won't affect your
development workflow with other editors or tools.

## What's Inside

```
.claude/
├── CLAUDE.md              # Main project memory with critical patterns
├── settings.json          # Permissions, hooks, and environment config
├── agents/                # Specialized AI assistants
│   ├── go-bun-expert.md
│   ├── nextjs-expert.md
│   └── api-tester.md
├── commands/              # Workflow shortcuts
│   ├── test-api.md
│   ├── rebuild-backend.md
│   ├── quality-check.md
│   ├── migrate-check.md
│   └── gendoc.md
└── hooks/                 # Automation scripts
    ├── format-go.sh
    ├── format-typescript.sh
    ├── check-commit-message.sh
    └── check-env-files.sh
```

## Configuration Highlights

### Security

- Blocks access to build artifacts (node_modules, .next, dist)
- Prevents web access (offline mode)
- Auto-validates commit messages

### Code Quality

- Auto-formats Go and TypeScript on save
- Zero warnings policy enforcement
- Conventional commit format validation

### Performance

- 2-minute timeout for Docker builds
- Large output handling (50,000 tokens)
- Fast file search with ripgrep

## Contributing to Configuration

If you're using Claude Code and want to improve this configuration:

1. Test your changes thoroughly
2. Ensure hooks remain executable: `chmod +x .claude/hooks/*.sh`
3. Validate JSON syntax: `cat .claude/settings.json | jq .`
4. Document new patterns in `CLAUDE.md`
5. Submit PR with clear description

## Learn More

- **Claude Code Documentation**: https://docs.claude.ai/code
- **Project Documentation**: See root `CLAUDE.md` for architecture details
- **Development Guide**: See `README.md` for setup instructions
