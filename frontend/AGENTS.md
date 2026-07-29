<!-- BEGIN:nextjs-agent-rules -->

# Next.js: ALWAYS read docs before coding

Before any Next.js work, find and read the relevant doc in `node_modules/next/dist/docs/`. Your training data is outdated — the docs are the source of truth.

<!-- END:nextjs-agent-rules -->

# Frontend skills

Interface skills live in `frontend/.claude/skills/` (mirrored as symlinks in
`frontend/.agents/skills/`); `frontend/.claude/skills/README.md` lists them.
They load once you touch a file under `frontend/`; if your agent cannot invoke
one by name, read its `SKILL.md` directly. `better-interface` coordinates the
six `better-*` skills for a full review pass.

`.claude/rules/frontend-ui-kit.md` outranks all of them.
