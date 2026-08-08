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

Imports are always by direct file path: `import { Button } from "~/components/ui/button"`. There is no `ui/index.ts` barrel — import every component from its own file.

## Study these first — canonical reference screens

Prose cannot transfer taste, density, spacing judgment, or restraint. Examples can. **Before building or changing any UI, open the relevant file(s) below and match their structure, density, spacing, color use, component choice, and restraint.** These are the bar for "looks like the rest of the app."

| Pattern | File | Why it's canonical |
|---|---|---|
| List / index page | `frontend/src/app/[tenant]/(protected)/staff/page.tsx` | `PageHeaderWithSearch` + filter chips + `DataTable`; compact and operational |
| Detail page | `frontend/src/app/[tenant]/(protected)/staff/[id]/page.tsx` | `ui/Tabs` + `detail-modal-components` field layout |
| Dense dashboard | `frontend/src/app/[tenant]/(protected)/time-tracking/page.tsx` | KPI/Saldo cards, charts, tables, modals — how a complex operational screen stays dense, not decorative (big file; study sections, don't read top-to-bottom) |
| Card primitive | `frontend/src/components/ui/info-card.tsx` | the canonical card surface |
| Table primitive | `frontend/src/components/ui/data-table.tsx` | the canonical table |

Keep this list current: if a reference is deleted or substantially rewritten, replace it with the new canonical example.

## Visual references — look at the real UI

Don't design from imagination. The app's real screens are captured as images you can open directly with the Read tool:

- **App screenshots** → `frontend/public/help/screens/*.webp` (~50 screens, maintained for the in-app help guide; treat as approximate-current). Open the relevant one before building or changing that screen. Anchors:
  - `mitarbeiter.webp` = staff list · `mitarbeitende-anlegen.webp` = staff create
  - `kinderdetailansicht.webp` = child detail · `kindersuche.webp` = student search/list
  - `meine-gruppen.webp` = groups · `aktivitaeten.webp` = activities · `feedback.webp` = feedback
  - `datenverwaltung.webp` = data admin · NFC kiosk = `nfc-*.webp`
- **Component gallery (Storybook)** → https://moto-nrw.github.io/design-system/ (or `pnpm storybook` in the sibling `../design-system` repo → :6006; project-phoenix itself has no storybook script). Good for component anatomy and states. Caveat: it renders the `@moto-nrw/design-system` **package** components, which share the visual language but are NOT the ones we import — use it for spacing/anatomy intuition, not as import targets.

Match what you see: density, neutral grays, brand-green accents used sparingly, no decorative hero cards.

## New UI is suspicious by default

Writing a new component is the exception, not the default. Every new card/button/table/modal/empty-state that could have reused the kit is drift. Before building one:

1. Search `ui/` and the canonical screens above for an existing fit.
2. If something fits, reuse it (extend via props/className, don't fork).
3. If nothing fits, STOP and state in the PR description **why** reuse doesn't fit and **what you added to the kit** instead of inlining a one-off.

An unexplained bespoke component is a review failure, not a style preference.

## Need X → use Y (do not hand-roll)

| You need… | Use | Import from |
|---|---|---|
| Button / CTA | `Button` — variants `primary` `secondary` `outline` `outline_danger` `danger` `success` `ghost`; sizes `sm` `base` `lg` `xl` (page-level), `md` (modal-footer / in-form action height, `px-4 py-2 text-sm`) + `compact` `icon` (flat dense chrome). Pass `type="button"` outside forms. | `~/components/ui/button` |
| Text input | `Input` | `~/components/ui/input` |
| Checkbox | `Checkbox` — brand-green (`#83CD2D`) checked state; wrap in your own `<label>` for the row | `~/components/ui/checkbox` |
| Inline alert / banner | `Alert` (`type`, `message`) | `~/components/ui/alert` |
| Modal dialog | `Modal`, `ConfirmationModal` | `~/components/ui/modal` |
| Form inside a modal | `FormModal` | `~/components/ui/form-modal` |
| Delete confirmation | `ConfirmDeleteModal` | `~/components/ui/confirm-delete-modal` |
| Multi-step wizard | `WizardStepper` | `~/components/ui/wizard-stepper` |
| Tabs (switching CONTENT panels) | `Tabs`, `TabsList`, `TabsTrigger`, `TabsContent` — `variant="default"` (pill) or `"line"` (underline) | `~/components/ui/tabs` |
| Segmented choice that is a VALUE, not a panel (mode picker, Monat/Woche, modal section switcher) | `SegmentedControl` — `variant="joined"` (bordered inline) or `"pills"` (tinted, per-item `tone`), `fullWidth` for modal tab bars | `~/components/ui/segmented-control` |
| Data / list table | `DataTable`, `DataTableStatusBadge` | `~/components/ui/data-table` |
| Info / stat card | `InfoCard`, `InfoItem` | `~/components/ui/info-card` |
| Detail-panel fields | `DataField`, `InfoSection`, `DataGrid`, `InfoText` | `~/components/ui/detail-modal-components` |
| Select dropdown (form value) | `CustomSelect`, `ListboxDropdown` (keyboard/ARIA listbox) | `~/components/ui/custom-select`, `~/components/ui/listbox-dropdown` |
| Date / range picker | `DatePicker`, date-range picker | `~/components/ui/date-picker`, `~/components/ui/date-range-picker` |
| Loading / skeleton | `Loading`, `Skeleton` | `~/components/ui/loading`, `~/components/ui/skeleton` |
| Avatar | `Avatar` | `~/components/ui/avatar` |
| Location / presence badge | `LocationBadge`, `PresenceBadge`, `StudentPresenceBadge` | `~/components/ui/location-badge`, etc. |
| Semantic status pill (fixed tone set) | `StatusBadge` — tinted pill + dot, `tone` = `blue` `green` `orange` `red` `gray` (brand hexes) | `~/components/ui/status-badge` |
| Data-driven status pill (raw hex) | `StatusDotBadge` | `~/components/ui/status-dot-badge` |
| Empty / no-results state | `EmptyState` — optional icon, title, description, action slot | `~/components/ui/empty-state` |
| Back navigation | `BackButton`, `MobileBackButton` | `~/components/ui/back-button`, `~/components/ui/mobile-back-button` |
| Overlay / side panel | `Drawer`, slide-over | `~/components/ui/drawer`, `~/components/ui/slide-over` |
| API error message text | `getApiErrorMessage` | `~/lib/api-error-message` |
| List/search page header | `PageHeaderWithSearch` | `~/components/ui/page-header/PageHeaderWithSearch` |
| Header nav tabs (sliding indicator, mobile dropdown) | `NavigationTabs` | `~/components/ui/page-header/NavigationTabs` |
| Kebab / overflow action menu | `OverflowMenu` | `~/components/ui/page-header/OverflowMenu` |
| Filter toggle button | `FilterButton` | `~/components/ui/page-header/FilterButton` |

If none fits, see **Kit gaps** below — extend the kit, don't inline a one-off.

## Canonical values

### Radius — set by the design-system `@theme`, NOT Tailwind defaults

`rounded-sm`=4px · `rounded-md`=8px · `rounded-lg`=12px · `rounded-xl`=16px · `rounded-2xl`=24px · `rounded-full`=9999px

- **Card / panel / floating surface wrapper** → the `.moto-content-surface` utility class (defined in `globals.css`: white bg, gray-200 border color, shadow-sm, backdrop blur, print-mode resets) combined with `rounded-2xl border p-4 shadow-sm sm:p-6` — this is what `InfoCard` renders and the dominant card surface across the app. The plain-Tailwind equivalent `rounded-2xl border border-gray-200 bg-white shadow-sm` also appears; prefer `moto-content-surface` for new cards. 24px = the design system's `--card-radius`. Do not invent another card shape, and never wrap content in a square `border-b`-only strip.
- Inner controls (buttons, inputs, pills) get their radius from the kit component. Don't sprinkle bare `rounded` (= 4px Tailwind default, off the brand scale).
- Component radius tokens: button & input = `--radius-md` (8px), card = `--radius-2xl` (24px), badge/pill = `--radius-full`.

### Colors — route every semantic color through `LOCATION_COLORS`

NEVER use a generic Tailwind color class (`text-green-500`, `bg-blue-500`, …) for a brand-semantic purpose; the Tailwind hues differ from the brand. Prefer the kit component that already encodes the color, then a `moto-*` utility (`bg-moto-green`, `text-moto-red-strong`), then `LOCATION_COLORS` / `MOTO_COLOR_PALETTE` in a `style` prop. Do NOT hardcode the raw hex in an arbitrary-value class: when the palette moves, a literal stays behind and silently drifts out of sync with the token beside it.

| Semantic | Hex | `LOCATION_COLORS` key |
|---|---|---|
| Brand green (primary) | `#83CD2D` | `GROUP_ROOM` |
| Brand blue | `#5080D8` | `OTHER_ROOM` |
| Neutral gray (Zuhause) | `#6B7280` | `HOME` |
| Orange (Schulhof) | `#F78C10` | `SCHOOLYARD` |
| Magenta (Unterwegs) | `#D946EF` | `TRANSIT` |
| Red (Krank / Fehler) | `#DC2626` | `SICK` / `DANGER` |
| Amber (Warnung) | `#EAB308` | `WARNING` |
| Purple (Entschuldigt) | `#7C3AED` | `EXCUSED` |
| Cyan (Klassenfahrt) | `#0891B2` | `CLASS_TRIP` |
| Navy (Kommt heute nicht) | `#365D83` | `NOT_ARRIVAL` |
| Stone (Unbekannt) | `#78716C` | `UNKNOWN` |

`SICK` and `DANGER` are the same hex on purpose — one names the child status,
the other the error semantic. `WARNING` is the amber "needs attention but is
not an error" hue (pending request, unstaffed slot, unexplained absence); it
used to be what `SICK` pointed at, so never treat amber as "sick".

**Two states shown together must never resolve to the same hex.** That is the
failure this table exists to prevent: a `Record` or `switch` that maps one
state to `SICK` and another to `DANGER` renders both identically. Check the
resolved hex, not the constant name.

Green CTA shades (from `GROUP_ROOM_SHADES` in `location-helper.ts`): base `#83CD2D`, hover `#74B825`, active `#6DB118`, text `#3F6F12`. Note: the kit `Button` `variant="primary"` is **gray-900**, not green — green CTAs use these hexes via arbitrary-value classes.

**Do NOT use the package color palette.** The `@moto-nrw/design-system` `@theme` ships a `steel` / `sage` / `warm` palette and `--color-brand-primary` (= sage `#7ba05b`). That is **not** the app's green. The app's brand green is `#83CD2D` (`LOCATION_COLORS.GROUP_ROOM` / the logo). Use `LOCATION_COLORS` for brand semantics and Tailwind `gray-*` for neutrals; never `bg-sage-*`, `bg-steel-*`, or `--color-brand-primary`, or you introduce a third, wrong green.

### Spacing & padding

Standard 4px scale — Tailwind `p-*` / `gap-*` / `m-*` map 1:1: `1`=4 · `2`=8 · `3`=12 · `4`=16 · `5`=20 · `6`=24 · `8`=32 · `10`=40 · `12`=48 · `16`=64 px. Canonical component padding (from the design-system component tokens):

- Card → `p-6` (24px); `p-4` compact, `p-10` large
- Button → page sizes `px-5 py-3`; `md` (modal/form action) `px-4 py-2 text-sm`; `compact` `h-8 px-2.5`
- Input → `px-4 py-3`

### Shadows

`shadow-sm` (cards / surfaces — the app default) · `shadow-md` (hover / raised) · `shadow-lg` / `shadow-xl` (overlays / modals). The canonical card uses `shadow-sm`.

## Kit gaps — extend the kit, don't inline a bespoke control

Compact / ghost / icon-only buttons now EXIST on `ui/Button` (`variant="ghost"`, `size="compact"`, `size="icon"`) — use those for dense toolbar/menu/icon chrome instead of hand-rolling. A modal-footer / in-form action height now EXISTS too: `size="md"` (`px-4 py-2 text-sm`) — use it for slide-over / modal footers and dense form actions instead of the oversized page sizes (`sm`/`base`/`lg`/`xl` are all `px-5 py-3`). A shared `Checkbox` (brand-green checked state) now EXISTS at `ui/checkbox` — never hand-roll a raw `<input type="checkbox">` with an ad-hoc accent color. A generic kebab/dropdown menu now EXISTS: `ui/page-header/OverflowMenu` covers action menus fully (portal-rendered, header/radio/separator entries, `href` link items incl. `external`, destructive tone, badges) — never hand-roll a `menuOpen` + click-outside menu again. Semantic status pills (`ui/status-badge`, tone-based, with an optional `title` for a hover tooltip) and empty states (`ui/empty-state`) now EXIST too. A segmented single-choice control EXISTS at `ui/segmented-control` — reach for it instead of hand-rolling a `<button>` cluster whenever the choice is a **value** (work mode, Monat/Woche, a modal's section switcher) rather than a content panel; `ui/Tabs` is Radix and stays the answer for real panels. Note the test consequence when picking between them: Radix tabs activate on **mousedown**, so a screen whose existing tests drive the switcher with `fireEvent.click` must use `SegmentedControl` (plain buttons), not `Tabs`. The kit still does **not** have a `Select` (native `<select>` styled to the kit `Input` look via the `moto-select` utility is the current convention). When you need a genuinely missing primitive, ADD it to `frontend/src/components/ui/` so the next screen reuses it — do not hand-roll a one-off `<button className="…">` inline. Call out the addition in the PR description.

## Gotchas

- **Kit `Tabs` is Radix-based.** In tests, select a tab with `fireEvent.mouseDown(tab, { button: 0 })`, **not** `fireEvent.click` — Radix activates on mousedown/focus, so a synthetic click does nothing. Mirror `src/components/database/detail-panel.test.tsx`.
- **`ui/Button` defaults `type="submit"`** — in a non-form context pass `type="button"`. The page-level sizes (`sm` `base` `lg` `xl`) are `rounded-lg px-5 py-3 shadow-md` and oversized for both dense chrome and modal footers; for toolbars, dropdown triggers, and icon actions use `size="compact"` / `size="icon"` (flat `h-8`, `rounded-md`) with `variant="ghost"`, and for **modal / slide-over footers and in-form actions** use `size="md"` (`px-4 py-2 text-sm`) instead of reaching for `size="sm"` (which is the same height as `base`).
- **`NavigationTabs` collapses to a dropdown on mobile** — use it for page-level navigation, not a compact segmented switcher. For a segmented switcher use `ui/Tabs` `variant="default"`.

## UI skills: this rule outranks all of them

The UI skills live in `frontend/.claude/skills/` and load only when an agent works in `frontend/`. Several of them overlap, so precedence is fixed: **this rule and the kit win; a skill's generic advice is the fallback where this rule is silent.** Run one review, not four. `better-interface` coordinates the six `better-*` skills and is the entry point for a full pass.

The three places where following the vendored skill literally produces wrong output here (two of them fail `pnpm run check`):

| Skill says | Here instead |
|---|---|
| `better-ui/surfaces.md`: prefer a `box-shadow` ring over a border for depth | Keep the canonical card surface: `.moto-content-surface` / `rounded-2xl border border-gray-200 bg-white shadow-sm`. Do not strip borders off kit surfaces. |
| `better-colors`: express colors as OKLCH tokens | Brand semantics come from `LOCATION_COLORS` hex only. New chromatic Tailwind utilities trip the `ui-kit/no-generic-brand-colors` ratchet. Convert notation only in an explicit, approved color-system migration. |
| `better-writing`: title case vs sentence case, English phrasing | All user-facing copy is German, with Umlauten. Take the structural advice (name the action in the label, errors next to the field, one clear action per empty state); match the wording of neighbouring screens. |

On motion, `ui-skills` is stricter than `better-ui/animations.md` and wins: no animation unless it was asked for, compositor properties only.

## Detection

**CI-enforced ratchet** (since #1629): the oxlint plugin `frontend/scripts/oxlint-plugin-ui-kit.mjs` fails `pnpm run check` on three drift patterns beyond its shrink-only, per-match baselines — `ui-kit/no-generic-brand-colors` (all chromatic Tailwind hues), `ui-kit/no-hand-rolled-overlay` (`fixed inset-0` across a complete `className` expression outside `ui/`), and `ui-kit/no-rounded-3xl` (off-scale card radius). Each legacy utility is tracked by value and count, so existing files cannot add violations. When you migrate a file, reduce its baseline; never increase one. Test/stories files are exempt (several assert on the legacy classes — the brand-hex fix for the kit primitives themselves is a deliberate rule change requiring test updates in the same PR).

```bash
# Components imported from the design-system package — should be ZERO (tokens only)
rg -n "from ['\"]@moto-nrw/design-system['\"]" frontend/src -g '*.tsx' -g '*.ts'

# Generic Tailwind brand-color utilities — prefer LOCATION_COLORS hex or a kit component
rg -n "(text|bg|border|ring|outline|fill|stroke|from|via|to|divide|accent|caret|decoration|shadow)-(red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-[0-9]" frontend/src -g '*.tsx'
```

## Mandatory visual check — do not skip

Code can be correct and the UI still feel wrong (spacing, density, decoration). For any change a school user would see, **run the app locally and look at the actual screen before calling the work done** — do not rely on the diff or on tests passing. Use the project's browser-verification setup (the `verify` / agent-browser flow against `{slug}.localhost:3000`), screenshot the changed screen, and compare it side by side with the nearest canonical screen above. If it doesn't sit naturally next to that screen, it isn't done.

## Design-review checklist

Apply before marking any UI work complete (and reviewers: before approving):

- [ ] Reuses kit components; any new component is justified in the PR description
- [ ] Colors come from `LOCATION_COLORS` / kit components — no generic Tailwind hues
- [ ] Radius, spacing, and density match the canonical screens (card = `rounded-2xl border border-gray-200 bg-white shadow-sm`)
- [ ] No marketing-style hero / decorative cards — it reads as an operational school tool
- [ ] Works on mobile
- [ ] Dense enough for daily operational use (not airy / landing-page)
- [ ] Visually verified: ran the app, screenshotted the changed screen, compared against a canonical screen

## When to deviate

Only with explicit reviewer approval recorded in the PR description, citing this rule and the reason. "It was faster to inline it" is not a reason — add the missing piece to the kit instead. The whole frontend pays for every bespoke one-off.
