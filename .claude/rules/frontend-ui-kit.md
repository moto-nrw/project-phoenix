# Frontend UI Kit — Default to the Shared Components

**RULE: All new frontend UI MUST be built from the shared kit in `frontend/src/components/ui/` (and `frontend/src/components/ui/page-header/`). Do NOT hand-roll a button, card, tab bar, modal, input, badge, table, or menu when a kit component exists. Do NOT import React components from the `@moto-nrw/design-system` package — that package is consumed for CSS tokens only.**

This kit is the de-facto MOTO design system for project-phoenix — the visual language the team converged on (much of it built by Florian / `fl0r14n28`). It encodes the correct radii, brand colors, shadows, and interaction patterns. Reusing it is how every screen ends up looking the same. Hand-rolling chrome with ad-hoc `rounded-*` and generic Tailwind colors is exactly the drift this rule exists to stop.

> The published `@moto-nrw/design-system` npm package ships components too, but project-phoenix is **not** on them yet — it only imports the package's CSS/tokens (`@import "@moto-nrw/design-system/tailwind"` in `globals.css`). Until the team explicitly migrates, the local `ui/` kit is the single source of truth. Don't import the package's components, and don't introduce a third component system.

This rule reinforces the "Reuse Existing Components" sections in `CLAUDE.md` and `frontend/CLAUDE.md` and the reuse-before-rebuild principle.

## Source of truth

| Concern | Location |
|---|---|
| UI components | `frontend/src/components/ui/` |
| Header / list-page kit | `frontend/src/components/ui/page-header/` |
| Semantic brand colors | `frontend/src/lib/location-helper.ts` → `LOCATION_COLORS` (the ONLY source) |
| Radius / spacing / shadow tokens | `@moto-nrw/design-system` via the `@theme` block `globals.css` pulls in |

Imports are usually by direct file path: `import { Button } from "~/components/ui/button"`. The `ui/index.ts` barrel only re-exports a subset (`input`, `button`, `alert`, `modal`, `password-change-modal`, `form-modal`, `wizard-stepper`); importing the other components by their file path is correct and expected.

## Need X → use Y (do not hand-roll)

| You need… | Use | Import from |
|---|---|---|
| Button / CTA | `Button` — variants `primary` `secondary` `outline` `outline_danger` `danger` `success`; sizes `sm` `base` `lg` `xl` | `~/components/ui/button` |
| Text input | `Input` | `~/components/ui/input` |
| Inline alert / banner | `Alert` (`type`, `message`) | `~/components/ui/alert` |
| Modal dialog | `Modal`, `ConfirmationModal` | `~/components/ui/modal` |
| Form inside a modal | `FormModal` | `~/components/ui/form-modal` |
| Delete confirmation | `ConfirmDeleteModal` | `~/components/ui/confirm-delete-modal` |
| Multi-step wizard | `WizardStepper` | `~/components/ui/wizard-stepper` |
| Tabs / segmented switcher | `Tabs`, `TabsList`, `TabsTrigger`, `TabsContent` — `variant="default"` (pill) or `"line"` (underline) | `~/components/ui/tabs` |
| Data / list table | `DataTable`, `DataTableStatusBadge` | `~/components/ui/data-table` |
| Info / stat card | `InfoCard`, `InfoItem` | `~/components/ui/info-card` |
| Detail-panel fields | `DataField`, `InfoSection`, `DataGrid`, `InfoText` | `~/components/ui/detail-modal-components` |
| Date / range picker | `DatePicker`, date-range picker | `~/components/ui/date-picker`, `~/components/ui/date-range-picker` |
| Loading / skeleton | `Loading`, `Skeleton` | `~/components/ui/loading`, `~/components/ui/skeleton` |
| Avatar | `Avatar` | `~/components/ui/avatar` |
| Location / presence badge | `LocationBadge`, `PresenceBadge`, `StudentPresenceBadge` | `~/components/ui/location-badge`, etc. |
| Back navigation | `BackButton`, `MobileBackButton` | `~/components/ui/back-button`, `~/components/ui/mobile-back-button` |
| Overlay / side panel | `Drawer`, slide-over | `~/components/ui/drawer`, `~/components/ui/slide-over` |
| API error message text | `getApiErrorMessage` | `~/components/ui/modal-utils` |
| List/search page header | `PageHeaderWithSearch` | `~/components/ui/page-header` |
| Header nav tabs (sliding indicator, mobile dropdown) | `NavigationTabs` | `~/components/ui/page-header/NavigationTabs` |
| Kebab / overflow action menu | `OverflowMenu` | `~/components/ui/page-header/OverflowMenu` |
| Filter toggle button | `FilterButton` | `~/components/ui/page-header/FilterButton` |

If none fits, see **Kit gaps** below — extend the kit, don't inline a one-off.

## Canonical values

### Radius — set by the design-system `@theme`, NOT Tailwind defaults

`rounded-sm`=4px · `rounded-md`=8px · `rounded-lg`=12px · `rounded-xl`=16px · `rounded-2xl`=24px · `rounded-full`=9999px

- **Card / panel / floating surface wrapper** → `rounded-2xl border border-gray-200 bg-white shadow-sm`. 24px = the design system's `--card-radius`; this is the canonical card (used 13×+ across the app and in `InfoCard`). Do not invent another card shape, and never wrap content in a square `border-b`-only strip.
- Inner controls (buttons, inputs, pills) get their radius from the kit component. Don't sprinkle bare `rounded` (= 4px Tailwind default, off the brand scale).

### Colors — route every semantic color through `LOCATION_COLORS`

NEVER use a generic Tailwind color class (`text-green-500`, `bg-blue-500`, …) for a brand-semantic purpose; the Tailwind hues differ from the brand. Use the kit component that already encodes the color, or the brand hex via arbitrary-value syntax (`bg-[#83CD2D]`).

| Semantic | Hex | `LOCATION_COLORS` key |
|---|---|---|
| Brand green (primary) | `#83CD2D` | `GROUP_ROOM` |
| Brand blue | `#5080D8` | `OTHER_ROOM` |
| Red | `#FF3130` | `HOME` |
| Orange | `#F78C10` | `SCHOOLYARD` |
| Magenta | `#D946EF` | `TRANSIT` |
| Amber | `#EAB308` | `SICK` |
| Purple | `#7C3AED` | `EXCUSED` |
| Gray (neutral) | `#6B7280` | `UNKNOWN` / `NOT_ARRIVAL` |

Primary action button green: base `#83CD2D`, hover `#74b827`, active `#669f21`.

## Kit gaps — extend the kit, don't inline a bespoke control

The kit does **not** yet have a compact / ghost / icon-only button or a generic `DropdownMenu`. When you need one, ADD it to `frontend/src/components/ui/` so the next screen reuses it — do not hand-roll a one-off `<button className="…">` inline. Call out the addition in the PR description.

## Gotchas

- **Kit `Tabs` is Radix-based.** In tests, select a tab with `fireEvent.mouseDown(tab, { button: 0 })`, **not** `fireEvent.click` — Radix activates on mousedown/focus, so a synthetic click does nothing. Mirror `src/components/database/detail-panel.test.tsx`.
- **`ui/Button` is page-level** (`rounded-lg px-5 py-3 shadow-md`) and defaults `type="submit"`. In a non-form context pass `type="button"`. For dense toolbar chrome it is oversized — prefer a compact variant (add one per **Kit gaps**) over reaching for raw Tailwind.
- **`NavigationTabs` collapses to a dropdown on mobile** — use it for page-level navigation, not a compact segmented switcher. For a segmented switcher use `ui/Tabs` `variant="default"`.

## Detection

```bash
# Components imported from the design-system package — should be ZERO (tokens only)
rg -n "from ['\"]@moto-nrw/design-system['\"]" frontend/src -g '*.tsx' -g '*.ts'

# Generic Tailwind brand-color utilities — prefer LOCATION_COLORS hex or a kit component
rg -n "(text|bg|border)-(red|green|blue|orange|purple|amber|yellow|emerald)-[0-9]" frontend/src -g '*.tsx'
```

## When to deviate

Only with explicit reviewer approval recorded in the PR description, citing this rule and the reason. "It was faster to inline it" is not a reason — add the missing piece to the kit instead. The whole frontend pays for every bespoke one-off.
