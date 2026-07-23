export const meta = {
  name: 'timetable-ui-fix',
  description: 'Apply the agreed UI-kit consistency fixes across the remaining timetable files (one agent per file)',
  phases: [{ title: 'Fix', detail: 'one agent edits one timetable file per the spec', model: 'sonnet' }],
}

const DIR = 'frontend/src/components/timetable'

const CONV = `
You are applying a UI-kit consistency refactor to ONE timetable file in the MOTO "Project Phoenix" Next.js app. Make ONLY the changes listed for your file. Do NOT change behavior, props, state, data flow, validation, or any unrelated styling. Match the surrounding code style and keep ALL German text (with umlauts ä/ö/ü/ß) intact. Do NOT edit any *.test.tsx file. Do NOT run any shell/pnpm/git commands. Read the file first; line numbers are hints — locate each target by its actual code.

SHARED KIT FACTS (already present in the codebase):
- Button — import { Button } from "~/components/ui/button". Variants: primary (solid gray-900, has a subtle shadow), secondary, outline, outline_danger, danger, success, ghost (flat, transparent, hover:bg-gray-100, NO shadow). Sizes: sm/base/lg/xl (page-level: rounded-lg px-5 py-3; solid variants carry shadow), compact (h-8 rounded-md px-2.5 text-xs gap-1.5, flat), icon (square h-8 w-8 rounded-md, flat).
  - Button defaults to type="submit" — ALWAYS pass type="button" when it is not submitting a form.
  - Put layout/positioning classes (w-full, flex-1, mt-*, order-*, col-span-*, justify-self-*) in className; do NOT re-add padding/radius/height/shadow the size already provides.
  - Icon-only action (chevron, kebab, close, single icon) → variant="ghost" size="icon" + an aria-label.
  - Plain text/link action with no border and no selected/toggle state → variant="ghost" size="compact".
  - Solid filled CTA → variant="primary"; size="compact" for dense chrome, size="sm" for a normal or empty-state page CTA.
  - Modal / slide-over FOOTER buttons and dense in-form actions → size="md" (rounded-lg px-4 py-2 text-sm, the ConfirmationModal footer height). NEVER leave footer Abbrechen/Speichern at size="sm" — that is the oversized page height.
- Checkbox — import { Checkbox } from "~/components/ui/checkbox". Replace any raw <input type="checkbox" .../> with <Checkbox .../> (brand-green #83CD2D checked state). Preserve checked/onChange/disabled and keep the surrounding <label>/layout; drop the ad-hoc h-4 w-4 / text-* / focus:ring-* classes the kit already provides (a layout-only className may stay).
  - DO NOT convert (leave as semantic <button>): toggle/segmented/chip buttons that carry a selected/active state; bordered "pill" triggers; full-row clickable list items; buttons inside a custom dropdown/popover menu; absolutely-positioned calendar-grid cells. (Documented kit gap — out of scope.)
- Native <select> — KEEP the <select>/<option> element and ALL option/value/onChange logic. Only swap its className to the kit-Input look via the shared moto-select utility (provides appearance-none + a consistent chevron). Preserve a leading w-full only if it was full-width.
  - Form select (full field): "moto-select block w-full rounded-lg border-0 bg-white py-3 pl-4 text-base text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400 disabled:bg-gray-50 disabled:text-gray-500"
  - Compact/inline filter select: "moto-select block w-full rounded-lg border-0 bg-white py-2 pl-3 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400 disabled:bg-gray-50 disabled:text-gray-500"
  - If a select already has a separate custom chevron icon element next to it, remove that now-redundant icon (moto-select draws one). If removing it is not trivially safe, instead keep that icon and DROP "moto-select" from the className (apply only the ring/radius/bg/text classes).
- Card / surface radius — a content card/panel = rounded-2xl with border border-gray-200 bg-white shadow-sm. If it uses the "moto-content-surface" utility, that utility ALREADY supplies border-color + bg-white + shadow-sm; in that case only ensure it also has the "border" (width) class and "rounded-2xl" — do NOT add border-gray-200/bg-white/shadow-sm (redundant). Compact content tiles/blocks may use rounded-xl. Badges/pills/chips/status dots = rounded-full. Floating dropdown/popover panels STAY rounded-xl (do NOT bump to rounded-2xl). Buttons/inputs keep rounded-md/rounded-lg.
- Modal inline error banner (a hand-rolled <div> rendering a string error message) → replace with <Alert type="error" message={theMessageExpression} /> from "~/components/ui/alert". Keep the same conditional that controls when it shows.
- Colors — brand red = #FF3130. Never use generic Tailwind brand hues for brand semantics. Keep existing correct brand hexes (#83CD2D green, #EAB308 amber).

After editing, return a precise summary: every change you made, anything you deliberately left alone (and why), and any uncertainty. Your entire output IS the structured object.
`

const FILES = [
  {
    path: `${DIR}/template-card.tsx`,
    label: 'template-card',
    instructions: `FIXES:
1. The series card root surface: ensure it is a canonical card — rounded-2xl + a visible border. If it already uses "moto-content-surface", just make sure it ALSO has "border" (width) and "rounded-2xl"; otherwise it must be "rounded-2xl border border-gray-200 bg-white shadow-sm". Keep any hover:shadow-md.
2. The small weekday indicator pills/dots: change bare "rounded" (or rounded-sm/rounded-md) to "rounded-full".
3. The primary "Termine erzeugen" CTA <button>: convert to <Button type="button" variant="primary" ...>. Choose size="compact" if the existing button is dense (h-8/text-xs) or size="sm" if it is a normal-height CTA. Preserve w-full / layout classes via className.
4. The "Bearbeiten" and "Archivieren" text actions (plain text buttons, no border, no toggle state): convert each to <Button type="button" variant="ghost" size="compact"> (keep their onClick + any color intent like a danger tint via className only if they currently have one; "Archivieren" if it is red should use variant="ghost" size="compact" with text color via className, or leave its color as-is).`,
  },
  {
    path: `${DIR}/template-list.tsx`,
    label: 'template-list',
    instructions: `FIXES:
1. The empty-state wrapper: change rounded-lg → rounded-2xl (keep border-dashed / bg-gray-50 if present).
2. The empty-state CTA <button> ("Serie anlegen" or similar): convert to <Button type="button" variant="primary" size="sm"> preserving mt-* / layout classes via className.`,
  },
  {
    path: `${DIR}/month-planner-grid.tsx`,
    label: 'month-planner-grid',
    instructions: `FIXES:
1. The activity instance chip: change rounded-md → rounded-full (it is a pill).
2. The inline "Spontan" badge: change bare "rounded" → rounded-full.
DO NOT touch the calendar day-cell <button> elements — leave them as semantic buttons.`,
  },
  {
    path: `${DIR}/year-planner-grid.tsx`,
    label: 'year-planner-grid',
    instructions: `FIXES:
1. The day-cell element: change bare "rounded" → "rounded-md" (control token).
DO NOT convert the month-header or day-cell <button> elements to Button — leave them semantic.`,
  },
  {
    path: `${DIR}/instance-block.tsx`,
    label: 'instance-block',
    instructions: `FIXES:
1. The event-block surface: change rounded-md → rounded-xl (a dense content block; rounded-2xl would be too heavy).
DO NOT convert the absolutely-positioned interactive block <button> — leave it semantic.`,
  },
  {
    path: `${DIR}/conflict-warnings-banner.tsx`,
    label: 'conflict-warnings-banner',
    instructions: `FIXES (minimal):
1. ONLY if the banner wrapper uses rounded-lg, change it to rounded-2xl and add shadow-sm if missing (surface consistency).
DO NOT replace this banner with the kit Alert component, and DO NOT change its brand-amber (#EAB308) colors — the amber is correct and the kit Alert would regress it to generic yellow. If the wrapper is already rounded-2xl, make NO changes and report that.`,
  },
  {
    path: `${DIR}/period-switcher-dropdown.tsx`,
    label: 'period-switcher-dropdown',
    instructions: `FIXES:
1. The empty-state CTA <button> ("Periode anlegen", solid gray-900, h-8): convert to <Button type="button" variant="primary" size="compact"> (keep title/aria + onClick).
2. The per-row "Bearbeiten" text button in the period list: convert to <Button type="button" variant="ghost" size="compact"> (keep aria-label + onClick).
DO NOT touch: the trigger pill button (bordered pill with the green status dot), the full-row period <button> select targets (multi-line list rows), the "Neue Periode anlegen" footer row, or the floating panel (it STAYS rounded-xl). Leave the Skeleton as-is.`,
  },
  {
    path: `${DIR}/calendar-period-modal.tsx`,
    label: 'calendar-period-modal',
    instructions: `FIXES:
1. INLINE ERROR BANNER: it already uses <Alert type="error" ... /> — leave it. (Only if you find a genuinely hand-rolled <div> string-error banner, convert it to <Alert type="error" message={expr} />.)
2. Any ad-hoc dark red hex (e.g. #991B1B) used for a brand-red purpose: change to #FF3130 (brand red). Verify contrast still reads.
3. The native <select> ("Typ"/period type): apply the FORM select className from the shared spec (keep the <select> and its options/onChange). It is currently rounded-lg — keep rounded-lg (the kit Input token).
4. CHECKBOX: replace the raw <input type="checkbox" .../> ("Periode ist aktiv", className "h-4 w-4") with the kit <Checkbox .../> (import { Checkbox } from "~/components/ui/checkbox"), preserving checked={form.isActive} and onChange. Keep the surrounding <label> row and its German texts.
5. FOOTER BUTTONS: in the FormModal footer, every action button currently using size="sm" (Abbrechen, Speichern, and any delete action) → change to size="md" (Flo's footer height). Keep variants and all other props.`,
  },
  {
    path: `${DIR}/plan-quality-panel.tsx`,
    label: 'plan-quality-panel',
    instructions: `FIXES:
1. The outer <section> card: change rounded-lg → rounded-2xl (it already has border border-gray-200 bg-white shadow-sm — keep those).
2. The metric sub-tile surface: change rounded-md → rounded-xl (it is a surface tile, not a control).
3. The two native <select> elements (absent staff, substitute staff): apply the COMPACT/inline select className from the shared spec (keep <select> + options + onChange).
4. If there is a hardcoded near-white hover hex such as #FFF7ED, change it to an amber tint "[#EAB308]/10" to align with the brand amber. Skip if not trivially safe.
DO NOT convert the full-row clickable <button> targets (the instance rows) — leave them semantic.`,
  },
  {
    path: `${DIR}/instance-detail-modal.tsx`,
    label: 'instance-detail-modal',
    instructions: `FIXES:
1. The "Spontan" badge and the activity-type badge: change bare "rounded" → "rounded-full".
2. FOOTER BUTTONS: in the Modal footer content, every action button currently using size="sm" (the outline/ghost cancel and the primary/lifecycle/danger actions) → change to size="md" (Flo's footer height). Keep each button's variant and all other props.
DO NOT touch the kit Modal's own close button (it lives in ui/modal.tsx) or the IconActionButton helper / lifecycle icon buttons — leave those as-is.`,
  },
  {
    path: `${DIR}/timetable-event-modal.tsx`,
    label: 'timetable-event-modal',
    model: 'opus',
    instructions: `This is the largest file. Apply these fixes carefully, one at a time, preserving ALL behavior. Button and Input are already imported here.
1. REPEAT-MODE SWITCHER ("Wiederholen": a hand-rolled segmented pill toggle of options like Nie / Jede Woche / Alle 2 Wochen, a div.inline-flex.bg-gray-100 with three raw <button>): convert to the kit Tabs. import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs". Render:
     <Tabs value={<currentModeState>} onValueChange={(v) => <existingSetter>(v as <ModeType>)}>
       <TabsList aria-label="Wiederholen">
         <TabsTrigger value="<exactValueA>" disabled={<keep any existing per-button disabled expr, else omit>}>Label A</TabsTrigger>
         ...one TabsTrigger per option, in the same order...
       </TabsList>
     </Tabs>
   CRITICAL: read the current buttons' onClick to find the EXACT state variable, setter, and EXACT string value each button sets; reuse them verbatim. One button's onClick ALSO seeds a default weekday when switching away from "none" — preserve that side-effect (run the same follow-up inside onValueChange after calling the setter). Each TabsTrigger value must equal what the old button set. Keep the "Wiederholen" field label above the control. This also removes the bare "rounded" (4px) on those buttons.
2. INLINE ERROR BANNERS: this file already uses <Alert type="error" ... /> for its error messages — leave those as-is. Only if you find a genuinely hand-rolled <div> error banner showing a string, convert it to <Alert type="error" message={expr} />.
3. MULTISELECT SEARCH INPUT (a raw <input type="search"> with an absolute Search icon overlay): KEEP the element and the icon; only align its className to the kit Input look — "block w-full rounded-lg border-0 bg-white py-3 text-base text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset placeholder:text-gray-400 focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400" plus the existing left padding for the icon (pl-9). Do NOT swap to the <Input> component (it would break the icon overlay).
4. NATIVE <select> elements (room, category, education group, period, primary staff = FORM selects; the "Alle Klassen"/"Alle Gruppen" filters = COMPACT/inline selects): apply the matching FORM or COMPACT select className from the shared spec. Keep every <select>, its <option>s, value and onChange exactly. Their rounded-lg radius is the kit Input token — keep rounded-lg.
5. CHECKBOX: in MultiSelectField, replace the raw <input type="checkbox" .../> (className "h-4 w-4 rounded border-gray-300 text-gray-900 focus:ring-gray-200") with the kit <Checkbox .../> (import { Checkbox } from "~/components/ui/checkbox"). Preserve checked={...} and onChange={...}; let Checkbox provide the sizing/color/focus.
6. FOOTER BUTTONS: in SlideOverFooter, the "Abbrechen" (variant="outline") and "Speichern" (variant="primary") buttons currently use size="sm" — change BOTH to size="md" (Flo's footer height). Keep every other prop (type, form, isLoading, loadingText, disabled, onClick) untouched.
7. READ-ONLY PERIOD BOX: the non-editable Planungsperiode display (the <div> showing "Gilt in ..." whose classes include "flex h-10 items-center rounded-2xl border border-gray-200 bg-gray-50 px-3") — change rounded-2xl → rounded-lg and replace "h-10" with "py-2.5" (drop the fixed height so padding defines it, matching the kit field rhythm). Keep "items-center", the text and the logic.
DO NOT convert: the type-selector cards ("Typ"), the weekday toggle chips ("Wochentage"), or the MultiSelect "Sichtbare auswählen/Auswahl leeren/Filter zurücksetzen" action buttons — the first two are toggle/chip controls with selected state (leave their structure + rounded-md). The three MultiSelect toolbar action buttons MAY be converted to <Button type="button" variant="ghost" size="compact"> (and the primary "Sichtbare auswählen" to variant="outline" size="compact") if clean; otherwise leave them.`,
  },
]

phase('Fix')

const EDIT_RESULT_SCHEMA = {
  type: 'object',
  additionalProperties: false,
  properties: {
    file: { type: 'string' },
    changesMade: {
      type: 'array',
      items: {
        type: 'object',
        additionalProperties: false,
        properties: {
          what: { type: 'string' },
          kitTarget: { type: 'string' },
        },
        required: ['what', 'kitTarget'],
      },
    },
    leftAlone: { type: 'array', items: { type: 'string' } },
    uncertainties: { type: 'array', items: { type: 'string' } },
    behaviorPreserved: { type: 'boolean' },
  },
  required: ['file', 'changesMade', 'leftAlone', 'uncertainties', 'behaviorPreserved'],
}

const results = await parallel(
  FILES.map((f) => () =>
    agent(`${CONV}\n\nYOUR FILE: ${f.path}\n\n${f.instructions}`, {
      label: `fix:${f.label}`,
      phase: 'Fix',
      schema: EDIT_RESULT_SCHEMA,
      ...(f.model ? { model: f.model } : { model: 'sonnet' }),
    }),
  ),
)

const done = results.filter(Boolean)
log(`Edited ${done.length}/${FILES.length} files`)
return { results: done }
