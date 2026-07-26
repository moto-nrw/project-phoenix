---
name: ui-before-after
description: Use when a UI change needs before/after screenshot pairs for a PR or review. Triggers on "vorher/nachher", "before/after screenshots", visual comparison of a frontend refactor, or verifying that a migration did not change the look.
metadata:
  author: moto-nrw
  version: "1.0.0"
---

# Before/After UI Screenshot Pairs

Produce matched before/after screenshots of frontend changes by hot-swapping
`frontend/src` between two git states on a RUNNING dev server, capturing the
same interactions twice with agent-browser, and compositing pairs side by side.

Core idea: the dev server (Turbopack) hot-reloads mounted sources, so you never
rebuild or restart anything between the two states — only the source tree flips.

## Prerequisites

- Working tree is CLEAN (changes committed). Verify: `git status --short` is empty.
- Local stack running: backend/postgres/mailpit via `docker compose up -d`.
- Frontend: **prefer local `pnpm dev`** (`PORT=<port> pnpm dev` in `frontend/`,
  with `frontend/.env.local` pointing `API_URL`/`NEXT_PUBLIC_API_URL` at the
  compose server port). The dockerized dev frontend reproducibly crash-loops
  compiling `[tenant]` pages (known Turbopack trap) — do not fight it.
- Seeded data + login: slug and credentials come from `backend/.seed-state.json`
  (fallback password `Test1234%`). Operator login needs an email MFA code —
  fetch it from Mailpit: `curl localhost:<MAILPIT_PORT>/api/v1/messages`.
- Use a dedicated agent-browser session: `agent-browser --session <name> ...`
  (the default session is shared across concurrent Claude sessions).

## Workflow

1. **Plan the shot list first.** One entry per changed surface: URL, the exact
   interaction (open menu X, search gibberish for the empty state), and the
   state needed (own post for owner-only menus, seeded rows for badges).
   Create any needed data ONCE before starting — it must exist in both passes.
2. **BEFORE pass** — flip sources to the baseline:
   ```bash
   git checkout origin/development -- frontend/src   # or any base ref
   ```
   Wait for recompile on first page hit (~2-3s), then walk the shot list:
   ```bash
   agent-browser --session s open <url>
   agent-browser --session s snapshot -i -c   # fresh refs EVERY page/state change
   agent-browser --session s click @eN        # e.g. open the kebab menu
   agent-browser --session s screenshot shots/before/<nn>-<name>.png
   ```
3. **AFTER pass** — restore and repeat the identical list:
   ```bash
   git checkout HEAD -- frontend/src
   ```
   Same URLs, same interactions, same filenames under `shots/after/`.
4. **Composite pairs** (ImageMagick, left=before right=after):
   ```bash
   for f in before/*.png; do n=$(basename "$f")
     magick "$f" -resize 50% -bordercolor gray -border 1 /tmp/b.png
     magick "after/$n" -resize 50% -bordercolor gray -border 1 /tmp/a.png
     magick /tmp/b.png /tmp/a.png +append "pair-$n"
   done
   ```
5. **Verify sources are restored**: `git status --short` must be empty again.
6. Deliver: Read the `pair-*.png` files to show them inline; for the PR, give
   the user the local paths to attach manually (never host via releases/gists —
   see the PR-screenshots rule in the root CLAUDE.md).

## Gotchas (all hit in practice)

- **Refs go stale after every hot-reload/re-render** — re-run `snapshot -i -c`
  before each interaction; a `fill` on a stale ref reports success but does
  nothing. If a page renders empty after the source flip, `reload` once.
- **Same viewport both passes**: set `agent-browser set viewport 1440 900` once
  at the start; a viewport mismatch makes pairs useless.
- **Identical data both passes**: never create/delete records between the
  passes; timestamps ("vor X Minuten") will differ — that is acceptable.
- **Menus/popovers**: screenshot with the menu OPEN, then `press Escape`
  before navigating on.
- **`git checkout <ref> -- frontend/src` only replaces tracked files** — new
  files added by your branch remain on disk during the BEFORE pass. That is
  fine (they are unimported in the old state), but do not delete them.
- Empty states are usually reachable by searching gibberish (`xyznichts`).

## Scope

Screenshot comparison only. For stack bootstrap (ports, certs, seeding) see
the worktree-setup memory/docs; for general browser driving see the
`agent-browser` skill.
