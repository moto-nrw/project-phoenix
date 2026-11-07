---
paths:
  - "frontend/**"
---

# Frontend UI Kit — Default to the Shared Components

**RULE: All new frontend UI MUST be built from the shared kit in `frontend/src/components/ui/` (and `frontend/src/components/ui/page-header/`). Do NOT hand-roll a button, card, tab bar, modal, input, badge, table, or menu when a kit component exists. Do NOT import React components from the `@moto-nrw/design-system` package — that package is consumed for CSS tokens only.**

This kit is the de-facto MOTO design system for project-phoenix. It encodes the correct radii, brand colors, shadows, and interaction patterns. Hand-rolling chrome is the drift.

[ADR 0149](../../docs/adr/0149-ui-kit-entscheidet-jede-konkurrierende-variante-einmal.md) decides between competing kit variants, including pieces not built yet; read it before adding or migrating kit pieces.

## Source of truth

| Concern | Location |
|---|---|
| UI components | `frontend/src/components/ui/` |
| Header / list-page kit | `frontend/src/components/ui/page-header/` |
| Semantic brand colors | `frontend/src/lib/location-helper.ts` → `LOCATION_COLORS` (the ONLY source) |
| Radius / spacing / shadow tokens | `@moto-nrw/design-system` via the `@theme` block `globals.css` pulls in |

Imports are always by direct file path: `import { Button } from "~/components/ui/button"`. There is no `ui/index.ts` barrel.

## Page scaffolding — one shell, no per-page layout

**RULE: Every page under `frontend/src/app/[tenant]/(protected)` and every signed-in page under `frontend/src/app/{traeger,school,operator}` renders `TenantPage` (`~/components/ui/tenant-page`) as its root. A page supplies data, never layout.** There are no exemptions — the dashboard, the profile page and the emergency list run through the same scaffold as every list. The exemption set in `tenant-page-scaffold.test.ts` is empty and may only be added to with reviewer approval recorded in the PR.

`TenantPage` fixes the order of the parts so no page can decide it again:

```
Kopfkarte:  Titel  ..................................  Aktionen
            Statuszeile (echte Zahlen der Seite)
            Suche + Filter
Reiter:     horizontale Seitenreiter
Inhalt:     Karten im 24-px-Rhythmus, die letzte Fläche bis zur Unterkante
```

What that forbids in a page file: `max-w`, `mx-auto`, own root padding, a second `<main>`, a hand-written `<h1>`, a separate action row above or below the head, a row that only carries a counter or a single select, and a bespoke loading/empty/error branch. Those live in the scaffold.

**The body fills the screen.** The last kit surface in the body grows to the bottom edge (`.moto-tenant-body` in `globals.css`; the shell hands a flex column down to the content wrapper), so an empty page, a one-row table and a thirty-row list share one outline. Only a `section`/`div` surface grows, directly in the body or up to two wrappers deeper (page wrapper, `DataTable`), and every wrapper in between must itself be a flex column (`flex flex-col`); grids and clickable tiles never grow. Do not fake it per page with `min-h-*` — unwrap the block instead.

**No mini-heading above the title.** The blue uppercase kicker is gone from the tenant portal: it meant too many things at once. Where the user is comes from the breadcrumbs and the sidebar. `SectionCard`'s `kicker` prop survives for the parents portal, the NFC print guide and `/start` only.

**Status line, not explanation.** The line under the title carries figures the page already loads — „116 Kinder · 107 zuhause · 9 krank", „29.07.2026 bis 27.08.2026 · 22 Betreuungstage". Use `TenantPageStats` for value/label pairs and `statsLoading` for the skeleton. An explanatory sentence in the head is a rule violation (`moto-einfache-sprache`, rule 8). A figure a tile or card in the body already shows does not repeat in the status line; leave `stats` empty then. Text a person needs only to get to know the page goes into `TenantPage`'s `info` prop: an info button next to the title opens it as a popover (mouse hover, tap, click and keyboard, may carry a link), so it costs no card row in daily use (#4385, #5682). Text needed on every visit stays a `SectionCard`.

**One tab component per position.** Page-level tabs go through `TenantPage`'s `tabs` prop. `ui/Tabs` is only for tabs INSIDE a card (the substitution slide-over's Bearbeiten/Verlauf), `SegmentedControl` for every value choice — Monat/Woche/Tag, A/B-Woche, target group, export range, list filter. `ui-kit/no-tabs-as-value-switcher` fails a value switcher built from `ui/Tabs`.

**One card per job, all from the kit.** A content surface in a tenant page is never a hand-written class chain: `SectionCard` (with `title`, or without it for the plain surface), `TileCard` for anything clickable (`containsControls` when the tile carries its own buttons), `StatCard` for every figure, `TenantPageHeaderSkeleton` for a loading head. `ui-kit/no-hand-rolled-page-card` fails a page that spells the surface out again.

**One create button per surface, also when it is empty.** The create action stays in the head (`TenantPage` `actions`, `SectionCard` `action`), where it also sits when the list is filled. The empty state shows title and description without a second create button and may point to the head in text; on the database pages that includes the mobile FAB. `empty.action` / `EmptyState` `action` is only for a different step, such as „Filter zurücksetzen" while a search or filter is active (#5034). Outside production `TenantPage` logs `tenant_page_empty_action_repeats_head_action` when an empty-state control carries the name of a head control.

**One empty-value glyph.** An empty table cell or field shows „–" (en dash); `ui-kit/no-em-dash-empty-value` fails „—" alone. With room, say why the value is missing.

**Planning surfaces have no second head.** `PlanningContextBar` is the control band inside the head card (`searchSlot`), not a card of its own.

The full conversion spec, including what to do when a page's head lives in a view component, is `frontend/src/components/ui/TENANT-PAGE-SPEC.md`.

## Study these first — canonical reference screens

**Before building or changing any UI, open the relevant file(s) below and match their structure, density, spacing, color use, component choice, and restraint.** These are the bar for "looks like the rest of the app."

| Pattern           | File                                                           | Why it's canonical                                                                                                                                          |
| ----------------- | -------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| List / index page | `frontend/src/app/[tenant]/(protected)/staff/page.tsx`         | `PageHeaderWithSearch` + filter chips + `DataTable`; compact and operational                                                                                |
| Detail page       | `frontend/src/app/[tenant]/(protected)/staff/[id]/page.tsx`    | `ui/Tabs` + `detail-modal-components` field layout                                                                                                          |
| Dense dashboard   | `frontend/src/app/[tenant]/(protected)/time-tracking/page.tsx` | KPI/Saldo cards, charts, tables, modals — how a complex operational screen stays dense, not decorative |
| Page scaffold     | `frontend/src/components/ui/tenant-page.tsx`                   | the root every tenant page renders                                                                                                                          |
| Card primitive    | `frontend/src/components/ui/section-card.tsx`                  | the canonical card surface                                                                                                                                  |
| Table primitive   | `frontend/src/components/ui/data-table.tsx`                    | the canonical table                                                                                                                                         |

## Visual references — look at the real UI

Don't design from imagination. The app's real screens are captured as images you can open directly with the Read tool:

- **App screenshots** → `frontend/public/help/screens/*.webp` (~85 screens, captured for the old help guide; nothing maintains them since the help became text-only, so treat them as dated but indicative). Open the relevant one before building or changing that screen. Anchors:
  - `mitarbeiter.webp` = staff list · `mitarbeitende-anlegen.webp` = staff create
  - `kinderdetailansicht.webp` = child detail · `kindersuche.webp` = student search/list
  - `meine-gruppen.webp` = groups · `aktivitaeten.webp` = activities
  - `datenverwaltung.webp` = data admin · NFC kiosk = `nfc-*.webp`
- **Component gallery** → https://moto-nrw.github.io/design-system/. Renders the **package** components, not ours: anatomy only.

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
| Button / CTA | `Button` (actions), `ButtonLink` (navigation), `buttonClassName` (anything else that must look like a button) — rules in [Buttons](#buttons) below. Pass `type="button"` outside forms. | `~/components/ui/button` |
| Text input | `Input`, multiline `Textarea` (`autoGrow`): `required`/`aria-required` shows the star; `error` sets hint and `aria-invalid`; `type="password" revealable`: eye button | `~/components/ui/input`, `ui/textarea` |
| Required field | `required` on the kit control. A control without its own label gets `FieldLabel required`, a required group's `<legend>` ends in `RequiredMark`. Never a star in a label string or translation (#5217) | `~/components/ui/field-label` |
| Checkbox | `Checkbox` — gray-900 checked state; wrap in your own `<label>` for the row | `~/components/ui/checkbox` |
| Consent or confirmation a person ticks (Einwilligung, „verbindlich“) | `ConsentCheckbox` — `required`, `error`, `action` for the legal text button; both public Anmeldungen use it | `~/components/ui/consent-checkbox` |
| Inline alert / banner (one notice) | `Alert` (`type`, `message`); a notice that needs attention is `type="warning"` (orange), never an amber box; several notices of the same kind follow [Several notices of the same kind](#several-notices-of-the-same-kind) | `~/components/ui/alert` |
| Rows with their own actions in a desktop column | `ResponsiveActionList` | `~/components/ui/responsive-action-list` |
| Transient feedback | `Toast` — presentational surface only; queue, lifetime, and placement stay in `ToastProvider` | `~/components/ui/toast` |
| Modal dialog | `Modal`, `ConfirmationModal` (header `icon`, `description`) | `~/components/ui/modal` |
| Form inside a modal | `FormModal` (built on `Modal`) | `~/components/ui/form-modal` |
| Delete confirmation (every deletion; `bauart/one-delete-confirm` fails a `ConfirmationModal`, hand-built `Modal` or `window.confirm` for Löschen) | `ConfirmDeleteModal` — gate `twoStep`, or `textConfirm` for irreversible data loss; a series/child scope choice goes in its `scope` slot, not in a `ChoiceModal` in front | `~/components/ui/confirm-delete-modal` |
| Multi-step wizard | `WizardStepper` | `~/components/ui/wizard-stepper` |
| Footer of an edit state (Abbrechen + Speichern, saving label, disabled while the draft is invalid) | `EditActions` — the only save affordance outside settings; auto-save on blur/change is ratcheted to zero by `bauart/no-autosave`. An object is edited in its tab, never in an overlay titled „… bearbeiten“ / „… verwalten“ (`bauart/no-edit-overlay`, #3116) | `~/components/ui/edit-actions` |
| Tabs (switching CONTENT panels) | `Tabs`, `TabsList`, `TabsTrigger`, `TabsContent` — `variant="default"` (pill) or `"line"` (underline) | `~/components/ui/tabs` |
| Segmented choice that is a VALUE, not a panel (mode picker, Monat/Woche, modal section switcher) | `SegmentedControl` — `variant="joined"` (bordered inline) or `"pills"` (tinted, per-item `tone`), `fullWidth` for modal tab bars | `~/components/ui/segmented-control` |
| Data / list table; `expandable`, `renderSubRows`, `summaryRow`; column key `actions` stays pinned | `DataTable`, `DataTableStatusBadge` | `~/components/ui/data-table` |
| Key figure tile | `StatCard` — default `variant="card"`; `inline` for dense grids in cards | `~/components/ui/stat-card` |
| Icon tile (symbol on a grey or tinted square/circle) | `IconTile` (`size`, `shape`, `tone`); a concept: `ConceptIconTile` | `~/components/ui/icon-tile` |
| Any card: page section with heading, explanation, actions, or a plain surface | `SectionCard`, the only card head: `concept`/`icon`/`leading`, `collapsible`, `flush`, `footer`, `padding="none"`, `loading`; `kicker`: ADR 0149 | `~/components/ui/section-card` |
| Any other framed white container (list wrapper, inner card, skeleton) | the `moto-content-surface` utility + `rounded-2xl border` (inner cards keep `rounded-xl`); never `border-gray-200 bg-white` by hand | `globals.css` |
| Floating panel at a control, with content | `AnchoredPopover` — `header`, `footer`, `align`, `focusPanelOnOpen`, `swallowOutsideClick` | `~/components/ui/anchored-popover` |
| Count on an icon or nav row | `NotificationBadge` — `tone`, `size` | `~/components/ui/notification-badge` |
| Selectable row or tile (label around a Checkbox/Radio, mode-picker button, row with a selection state) | `ChoiceTile` — `as` = `label` (default) / `button` / `div`, `selected`, `disabled`, `tone` = `gray` `green` `blue`; padding via `className` | `~/components/ui/choice-tile` |
| Label/value fields | `DataField` in a `dl`, `InfoSection`, `DataGrid`, `InfoText` | `~/components/ui/detail-modal-components` |
| Select | `CustomSelect`, `MultiSelect`; own content: `ListboxDropdown` | `~/components/ui/<kebab-name>` |
| Weekday choice | `WeekdayToggleGroup` | `~/components/ui/weekday-toggle-group` |
| Chosen entry with a remove X; read-only without `onRemove` | `RemovableChip` | `~/components/ui/removable-chip` |
| Number with minus and plus (except time tracking's break stepper) | `NumberStepper` | `~/components/ui/number-stepper` |
| Day strip on a phone | `PlanningDayChip stacked` (`marker`, `statusLabel`, `today`) | `~/components/ui/planning/planning-context-bar` |
| Date / range picker | `DatePicker`, `DateRangePicker` (selection gray-900, today blue); with time: `DateTimePicker` | `~/components/ui/<kebab-name>` |
| Loading | Area: `Loading fullPage={false}`; beside text or in icon button: `SpinnerIcon`; text button: `Button isLoading loadingText`; known shape: `SectionCard loading`, `page-skeletons` in `SkeletonRegion`, `Skeleton` (`shape`), `SkeletonRows` | `~/components/ui/`: `loading`, `icons`, `button`, `page-skeletons`, `skeleton` |
| Avatar | `Avatar` | `~/components/ui/avatar` |
| File drop or choice | `Dropzone`; caller checks beyond `accept` | `~/components/ui/dropzone` |
| Key or link to copy | `CopyField`; button only: `useClipboardCopy` | `~/components/ui/copy-field` |
| Location / presence badge | `LocationBadge`, `PresenceBadge`, `StudentPresenceBadge` | `~/components/ui/location-badge`, etc. |
| Status pill | `StatusBadge` (#5584) | `~/components/ui/status-badge` |
| Pill in a data colour (amber: `WARNING`) | `StatusColorBadge` | `~/components/ui/status-color-badge` |
| Colour dot, only in legends and plans | `StatusDot` — hex `color` or `bg-…` in `className`, `size`; `label` (no text beside it) or `decorative` | `~/components/ui/status-dot` |
| Colour bar marking a row's type | `ColorBar` — `color`, `size` | `~/components/ui/color-bar` |
| Progress bar | `ProgressBar` — `label`, `tone` or hex `color`, `size` | `~/components/ui/progress-bar` |
| Legend, also with counts | `PlanLegend` — entries with `value`, `variant="dot"`, `size="md"` | `~/components/ui/planning/plan-legend` |
| Empty / no-results state | `EmptyState` — optional icon, title, description, action slot | `~/components/ui/empty-state` |
| Back navigation (Unterseite → Sammlung) | `TenantPage` mit `back` / `backHref` / `backLabel`; außerhalb des Gerüsts `TenantBackLink` (Tenant) oder `BackLink` (anderes Portal, fertiger Pfad). Ein Link mit Pfeil und Ziel als Text („Zurück zu den Räumen"), auf jeder Breite, über dem Titel; nie ein eigener Knopf im Inhalt (#4364). Eine Ansicht ohne eigene Adresse (Editor → Übersicht derselben Seite) nimmt `BackAction`, denselben Rückweg als Knopf (#4397) | `~/components/ui/tenant-page`, `~/components/ui/back-link` |
| Portal sidebar, bottom bar | `PortalSidebar`, `PortalSidebarFrame`; `PortalBottomNav`. Lint: `no-hand-rolled-portal-nav` | `~/components/ui/portal-sidebar`, `~/components/ui/portal-bottom-nav` |
| Icon in a sidebar row (any portal) | `MotoNavIcon` with `scene` / `NavSceneIcon`: the glyph's animated scene; a glyph new to a sidebar needs a scene: `.claude/rules/sidebar-icon-scenes.md` | `~/components/ui/moto-nav-icon`, `~/components/ui/nav-scene-icon/nav-scene-icon` |
| Link in the app shell (sidebar, bottom nav, breadcrumbs, header menus) | `NavLink` — `next/link` without viewport prefetch; prefetches on hover, focus, or touch start instead (#2976). | `~/components/ui/nav-link` |
| Interner Link im Seiteninhalt | `NavigationLink` — meldet den Wechsel an die Portalhülle und verwirft die Meldung bei abgebrochenem Klick. `ButtonLink` nutzt dieselbe Meldung bei String-Zielen. Plain `next/link` bleibt richtig für externe Ziele, Downloads, neue Tabs und Bereiche ohne `PortalShell`. | `~/components/ui/navigation-link` |
| Rückmeldung, dass ein Seitenwechsel läuft | `NavigationProgressBar` / `NavigationProgressProvider` — steckt bereits in `PortalShell`; `NavLink`, `NavigationLink` und `ButtonLink` melden den ausstehenden Wechsel selbst (#2828). Nicht nachbauen, keine eigene Ladehülle über mehrere Seiten. | `~/components/ui/navigation-progress` |
| Overlay / side panel | `Drawer`, slide-over; levels: `Z_LAYER` (`overlay-styles`) | `~/components/ui/drawer`, `~/components/ui/slide-over` |
| Failed API call shown to the user | `useApiErrorDisplay` (toast), `useApiFormError` + `FormErrorAlert`, `useApiLoadError` + `LoadErrorAlert`; rules in `.claude/rules/error-path.md` | `~/contexts/ToastContext`, `~/components/ui/form-error-alert` |
| List/search page header | `PageHeaderWithSearch` | `~/components/ui/page-header/PageHeaderWithSearch` |
| Header nav tabs (sliding indicator, mobile dropdown) | `NavigationTabs` | `~/components/ui/page-header/NavigationTabs` |
| Kebab / overflow action menu (every row action of a list item goes here; `bauart/no-row-action-buttons` fails a per-row `Button` named Bearbeiten/Löschen/Archivieren/…); menu behind a text button | `OverflowMenu` — entry `description`, `selected`; `triggerButton` + `triggerContent` for a kit button, `wrapperClassName` | `~/components/ui/page-header/OverflowMenu` |
| Filter toggle button | `FilterButton` | `~/components/ui/page-header/FilterButton` |
| Stammdaten-Katalog einer Schule (Kategorien, Planungsspuren, Schichtarten, Abwesenheitsarten): Sammlung + Objektbearbeitung | `CatalogPage` auf einer eigenen Route unter `/database/…`; ein Formular, dem ein Eintrag fehlt, trägt nur die Auswahl und `CatalogManageLink`. Nie ein Slide-over, Modal oder Auswahlfeld — `bauart/no-manage-surface-in-overlay` fällt darauf (#3114). | `~/components/database/catalog/catalog-page`, `~/components/database/catalog/catalog-manage-link` |

If none fits, see **Kit gaps** below — extend the kit, don't inline a one-off.

## Buttons

Decided in #5529, enforced since #5531. Not up for renegotiation: if code cannot meet a rule, comment on the ticket instead of deviating.

**Every action uses the kit.** `className` on a kit button carries layout only (`w-full`, `flex-1`, `gap-*`, `mt-*`, `shrink-0`, `self-*`, `hidden md:flex` …), never height, padding, radius, font size/weight, background, text/ring/shadow colour or `scale`. A missing look is a new kit variant.

| Size | Height (mouse) | Use for |
|---|---|---|
| `md` | `px-4 py-2 text-sm` | **Default:** `TenantPage` head (sets its own height), `SectionCard` heads, modal/slide-over footers, forms |
| `field` | 40 px (`h-10`, = `Input controlSize="compact"`) | Auth forms; a button in the same row as a 40-px field |
| `compact`, `icon` | 32 px | Dense toolbars, table rows, icon actions |
| `sm`, `base`, `lg`, `xl` | `px-5 py-3` | Only one large action standing alone (submit an enrolment) |

| Variant | Use for |
|---|---|
| `primary` (gray-900), `outline`, `surface`, `ghost` | Neutral actions |
| `ghost_tone` | Quiet action on a coloured notice (toast, alert): takes the notice's text colour |
| `success` | Green confirm, every import submit |
| `danger`, `outline_danger`, `ghost_danger` (quiet red) | Destructive |
| `warning` | Proceed despite a side effect (orange) |
| `link` | Text button, no padding, visible focus ring |
| `fab` | Floating create button, round, 56 px, green |
| `dashed` | Dashed add surface |

**No grey buttons (#5574)**, no light-grey fill at rest. Value choice: `SegmentedControl`; on/off: `ToggleChip`; removal: `outline_danger`.

**Finger rule.** Under `pointer: coarse` every kit button is at least 44 px high, `icon` 44 × 44 px; with a mouse nothing changes. The kit does this for every portal; no `min-h-11` overrides, no `touch` size.

**Defaults.** No `scale` animation. `Alert` actions are `compact`. Equal actions on one surface share size and variant. Every overlay close X is `ghost` `icon`; on a coloured notice (toast) `ghost_tone` `icon`.

**Deliberate exceptions (do not align):** the stamp buttons and the break stepper −/+ of time tracking; `global-error` (inline styles, loads no CSS; only colour, radius and weight follow kit `primary`); event rows and month chips in the parents calendar. **Not buttons in this sense:** list and selection rows, tiles (`TileCard`), tabs, `SegmentedControl`, checkboxes, switches, menu entries, navigation rows, steppers inside fields.

**Ratchet.** `ui-kit/no-hand-rolled-button` fails a native `button`/`a`/`Link`/`NavLink`/`NavigationLink`/`role="button"` with background or ring/border colour plus radius plus padding or height; `ui-kit/no-button-override` fails such classes on `Button`/`ButtonLink` or in `buttonClassName({ className })`. Findings in `frontend/scripts/ui-kit-button-baseline.json`; extras fail, removals require lowering it (`pnpm run ui-kit:button-baseline`). Exceptions with reasons: `ui-kit-button-baseline.mjs`. `ui-kit/no-grey-button-fill` (zero) fails resting `bg-gray-100`/`-200`/`-300`; exempt: switches, tabs, menus, `aria-pressed` choices, nav rows.

## Several notices of the same kind

Use when one screen shows two or more notices of the same kind, each about its own object (offerings without a Regeltermin, Regeltermine the system did not adjust). One notice stays an `Alert`; notices of different kinds keep their own block.

Build it from existing kit parts, as Tristan Heitger and Florian (`fl0r14n28`) set them up; add no wrapper component:

1. **One neutral section.** A `SectionCard` holds all entries as rows in `ResponsiveActionList` (`divide-y divide-gray-100`). Colour stays on the icon (`leading`) and on a count that names a fact („3 offen“); the rows are white, never one coloured `Alert` per entry.
2. **Actions in one flush column.** From `sm` on, every row's actions start at the same place (rows take `sm:col-span-2 sm:grid sm:grid-cols-subgrid`); below `sm` they sit under the text, side by side from 480 px and one per line below. Actions of equal rank look the same: `Button` or, for navigation, `ButtonLink`, both `variant="surface" size="compact"`, with an `aria-label` naming the row's object. A text link next to a button is no equal-rank action.
3. **Shared text once.** A hint that applies to several rows stands once above the list, also when only one row needs it, never repeated per row.
4. **Only concrete labels.** Show what OGS staff can act on (name, children, days). Skip markers or badges that carry no concrete information.
5. **Long lists fold.** More than 5 open rows: show the first 5 and a `ghost` button „N weitere anzeigen“ / „Weniger anzeigen“ with `aria-expanded`. Decided or done entries go into a second `SectionCard` with `collapsible defaultCollapsed`.
6. **Load error stays small.** `LoadErrorAlert` with `actionLayout="responsive"` puts „Wiederholen“ right of the message from `sm` on and under it on a phone; never an empty list instead of the error. Exceptions in [`form-error-alert.tsx`](../../frontend/src/components/ui/form-error-alert.tsx): an error with a request ID stays `stacked` (retry and ID under the message), a retry shows only when the error carries one, and a module the school did not book shows its explanation without a retry.

Reference implementations: [`offering-gap-hints.tsx`](../../frontend/src/components/timetable/offering-gap-hints.tsx) (#5181, all six points) and [`care-plan-impact.tsx`](../../frontend/src/components/students/care-plan-impact.tsx) (#5298, shared hint above the list, equal `surface compact` buttons).

Review check: with two or more entries that carry actions, the screen shows one card, no `Alert` per entry, a flush action column from `sm` on, actions under the text below `sm`, and no repeated hint text. The before/after pairs for desktop and phone follow [Before/After Screenshots](../../frontend/CLAUDE.md#beforeafter-screenshots-for-ui-changes-mandatory).

## Canonical values

### Radius — set by the design-system `@theme`, NOT Tailwind defaults

`rounded-sm`=4px · `rounded-md`=8px · `rounded-lg`=12px · `rounded-xl`=16px · `rounded-2xl`=24px · `rounded-full`=9999px

- **Card / panel wrapper** → `SectionCard`, which renders the `.moto-content-surface` utility class (defined in `globals.css`: white bg, gray-200 border color, shadow-sm, backdrop blur, print-mode resets) with `rounded-2xl border p-5 max-sm:p-4`. 24px = the design system's `--card-radius`. Do not invent another card shape, and never wrap content in a square `border-b`-only strip.
- Inner controls (buttons, inputs, pills) get their radius from the kit component. Don't sprinkle bare `rounded` (= 4px Tailwind default, off the brand scale).
- The code is the source (ADR 0149): controls (input, select, button) `rounded-lg` (12px; 32-px `compact`/`icon` buttons and the checkbox box `rounded-md`), inner surfaces and popovers `rounded-xl` (16px), cards `rounded-2xl` (24px), badge/pill `rounded-full`.

### Colors — route every semantic color through `LOCATION_COLORS`

NEVER use a generic Tailwind color class (`text-green-500`, `bg-blue-500`, …) for a brand-semantic purpose; the Tailwind hues differ from the brand. Prefer the kit component that already encodes the color, then a `moto-*` utility (`bg-moto-green`, `text-moto-red-strong`), then `LOCATION_COLORS` / `MOTO_COLOR_PALETTE` in a `style` prop. Do NOT hardcode the raw hex in an arbitrary-value class: a literal drifts when the palette moves.

| Semantic | Hex | `LOCATION_COLORS` key |
|---|---|---|
| Brand green (primary) | `#83CD2D` | `GROUP_ROOM` |
| Brand blue | `#5080D8` | `OTHER_ROOM` |
| Neutral gray (Zuhause) | `#6B7280` | `HOME` |
| Petrol (Schule) | `#217A78` | `AT_SCHOOL` |
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

Green shades (from `GROUP_ROOM_SHADES` in `location-helper.ts`): base `#83CD2D`, hover `#74B825`, active `#6DB118`, text `#3F6F12`, fill `#4A7F14`. **An opaque green surface with text or an icon carries white on the fill steps** (#5141): `bg-moto-green-fill text-white hover:bg-moto-green-fill-hover active:bg-moto-green-fill-active` (4.84 / 6.0 / 7.0 : 1), or `Button variant="success"`, which already does this. Base, hover, active, light and vivid are for indicators without text (dots, bars, borders): white on `#83CD2D` is 1.96:1. Tinted surfaces (`bg-moto-green/10`, `-soft`) keep dark or `text-moto-green-strong` text. The ratchet `ui-kit/green-fill-white-text` fails dark text on any opaque green and white text on the light greens. Note: the kit `Button` `variant="primary"` is **gray-900**, not green — green CTAs use `variant="success"`, the `moto-*` utilities or `LOCATION_COLORS` / `GROUP_ROOM_SHADES`, not copied hex classes.

**Do NOT use the package color palette.** The `@moto-nrw/design-system` `@theme` ships a `steel` / `sage` / `warm` palette and `--color-brand-primary` (= sage `#7ba05b`). That is **not** the app's green. The app's brand green is `#83CD2D` (`LOCATION_COLORS.GROUP_ROOM` / the logo). Use `LOCATION_COLORS` for brand semantics and Tailwind `gray-*` for neutrals; never `bg-sage-*`, `bg-steel-*`, or `--color-brand-primary`.

### Concept icons — one symbol per domain concept

Each concept's definition in `lib/concepts/<key>.ts` (collected in `MOTO_CONCEPTS`, `lib/moto-concepts.ts`) is the single source for which symbol and tone it gets. Render it with `<MotoConceptIcon concept={childrenConcept} />` (`~/components/ui/moto-concept-icon`), importing the definition from its own module so a page carries only the glyphs it shows; production code never imports the `MOTO_CONCEPTS` registry at runtime (`moto-concepts.test.ts` fails on it). Never import the Phosphor glyph and pick a tone by hand. The people concepts:

| Concept     | Key        | Phosphor glyph           | Tone         |
| ----------- | ---------- | ------------------------ | ------------ |
| Kind        | `children` | `UserIcon` (one head)    | `greenVivid` |
| Eltern      | `parents`  | `UsersIcon` (two heads)  | `blue`       |
| Gruppe      | `groups`   | `UsersThreeIcon` (three) | `greenDeep`  |
| Personal    | `staff`    | `IdentificationCardIcon` | `orange`     |
| Klassentag  | `classDay` | `ChalkboardTeacherIcon`  | `blue`       |

- One head always means a child, two heads parents, three heads a group, also where the icon is monochrome (sidebar entries, tab triggers, help tiles in `HELP_ICONS`): keep the glyph, drop the tone.
- Deliberate exceptions: the profile menu (`UserIcon` = the signed-in user) and the neutral gray meta icons on the child card (`tone="neutral"`, same glyph as the concept).
- Need a new concept? Add a module under `lib/concepts/` and its entry in `MOTO_CONCEPTS`; don't define a one-off glyph/tone pair.

### Spacing & padding

Standard 4px scale — Tailwind `p-*` / `gap-*` / `m-*` map 1:1: `1`=4 · `2`=8 · `3`=12 · `4`=16 · `5`=20 · `6`=24 · `8`=32 · `10`=40 · `12`=48 · `16`=64 px. Canonical component padding (from the design-system component tokens):

- Card → `p-5` (20px), `p-4` on a phone (`SectionCard`)
- Button → [Buttons](#buttons)
- Input → `px-4 py-3`

### Shadows

`shadow-sm` (cards / surfaces — the app default) · `shadow-md` (hover / raised) · `shadow-lg` / `shadow-xl` (overlays / modals). The canonical card uses `shadow-sm`.

## Kit gaps — extend the kit, don't inline a bespoke control

Buttons, `Checkbox`, `OverflowMenu`, `StatusBadge`, `EmptyState`, `SegmentedControl` and the selects (no styled native `<select>`) all EXIST — see the table above. A genuinely missing primitive goes into `frontend/src/components/ui/`, not inline; call it out in the PR description.

## Gotchas

- **Kit `Tabs` is Radix-based.** In tests, select a tab with `fireEvent.mouseDown(tab, { button: 0 })`, **not** `fireEvent.click` — Radix activates on mousedown/focus, so a synthetic click does nothing. Mirror `src/components/database/detail-panel.test.tsx`.
- **`ui/Button` defaults `type="submit"`** — in a non-form context pass `type="button"`. `size="sm"` is the same height as `base` (`px-5 py-3`), not a small button; the normal action size is `md` ([Buttons](#buttons)).
- **The last surface of a `TenantPage` grows to the bottom of the screen.** The unlayered `.moto-tenant-body` rule in `globals.css` sets `flex: 1 0 auto` on it and its wrappers, which beats any `flex-1 min-h-0` utility. A surface whose height comes from a wrapper and that scrolls inside must carry `moto-scroll-surface`, the documented exit from the rule. Otherwise it grows to its full content height and the fixed-height wrapper clips it or it spills past the wrapper (#3328, #3330). Current users: the chats (`useChatViewportLock`), and `MasterDetailLayout` and `DatabaseListLayout` on desktop (`useFillHeight`). Surfaces nested inside a scroll surface do not count for the rule, so its wrappers keep their own `flex-direction`. On a phone, prefer page scrolling: no fixed height, no exit, the card grows with its list (both database layouts do this below `md`). Do not work around the rule with inline `flex` styles. Browser check: `pnpm run test:layout` (`frontend/e2e/layout/`); add a case there for every new user of the exit.
- **`NavigationTabs` collapses to a dropdown on mobile**: page navigation only; value switcher: `SegmentedControl`.

## UI skills: this rule outranks all of them

The UI skills live in `frontend/.claude/skills/` and load only when an agent works in `frontend/`. Several of them overlap, so precedence is fixed: **this rule and the kit win; a skill's generic advice is the fallback where this rule is silent.** Run one review, not four. `better-interface` coordinates the six `better-*` skills and is the entry point for a full pass.

The three places where following the vendored skill literally produces wrong output here (two of them fail `pnpm run check`):

| Skill says | Here instead |
|---|---|
| `frontend/.claude/skills/better-ui/surfaces.md`: prefer a `box-shadow` ring over a border for depth | Keep the canonical card surface: `SectionCard`. Do not strip borders off kit surfaces. |
| `better-colors`: express colors as OKLCH tokens | Brand semantics come from `LOCATION_COLORS` hex only. New chromatic Tailwind utilities trip the `ui-kit/no-generic-brand-colors` ratchet. Convert notation only in an explicit, approved color-system migration. |
| `better-writing`: title case vs sentence case, English phrasing | All user-facing copy is German, with Umlauten. Take the structural advice (name the action in the label, errors next to the field, one clear action per empty state); match the wording of neighbouring screens. |

On motion, `ui-skills` is stricter than `frontend/.claude/skills/better-ui/animations.md` and wins: no animation unless it was asked for, compositor properties only.

## Detection

**CI-enforced ratchet** (since #1629): the oxlint plugin `frontend/scripts/oxlint-plugin-ui-kit.mjs` fails `pnpm run check` on drift patterns, first these five — `ui-kit/no-generic-brand-colors` (all chromatic Tailwind hues), `ui-kit/no-hand-rolled-overlay` (`fixed inset-0` across a complete `className` expression outside `ui/`), `ui-kit/no-hand-rolled-surface` (unprefixed `rounded-xl/2xl` + `border` + `bg-white` on one element, also across conditional branches; use `moto-content-surface`, `AnchoredPopover`, `ChoiceTile` or a kit surface component — #2933), `ui-kit/no-rounded-3xl` (off-scale radius, `rounded-t-3xl` too), and `ui-kit/require-checkbox-label` (unlabeled shared checkboxes; a Checkbox inside a label-shaped `ChoiceTile` counts as labeled). These five are hard-zero. A deliberate non-card match (white pill) may stay behind `{/* oxlint-disable-next-line ui-kit/no-hand-rolled-surface -- <reason> */}` with the reason in the PR description; no baseline. Test/stories files are exempt from the class-string rules.
`bauart/*` exempts only the parent portal (#5604). The tenant page scaffold (#2619) adds four more rules in the same plugin: `ui-kit/no-tenant-kicker` (no mini-heading outside the parent portal), `ui-kit/no-hand-rolled-page-card` (`moto-content-surface` + `rounded-2xl` spelled out in a portal page/loading file instead of a kit card), `ui-kit/no-tabs-as-value-switcher` (`ui/Tabs` carrying a value choice), and `ui-kit/no-tiny-text` (sub-12px text utilities; shrink-only per-file baseline). `ui-kit/green-fill-white-text` (#5141, hard-zero) keeps text on opaque green surfaces white and on the dark fill steps. #5577: `no-dynamic-color-class`, `no-white-on-warm-fill` (white on orange only `-strong`). #5531: button ratchet, see [Buttons](#buttons). #5579: `no-shadcn-tokens`, `no-raw-z-index` (`z-[n]`). #5580: `no-select-override` (no look classes on selects, `DateRangePicker`). #5581: `no-important-override` (`!` on kit components, baseline), `no-section-card-bleed`. #5605: `no-control-height-override` (`h-8`/`h-9`/`!size-*`). #5582: `no-hand-rolled-icon-tile` (also in `ui/`). #5583: `no-shadow-kit-name` (no local name equal to a kit export). #5584 (hard-zero): `no-hand-rolled-pill`, `no-pill-dot`, `no-hex-alpha-tint`. #5596 (hard-zero): `no-hand-rolled-weekday-toggle` (`.map` over weekdays returning a checkbox label or pressed button), `no-hand-rolled-removable-chip` (chip with its own X), `no-hand-rolled-number-stepper` (minus/plus around a field). #5599: `no-hand-rolled-dropzone` (`onDrop`), `no-hand-rolled-copy-field` (framed `<code>`, `navigator.clipboard`). `ui-kit/no-raw-input` (#5217, hard-zero) fails a hand-built `<input>`/`<textarea>` except in the primitives of `RAW_INPUT_KIT_PRIMITIVES`; `type="hidden"` and `type="file"` stay native. A field with no kit counterpart (OTP grid) stays behind `oxlint-disable-next-line ui-kit/no-raw-input -- <reason>`. A form jumps to its first failed field only through `FormErrorAlert`; there is no second scroll hook.

`ui-kit/no-radix-outside-kit` (#5604, hard-zero): `@radix-ui/*` only under `components/ui/`.

`ui-kit/no-raw-table` (#5600, hard-zero): no `<table>` outside `ui/`.

`ui-kit/no-kit-domain-import` (#5586, hard-zero): `components/ui/` imports no other component folder and calls no `fetch`/`useSWR*`; context comes through props and slots. Planning pieces live in `ui/planning/`. `no-hand-rolled-surface` applies in `ui/` too.

`ui-kit/no-hand-rolled-menu` (#5601, hard-zero): `role="menu|menuitem*|listbox|option"` only in `ui/`; `ui-kit/no-hand-rolled-popover` (#5601, hard-zero): `moto-popover-surface` only in `ui/`.

`ui-kit/no-own-spinner` (#5588, hard-zero): no `animate-spin` or icon-package spinner outside `ui/`. `bauart/no-own-skeleton` also fails `<Skeleton>` rows in an own surface and radius classes on it. The scaffold test fails visible loading text in `page.tsx`.

`ui-kit/no-hand-rolled-dot` fails a hand-built dot or colour bar outside `ui/` (hard-zero); `ui-kit/no-status-dot-outside-plans` keeps `StatusDot` to the legends and plans listed in the plugin (ADR 0149).

`moto-content-surface` and `moto-popover-surface` are unlayered CSS classes, so they beat every Tailwind utility on the same element for border color, background and shadow and ring; `ui-kit/no-dead-surface-utility` fails such utilities beside them (focus: `outline-2`, override: `!`). Never put `moto-content-surface` on an element that needs a tinted branch (`danger ? "border-moto-red/20 bg-moto-red/10" : …`) — put it in the neutral branch only. It also adds `backdrop-filter: blur(8px)`, which turns the element into the containing block for `position: fixed` descendants (kit `Modal` is portal-rendered and unaffected).

```bash
# Components imported from the design-system package — should be ZERO (tokens only)
rg -n "from ['\"]@moto-nrw/design-system['\"]" frontend/src -g '*.tsx' -g '*.ts'
```

## Mandatory visual check — do not skip

Code can be correct and the UI still feel wrong (spacing, density, decoration). For any change a school user would see, **run the app locally and look at the actual screen before calling the work done** — do not rely on the diff or on tests passing. Use the project's browser-verification setup (the `verify` / agent-browser flow against `{slug}.localhost:3000`), screenshot the changed screen, and compare it side by side with the nearest canonical screen above. If it doesn't sit naturally next to that screen, it isn't done.

## Design-review checklist

Apply before marking any UI work complete (and reviewers: before approving):

- [ ] Tenant page renders `TenantPage` as its root; no own `max-w`/`mx-auto`/root padding/`<h1>`/action row
- [ ] No mini-heading (kicker) above a page title; status line carries real figures, not an explanation
- [ ] Reuses kit components; any new component is justified in the PR description
- [ ] Colors come from `LOCATION_COLORS` / kit components — no generic Tailwind hues
- [ ] Radius, spacing, and density match the canonical screens (card = `SectionCard`)
- [ ] No marketing-style hero / decorative cards — it reads as an operational school tool
- [ ] Works on mobile
- [ ] Dense enough for daily operational use (not airy / landing-page)
- [ ] Visually verified: ran the app, screenshotted the changed screen, compared against a canonical screen
- [ ] **Missverständnis-Check** passed and recorded in the PR description: every visible block explains its purpose in one sentence, nothing read-only looks clickable, preconditions are stated in the product, no two labels share a word stem without a visible boundary, copy follows `moto-einfache-sprache` — full checklist in `.claude/rules/verstaendlichkeit.md`

## When to deviate

Only with explicit reviewer approval recorded in the PR description, citing this rule and the reason. "It was faster to inline it" is not a reason — add the missing piece to the kit instead. The whole frontend pays for every bespoke one-off.
