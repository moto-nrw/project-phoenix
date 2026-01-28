# Claude Code Configuration

This directory contains a **batteries-included** configuration for [Claude Code](https://claude.ai/code). Everything is pre-configured so you can start coding immediately with maximum productivity.

## Quick Start

```bash
# 1. Install Claude Code (if you haven't)
# Visit: https://claude.ai/code

# 2. Navigate to project and start
cd project-phoenix
claude

# That's it! Everything loads automatically.
```

## What You Get (Out of the Box)

### Plugins (11 active)

| Plugin | Purpose |
|--------|---------|
| **superpowers** | Enhanced skills and workflows |
| **codeguard-security** | Security vulnerability detection |
| **pr-review-toolkit** | Automated PR review assistance |
| **ralph-wiggum** | Iterative AI development loops |
| **code-refactoring** | Smart code refactoring patterns |
| **cicd-automation** | CI/CD pipeline automation |
| **backend-api-security** | API security best practices |
| **api-scaffolding** | API endpoint scaffolding |
| **codebase-cleanup** | Dead code detection and cleanup |
| **agent-browser** | Browser automation CLI for AI agents |
| **typescript-lsp** | TypeScript language server |
| **gopls-lsp** | Go language server |

### Specialized Agents (6 available)

| Agent | When to Use |
|-------|-------------|
| **go-bun-expert** | BUN ORM queries, repository patterns, Chi router |
| **nextjs-expert** | Next.js 15 + React 19, App Router, Server Components |
| **api-tester** | Bruno API testing, authentication flows |
| **prompt-engineer** | AI prompt optimization, agent design |
| **deep-dive-investigator** | Complex debugging, tracing code execution paths |
| **research-thinker** | Research topics, compare alternatives, gather info |

### Skills (8 available)

| Skill | Purpose |
|-------|---------|
| **vercel-react-best-practices** | React/Next.js performance patterns (45 rules) |
| **gh-cli** | GitHub CLI command reference |
| **slack-formatter** | Format messages for Slack |
| **sonarqube-mcp** | SonarQube integration |
| **ui-skills** | UI/UX design constraints |
| **web-design-guidelines** | Web interface guidelines |
| **ralph-setup** | Set up iterative AI development loops |
| **agent-browser** | Browser automation for web testing |

### Slash Commands

| Command | Description |
|---------|-------------|
| `/test-api [domain\|all]` | Run Bruno API tests |
| `/rebuild-backend` | Rebuild Docker backend container |
| `/quality-check` | Frontend lint + typecheck |
| `/migrate-check` | Validate database migrations |
| `/gendoc` | Generate API documentation |

### Auto-Formatting Hooks

- **Go files**: Auto-formatted with `gofmt` + `goimports` on save
- **TypeScript files**: Auto-formatted with `prettier` on save
- **Commit messages**: Validated for conventional commit format
- **Environment files**: Checked for security issues on session start

### Security Rules (23 rules)

Comprehensive security guidelines covering:
- Cryptographic algorithms (banned/deprecated)
- Digital certificates validation
- Hardcoded credentials detection
- Input validation and injection prevention
- API security patterns
- And 18 more categories...

## Configuration Files

```
.claude/
├── settings.json              # Shared team config (plugins, hooks, permissions)
├── settings.local.json        # YOUR local overrides (gitignored)
├── settings.local.json.example # Template for local settings
├── agents/                # 6 specialized AI assistants
│   ├── go-bun-expert.md
│   ├── nextjs-expert.md
│   ├── api-tester.md
│   ├── prompt-engineer.md
│   ├── deep-dive-investigator.md
│   └── research-thinker.md
├── commands/              # 8 workflow shortcuts
│   ├── test-api.md
│   ├── rebuild-backend.md
│   ├── quality-check.md
│   ├── migrate-check.md
│   ├── gendoc.md
│   └── openspec/          # OpenSpec change management
├── hooks/                 # 4 automation scripts
│   ├── format-go.sh
│   ├── format-typescript.sh
│   ├── check-commit-message.sh
│   └── check-env-files.sh
├── skills/                # 7 reusable skill definitions
│   ├── vercel-react-best-practices/
│   ├── gh-cli/
│   ├── slack-formatter/
│   ├── sonarqube-mcp/
│   ├── ui-skills/
│   ├── web-design-guidelines/
│   ├── ralph-setup/
│   └── agent-browser/
└── rules/                 # 23 security guidelines
    ├── openspec.md
    └── security/
```

## For Non-Claude-Code Users

**You can safely ignore this directory.** All development info is also in:
- `CLAUDE.md` (project root) - Architecture and patterns
- `README.md` (project root) - General setup

## Key Features Explained

### Model Default
Configured to use **opus** model for best code generation quality.

### Token Limits
- Output: 100,000 tokens
- Read: 50,000 lines
- Thinking: 10,000 tokens

### MCP Servers
All project MCP servers are auto-enabled. Current MCPs include:
- Context7 (documentation lookup)
- Brave Search (web search)
- SonarQube (code quality)
- agent-browser (browser automation CLI)
- Sequential Thinking (complex reasoning)

## Personal Customization

Want to customize your experience? Create a `settings.local.json`:

```bash
cp .claude/settings.local.json.example .claude/settings.local.json
# Edit to add your preferences (gitignored, won't affect team)
```

**Common customizations:**
- `outputStyle`: "Learning" for educational mode, "concise" for brief responses
- MCP permissions: Add your API keys' MCP tools (Brave Search, Context7, etc.)
- Telemetry: Add `"CLAUDE_CODE_ENABLE_TELEMETRY": "1"` to env if you want it

## Troubleshooting

### Plugins Not Loading
Run `claude /doctor` to check plugin status.

### Hooks Not Executing
Ensure scripts are executable:
```bash
chmod +x .claude/hooks/*.sh
```

### JSON Syntax Errors
Validate settings:
```bash
cat .claude/settings.json | jq .
```

## Contributing to Configuration

1. Test changes thoroughly before committing
2. Keep hooks executable: `chmod +x .claude/hooks/*.sh`
3. Validate JSON: `cat .claude/settings.json | jq .`
4. Document new patterns in `CLAUDE.md`
5. Submit PR targeting `development` branch

## Learn More

- **Claude Code Docs**: https://docs.claude.ai/code
- **Project Architecture**: See root `CLAUDE.md`
- **Development Setup**: See root `README.md`
