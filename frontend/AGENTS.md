<!-- BEGIN:nextjs-agent-rules -->

# Next.js: ALWAYS read docs before coding

Before any Next.js work, find and read the relevant doc in `node_modules/next/dist/docs/`. Your training data is outdated — the docs are the source of truth.

<!-- END:nextjs-agent-rules -->

# Frontend skills

Interface skills live in `frontend/.claude/skills/` (mirrored as symlinks in
`frontend/.agents/skills/`); `frontend/.claude/skills/README.md` lists them.
The root `.codex/hooks/skill-reminder.sh` does not scan this directory, so read
the relevant `SKILL.md` yourself when doing UI work. `better-interface`
coordinates the six `better-*` skills for a full review pass.

`.claude/rules/frontend-ui-kit.md` outranks all of them.
