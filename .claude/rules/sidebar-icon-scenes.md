---
paths:
  - "frontend/src/components/**/*sidebar*"
  - "frontend/src/components/**/*-nav-items.ts"
  - "frontend/src/lib/staff-navigation.ts"
  - "frontend/src/lib/moto-concepts.ts"
  - "frontend/src/lib/concepts/**"
  - "frontend/src/components/ui/nav-scene-icon/**"
  - "frontend/src/styles/nav-scenes.css"
---

# Sidebar Icon Scenes

**RULE: Every icon in a sidebar row of any portal (tenant, Schule, Träger, Eltern) plays its own scene on hover and keyboard focus: parts of the glyph act out what the symbol means, at the agreed "Mittel" tempo. A sidebar icon that is new, or whose glyph changes, ships with its scene in the same PR.**

The canonical spec is `frontend/src/components/ui/nav-scene-icon/README.md`: the build steps, the measurements ("Maße und Tempo"), the techniques and the behavior. This rule holds the invariant and the gotchas; the README holds the values.

## Character

Each glyph gets its own *action*: the calendar tears off a sheet, the coin flips, the eye blinks, the door swings open. The action comes from the symbol's meaning. Scaling the whole glyph, or reusing another icon's motion from a different side, misses the character the team chose. One glyph shared by several concepts has one scene, keyed by the glyph's `displayName`.

## When you touch a sidebar

1. Render the icon through `NavConceptIcon` (a concept) or `NavSceneIcon` (a Phosphor glyph). A hand-drawn path icon goes inside `NavSceneFallback`.
2. A glyph without an entry in `NAV_SCENES` needs a scene: build it per the README, as a redrawn SVG in `nav-scenes.tsx` plus its motion in `styles/nav-scenes.css`. Changing a concept's icon counts as a new glyph.
3. A new row type sets `--nav-row-bg` for idle, hover and active, like `ROW_BG_IDLE` / `ROW_BG_ACTIVE` in `sidebar-geometry.ts`. Masks paint with that color.
4. Done when `pnpm exec vitest run src/components/ui/nav-scene-icon` is green and, in the running portal, hovering the row on an idle and on an active row plays the scene from the first frame to the rest state without a jump. The PR carries a hover recording per the screenshot rules.

## Guard

`nav-scene-coverage.test.ts` reads the sidebar sources and fails naming the file and the glyph that lacks a scene. It also fails for a scene with no moving part or an undefined keyframe. `scripts/test-frontend-changed.mjs` runs it on every frontend change.

- The scan covers `lib/staff-navigation.ts` and files under `components/` named `*sidebar*` or `*-nav-items.ts(x)`. Keep sidebar entries in a file with such a name, or extend the scan in the same PR.
- A red guard is fixed by adding the scene. The scan scope and assertions stay as strong as they are (`.claude/rules/no-test-modifications.md`).
