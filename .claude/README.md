# Agent context and hooks

Shared guidance for Claude Code and Codex. Start with the root `CLAUDE.md`;
load its references only when their task trigger matches.

## Entry points and canonical sources

| Surface | Source / load path |
|---|---|
| Root | `AGENTS.md` is a symlink to `CLAUDE.md`; maintain the target only |
| Backend | `backend/AGENTS.md` is a symlink to `backend/CLAUDE.md` |
| Frontend | `frontend/AGENTS.md` contains the Next.js-managed block and routes to `frontend/CLAUDE.md`; preserve the generated block |
| Cross-stack contracts | `docs/agents/contracts.md`: portals, tenant scoping, IoT, enrollment |
| Operations | `docs/agents/operations.md`: commands, test DB, SOPS, PR screenshots |
| Domain and decisions | `CONTEXT.md`, `docs/adr/`; conventions in `docs/agents/domain.md` |
| Task workflows | Root `.agents/skills/` is canonical; `.claude/skills/` contains directory symlinks. Preserve area-local canonical directions |
| Backend detail | `docs/agents/backend-testing.md` for fixtures; `docs/agents/backend-data.md` for BUN and migrations |
| Frontend detail | `docs/agents/frontend-api.md` for server/API boundaries; `docs/agents/frontend-performance.md` for budgets |
| Real-time contract | `docs/agents/realtime.md` for producers, streaming, and client refetches |

Codex instruction discovery uses `AGENTS.md`; a `.codex/rules` symlink alone
is not proof that a runtime loaded every rule. The entry points explicitly
require relevant references, independently of automatic loading. Check the
active session's catalog and loaded instructions when diagnosing discovery.

In Claude Code, rules without `paths:` frontmatter are always-on; only
production-request and credential guards remain unconditional. Other rules,
including UI comprehension, test contracts, and guardian permissions, have
path scopes. Semantic triggers in the root table cover tasks that need a rule
before touching those paths (including kiosk changes in a sibling repo).
Label and quorum workflows have no reliable source-file trigger: they live in
`docs/agents/github-labels.md` and `docs/agents/quorum-review-loop.md`, loaded
by the root task table. The quorum Stop hook remains unchanged.

## Skills

Use the runtime skill catalog first. If a named skill cannot be invoked,
read its `SKILL.md` directly. A skill is a directory containing `SKILL.md`;
a loose Markdown file is reference material, not a discoverable skill.
Frontend interface skills are indexed in `frontend/.claude/skills/README.md`.
The UI-kit rule outranks generic UI skill advice.

`skill-reminder.sh` emits a short pointer on UserPromptSubmit. It does not
rescan personal directories, repeat the catalog, or infer task scope from
unrelated dirty files. Root and area pointers remain available on every turn.
Skill mirrors resolve to the same directory, not independently maintained
copies. Env-sync entries also delegate to one canonical rule.

## Hook wiring and limits

`.claude/settings.json` and `.codex/hooks.json` wire shared scripts;
`.codex/hooks/*.sh` points into `.claude/hooks/`. Codex's repo config enables
hooks, but runtime tool matchers still determine which calls are intercepted.

| Hook | Responsibility |
|---|---|
| `format-go.sh`, `format-typescript.sh` | Formatting after matching edits |
| `check-commit-message.sh` | Commit convention validation |
| `check-env-files.sh` | Session-start env file check |
| `guard-absolute-rules.sh` | Block moto staging/production requests, ciphertext hand-edits, RLS disabling in migrations, commit-hook bypass, and untracked/outside-repo script execution |
| `skill-reminder.sh` | Task-relevant skill pointers |
| `subagent-reminder.sh` | Delegation guidance subject to session instructions |
| `scripts/quorum-rerequest.sh --stop-hook` | Re-request quorum after pushed review fixes |
| `scripts/stop-quality-gate.sh --stop-hook` | Changed-area build/vet/frontend checks |

The Codex config matches shell PreToolUse calls, not Edit/Write for the guard;
Claude wires both. Keep lefthook and CI backstops. The guard resolves tracked
scripts against payload `cwd`, falling back to the hook process working directory.
Do not assume a hook ran merely because a config file exists.

## Maintaining context

- Keep a rule in one canonical source. Entry points carry its trigger and a
  short invariant, not a second tutorial. Keep security boundaries visible.
- Put only cross-task information in the root. Put domain-specific examples
  beside their domain; versions and script inventories come from manifests.
- Temporary migration guidance names its issue and removal criteria.
  Current #2580 guidance in `backend/CLAUDE.md` overrides legacy layer examples.
- Keep user preferences out of team rules. Optional `CLAUDE.local.md` is read
  if present; global Codex/Claude instruction files are outside this repo.
- Run `node scripts/check-agent-context.mjs`, its fixture tests with
  `node --test scripts/check-agent-context.test.mjs`, and `git diff --check`
  through the Devbox environment. Lefthook runs both checks on context changes
  before committing. The checker covers local Markdown links,
  concrete inline document pointers, path casing, symlinks, and skill mirrors.
  It does not prove instructions are correct or automatically loaded.
- Check changed hook behavior separately. For reminder changes, check valid
  JSON, empty input, non-repo cwd, and missing `jq`. Missing tooling must not
  report a successful check.

### Routing checks after a context change

| Example task | Expected references / behavior |
|---|---|
| Fix a backend calendar-day bug | Backend migration policy, calendar-date rule, hermetic tests; fix implementation without asking to preserve an unchanged contract |
| Change a tenant UI flow | Frontend entry points, UI kit, Verständlichkeit, German-copy skill, help-guide and screenshot rules |
| Review a login/refresh change | Review skill, affected area rules, contracts and security; include MFA and wrong-portal rejection |

These are routing checks, not a measured model-quality benchmark. To compare
agent behavior, use the same base revision and tasks in fresh sessions and
record loaded references, unnecessary questions, checks, and missed requirements.
Do not claim a behavioral improvement from file size alone. The September 2026
comparison and its limits are recorded in
[context routing checks](../docs/agents/context-routing-check.md).

### Context cleanup: disposition

| Previous root content | Decision |
|---|---|
| Tenant/portal tables, IoT presence mode, enrollment details | Move to `docs/agents/contracts.md`; correct portal count and include school scope |
| Service commands, test DB maintenance, SOPS, screenshot hosting | Move to `docs/agents/operations.md`; reconcile test-clone lifecycle with backend guidance |
| Backend detail | Keep recurrence, tenant safety and #2580 precedence in `backend/CLAUDE.md`; disclose BUN/migration recipes and fixture lifecycle |
| Frontend detail and SSE | Disclose API examples, performance maintenance, and cross-stack SSE; entry points name their task triggers |
| UI, dates, env sync, settings, help details | Keep canonical rules; root provides explicit task triggers |
| Versions, table counts, repeated examples | Remove caches where code/manifests provide the answer |
| Blanket stop on failing tests | Replace with evidence-based contract handling; preserve assertion strength |
| Full per-prompt skill catalog and unavailable-tool instruction | Replace with short, tool-neutral pointers |

Personal review-persona instructions (such as the $100 approval wager) live
outside this repository. They were not deleted or changed by this cleanup.
