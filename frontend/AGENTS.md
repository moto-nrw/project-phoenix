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
