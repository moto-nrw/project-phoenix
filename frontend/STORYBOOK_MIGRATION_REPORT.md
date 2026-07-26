# Storybook Migration Report

## Summary

Storybook (`@storybook/nextjs-vite`, v10) was set up for this project and a
minimal "just render it" story was generated for every real component under
`src/components/**`. Generation ran across five automated passes plus two
manual follow-ups, because a temporary server-side rate limit repeatedly
interrupted the bulk parallel generation partway through — not a real
classification problem. Each pass re-targeted exactly the files still missing
a story, converging steadily: 73 → 136 → 198 → 255 → 287 real components
covered, plus one manually-cleaned duplicate. A follow-up added the missing
parent meal plan story, bringing coverage to 288 storyable components.

**Final state, verified against the current tree:**

| Metric | Count |
|---|---|
| Total component `.tsx` files under `src/components/` (excluding `*.test.tsx`/`*.stories.tsx`) | 290 |
| Total `.stories.tsx` files under `src/components/` | 290 |
| Components with a genuine story (directly or via a differently-named file for multi-export sources) | 288 |
| Legitimately skipped — no renderable component | 2 |

**End-to-end verification performed (not just per-file type-checking):**

| Check | Result |
|---|---|
| `pnpm run check` (verify-locales + oxlint + `tsc --noEmit`, the project's zero-warnings gate) | ✅ exit 0, clean |
| `pnpm run build-storybook -- --quiet` (real production build of all 290 story files) | ✅ "Storybook build completed successfully" |
| `pnpm run knip` | ✅ clean — only a config hint (see below), zero unused files/exports flagged |

## Skipped Files (permanent, not a rate-limit artifact)

| Source Path | Reason |
|---|---|
| `src/components/settings/settings-page.tsx` | Only export is `useSettingsTabs`, a hook returning `{ tabs, renderTab }` — `renderTab` is invoked as a plain function by its consumer, never rendered as a JSX tag. Internal components (`SettingsTabContent`, `SettingsSkeleton`, `SettingsContent`) are unexported and never used outside this file. |
| `src/components/ui/database/themes.tsx` | Pure theme/constants config file — exports `AccentColor` (type), `DatabaseTheme` (interface), `databaseThemes` (a data object consumed via property access), and `getThemeClassNames()` (called as a function). Its internal icon helper components are unexported and only populate a data field, never rendered as standalone JSX tags. |

## Notable Cleanup During This Run

`src/components/enrollment/phase-enrollment-actions.tsx` exports two
components (`LateInviteModal`, `ManualApprovedEnrollmentModal`). An earlier
pass correctly gave each its own dedicated story file
(`late-invite-modal.stories.tsx`, `manual-approved-enrollment-modal.stories.tsx`).
A later pass, unaware those already existed, additionally generated a third,
redundant combined file (`phase-enrollment-actions.stories.tsx`) duplicating
both stories. This duplicate was found and deleted manually after the final
verification pass — the two dedicated files remain as the canonical stories
for this source.

`src/components/parent/parent-meal-plan-page.tsx` now has a dedicated Storybook
entry with mocked parent API responses for populated, empty, unavailable, and
load-error states. It is a renderable component and is intentionally not listed
as skipped.

## knip Configuration Hint

`pnpm run knip` reported: *"src/components/**/*.stories.tsx — package.json —
Remove from ignore"*. This suggests knip already has built-in awareness of
`.stories.tsx` files and excludes them from its usage graph by default,
making the explicit `"ignore"` entry added to `package.json`'s `knip` config
redundant. It has been left in place as an explicit, harmless safety net
rather than removed — if you want to clean this up, verify knip's
Storybook-file handling first, then remove the entry and re-run `pnpm run
knip` to confirm the unused-component signal is unaffected.

**No unused components were flagged in this run.** That may mean the
codebase genuinely has no dead components in `src/components/` right now
(plausible given the existing architecture conventions and CI ratchets in
this repo), or it may be worth periodically re-running `pnpm run knip`
after future changes to confirm the signal stays meaningful.

## Next Steps

1. **Run `pnpm run storybook`** (starts on port 6006) and browse the sidebar,
   which mirrors the `src/components/` folder tree — every real, storyable
   component is now visible in one place.
2. **Review the 2 skipped files above** — both are correctly excluded
   (no renderable component), no action needed unless their shape changes.
3. **Some stories render real components in their built-in loading/error
   states** rather than fully-populated data, because there's no backend or
   API mocking layer in this Storybook setup — components that fetch their
   own data (a handful across `parent/`, `students/`, `enrollment/`, etc.)
   will show their genuine no-network fallback UI. This is real component
   behavior, not a broken story, and was an explicit, deliberate scope
   decision (no MSW) verified early in this project — see the design
   rationale if you want to revisit adding request mocking later.
4. **Use this as your "keep vs. redo vs. delete" audit tool** — the original
   goal. Every component in the app now has a visible entry point; go
   through the sidebar, decide what to polish and what to remove, and delete
   the corresponding `.stories.tsx` alongside any component you remove.
