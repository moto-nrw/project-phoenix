# Context routing comparison — 2026-09-04

## Method

Six isolated subagent sessions rehearse three tasks against before/after
context snapshots from the same code revision. Each starts with `AGENTS.md`
and follows its own task-relevant pointers. Each receives the same scenario
and reporting instructions; only the snapshot path changes. No conversation
history is inherited. Application code, installed dependencies, personal
instructions, and automatic rule injection are not part of these snapshots.

The before snapshot includes the user's uncommitted context cleanup, not just
HEAD. The after snapshot adds this consolidation. These are read-only routing
rehearsals: no application checks or implementation were requested or run.
They cannot establish production correctness or measure the host's automatic
context loading. One run per scenario is not a statistical model benchmark.

## Scenarios and acceptance criteria

| Scenario | Required discovery |
|---|---|
| Fix a backend attendance calendar-day bug around Berlin midnight; existing expected date unchanged | Calendar-date rule, architecture policy, test contracts and hermetic lifecycle; fix implementation without another approval; focused uncached test and architecture checks |
| Rename a German control and move its action in a documented tenant UI flow; business behavior unchanged | Frontend entry points, local UI kit, German-copy skill, comprehension check, help-guide updates, matched screenshots, unchanged action/permission/error behavior |
| Review login and refresh across backend/frontend, including school sessions; no diff supplied | Review skill, area guidance, contracts, security, affected architecture; MFA and wrong-portal rejection including refresh; request a real diff only for an actual review |

## Results

Both snapshots use code revision `9f1012d6ecc99aa62e6611542bb85b437b650f31`.
Local snapshots: `/tmp/moto-context-before-BsMbwu` and
`/tmp/moto-context-after-biLPVc` (temporary, not required by the repo).

| Scenario | Documents read, before → after | Observed result | Clarification |
|---|---|---|---|
| Calendar bug | 14 → 15 | Both recovered DATE semantics, uncached tests, tenant fixtures and architecture checks; after found both new backend references | None; unchanged expectations require implementation repair, not approval |
| Tenant UI flow | 17 → 17 | Both required guide/screenshots despite unchanged business behavior; after found the new API reference | Actual control/destination needed for implementation, not permission to proceed |
| Login/refresh review | 14 → 16 | Both recovered school tenant binding, four mint/renew guards, MFA, portal cookies and wrong-scope rejection; after found fixture/API references | Real diff/base/spec needed for an actual review |

No core acceptance criterion above was absent from the six reports.
None introduced an unnecessary permission request. All proposed focused
behavior checks plus area quality checks and the before-push test command.
These are reported intentions, not executed application checks.

**Limits and unresolved context:** the after calendar session reported a
truncated opening read of `CONTEXT.md`; architecture-policy reads were
selective, not complete audits. Both UI sessions found the screenshot skill's
unspecified “worktree-setup memory/docs” reference; its source-swapping workflow
also requires a clean committed tree. This comparison does not validate that
workflow. All runs lacked application code, manifests, bundled Next.js docs,
and optional local notes by design.

The after calendar session also found the stale “rule 11 in CLAUDE.md” pointer.
It was subsequently replaced by a checked link to backend date/time invariants.
After the snapshots, static validation also repaired nine Postgres TOC anchors,
removed two dangling Codex aliases, and clarified moved reference paths.
Those mechanical fixes were checked statically, not rerun as routing probes.

The observations support preserved routing for these three tasks, not a claim
of better model quality. Document counts did not decrease: splitting a file
can require more reads while reducing irrelevant content per read.

## Actually loaded reference inventories

These lists preserve each session's reported path spelling. Skill aliases may
resolve to the same source. They exclude merely planned code inspections.
`policy.json` was read selectively in backend scenarios.

### Calendar-day bug

Both sessions:

```text
AGENTS.md
backend/CLAUDE.md
.claude/rules/calendar-dates.md
.claude/rules/no-test-modifications.md
.claude/rules/backend-conventions.md
.claude/rules/security/hardcoded-credentials.md
.claude/rules/security/project-security.md
backend/architecture/README.md
backend/architecture/policy.json
CONTEXT.md
docs/agents/operations.md
docs/adr/0004-testsuite-besitzt-datenbanken-nicht-server.md
```

Before only:

```text
.agents/skills/diagnosing-bugs/SKILL.md
docs/agents/domain.md
```

After only:

```text
.claude/skills/diagnosing-bugs/SKILL.md
docs/agents/backend-testing.md
docs/agents/backend-data.md
```

### Tenant UI flow

Both sessions:

```text
AGENTS.md
frontend/AGENTS.md
frontend/CLAUDE.md
frontend/.claude/skills/README.md
.claude/rules/verstaendlichkeit.md
.claude/rules/frontend-ui-kit.md
.claude/rules/help-guide-sync.md
.claude/rules/no-test-modifications.md
.claude/rules/no-production-requests.md
.claude/rules/security/hardcoded-credentials.md
docs/agents/contracts.md
docs/agents/operations.md
.claude/skills/moto-einfache-sprache/SKILL.md
.claude/skills/help-guide-sync/SKILL.md
frontend/.claude/skills/ui-before-after/SKILL.md
frontend/.claude/skills/agent-browser/SKILL.md
```

Before only:

```text
.claude/rules/security/project-security.md
```

After only:

```text
docs/agents/frontend-api.md
```

### Login/refresh review

Both sessions:

```text
AGENTS.md
backend/CLAUDE.md
frontend/AGENTS.md
frontend/CLAUDE.md
docs/agents/contracts.md
.claude/rules/security/project-security.md
.claude/rules/security/hardcoded-credentials.md
.claude/rules/backend-conventions.md
.claude/rules/no-production-requests.md
.claude/rules/no-test-modifications.md
backend/architecture/README.md
backend/architecture/policy.json
```

Before only:

```text
docs/agents/operations.md
.agents/skills/code-review/SKILL.md
```

After only:

```text
docs/agents/frontend-api.md
docs/agents/backend-testing.md
docs/agents/issue-tracker.md
.claude/skills/code-review/SKILL.md
```

## Static validation

The checker is a separate deterministic check, not part of the model probes.
It scans context documents, explicit document pointers/imports, local Markdown
anchors, symlinks, and canonical skill mirrors. Synthetic fixtures exercise
missing targets, casing, cycles, divergent/identical copies, and anchor failures.
Lefthook runs the checker and fixtures for context changes. Remote URLs,
placeholder paths, optional notes, and arbitrary prose are outside its scope.

Executed through Devbox: `lefthook validate` passed. The new pre-commit job
was exercised with `lefthook run pre-commit --command agent-context --file
docs/agents/context-routing-check.md --no-auto-install --no-stage-fixed`:
210 documents checked, zero errors, 11 tests passed, none skipped.
`git diff --check` also passed. No application suites ran for this
documentation/checker-only change.
