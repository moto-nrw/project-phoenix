# Frontend skills

Frontend-scoped on purpose. Claude Code loads them the moment an agent touches
a file under `frontend/`, so they cost no context during backend work. A skill
whose name also exists globally is offered as `frontend:<name>` and wins for
files under `frontend/`.

**`.claude/rules/frontend-ui-kit.md` outranks every skill in here.** Its "UI
skills" section names the three places where the vendored `better-*` advice is
wrong for this repo. Check it before acting on a skill's colour, surface, or
copy recommendation, and do not restate its rules here. One authority is the
whole point.

## Layout

Real files live in `frontend/.claude/skills/`; `frontend/.agents/skills/`
symlinks to them for Codex and other agents that follow the `.agents`
convention. One real copy per skill: two copies drift, and the root
`.agents/skills/` mirror of our own skills shows what that costs: a naive
mirror rewrote its `.claude/…` paths to `.Codex/…`, which resolve nowhere.

`react-doctor` points the other way (real files in `frontend/.agents/skills/`,
symlink from here). Left as it was.

## Vendored: `better-*`

Seven interface skills from [jakubkrehel/skills](https://github.com/jakubkrehel/skills):
`better-interface` plus `better-accessibility`, `better-layout`,
`better-writing`, `better-typography`, `better-colors`, `better-ui`.
MIT-licensed; the upstream licence text is in `LICENSE-better-skills` and has
to stay with the files.

`better-interface` coordinates the other six. Invoke it for a full pass
instead of running several overlapping reviews:

```
/better-interface                    # full review of the current scope
/better-interface quick              # HIGH and MEDIUM only, capped at 5 findings
/better-interface full checkout flow
```

The other six trigger from context on their own.

### Updating

```bash
cd frontend && npx skills add jakubkrehel/skills --skill '*'
```

The CLI writes real directories into `.agents/skills/` and replaces the
symlinks. Move them back and restore the links:

```bash
cd frontend
for s in .agents/skills/better-*; do
  n=$(basename "$s")
  rm -rf ".claude/skills/$n" && mv "$s" ".claude/skills/$n"
  ln -s "../../.claude/skills/$n" ".agents/skills/$n"
done
```

Then re-check the conflict table in `.claude/rules/frontend-ui-kit.md` against
the new content.
