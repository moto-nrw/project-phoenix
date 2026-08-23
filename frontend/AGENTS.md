<!-- BEGIN:nextjs-agent-rules -->

# This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` (resolved from this file's directory; in monorepos the `next` package may not be visible from the repo root) before writing any code. Heed deprecation notices.

This block is written and re-added by `next dev` — verify at `node_modules/next/dist/server/lib/generate-agent-files.js`. Removing it from a diff only re-creates the uncommitted change; committing it with your work keeps the tree clean.

<!-- END:nextjs-agent-rules -->

# Frontend skills

Interface skills live in `frontend/.claude/skills/` (mirrored as symlinks in
`frontend/.agents/skills/`); `frontend/.claude/skills/README.md` lists them.
They load once you touch a file under `frontend/`; if your agent cannot invoke
one by name, read its `SKILL.md` directly. `better-interface` coordinates the
six `better-*` skills for a full review pass.

`.claude/rules/frontend-ui-kit.md` outranks all of them.

# Frontend rules

This file is not a symlink to `frontend/CLAUDE.md` (the root and `backend/`
AGENTS.md are). Read `frontend/CLAUDE.md` for the frontend context, and these
two rules before any user-visible change:

- `.claude/rules/frontend-ui-kit.md` — build from the shared UI kit, brand
  colors from `LOCATION_COLORS` only.
- `.claude/rules/verstaendlichkeit.md` — the Missverständnis-Check: every
  visible block explains its purpose in one sentence, read-only blocks carry no
  button/chevron/pointer affordance, a function with a precondition states it in
  the product, no two labels share a word stem without a visible boundary. All
  user-facing German copy follows the `moto-einfache-sprache` skill.
