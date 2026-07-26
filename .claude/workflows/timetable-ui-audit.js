export const meta = {
  name: 'timetable-ui-audit',
  description: 'Audit every timetable UI file against the MOTO shared UI kit, synthesize a unified fix plan',
  phases: [
    { title: 'Audit', detail: 'one agent per timetable UI file → structured findings', model: 'sonnet' },
    { title: 'Synthesize', detail: 'dedup, find cross-cutting themes + kit gaps, build fix plan' },
  ],
}

// ---- The timetable UI surface (non-test, non-API-route) ----
const DIR = 'frontend/src/components/timetable'
const PAGE = 'frontend/src/app/[tenant]/(protected)/timetables/page.tsx'
const FILES = [
  { path: PAGE, role: 'Main planner page — header, view switching, surfaces, wiring of all sub-products' },
  { path: `${DIR}/timetable-toolbar.tsx`, role: 'Top toolbar (ALREADY partly refactored on this branch — verify it is now fully kit-correct)' },
  { path: `${DIR}/planning-menu.tsx`, role: 'Dropdown menu: "Angebote einplanen" (materialize / replan)' },
  { path: `${DIR}/period-switcher-dropdown.tsx`, role: 'Calendar-period switcher dropdown in the page header' },
  { path: `${DIR}/weekly-calendar-grid.tsx`, role: 'Week view calendar grid + empty state' },
  { path: `${DIR}/month-planner-grid.tsx`, role: 'Month view grid' },
  { path: `${DIR}/year-planner-grid.tsx`, role: 'Year view grid' },
  { path: `${DIR}/instance-block.tsx`, role: 'Single instance block rendered inside the week grid' },
  { path: `${DIR}/template-list.tsx`, role: 'Series (Serien) list view' },
  { path: `${DIR}/template-card.tsx`, role: 'Single series card' },
  { path: `${DIR}/plan-quality-panel.tsx`, role: 'Side card: plan-quality / staff gaps / substitution (contains native <select>)' },
  { path: `${DIR}/conflict-warnings-banner.tsx`, role: 'Inline warning banner for conflicts' },
  { path: `${DIR}/calendar-period-modal.tsx`, role: 'Modal: create/edit calendar period (contains native <select>)' },
  { path: `${DIR}/timetable-event-modal.tsx`, role: 'Large modal/slide-over: create/edit instance or series (many native <select>)' },
  { path: `${DIR}/instance-detail-modal.tsx`, role: 'Modal: instance detail, lifecycle actions, roster' },
]

// ---- Kit reference embedded so auditors share one rubric ----
const KIT_REFERENCE = `
SHARED UI KIT — THE SOURCE OF TRUTH (frontend/src/components/ui/ and ui/page-header/).
Rule file: .claude/rules/frontend-ui-kit.md (READ it). Brand colors: frontend/src/lib/location-helper.ts LOCATION_COLORS (READ it for exact hex).

Need X → use Y (do NOT hand-roll):
- Page/list header           → PageHeaderWithSearch / PageHeader   (~/components/ui/page-header). A raw <header>+<h1> page title is a VIOLATION.
- Header nav tabs            → NavigationTabs                       (~/components/ui/page-header/NavigationTabs)
- Button / CTA              → Button (variants primary|secondary|outline|outline_danger|danger|success; sizes sm|base|lg|xl)  (~/components/ui/button). A raw <button> used as a CTA/action is a VIOLATION (small icon-only chrome may be a documented kit gap).
- Tabs / segmented switcher  → Tabs/TabsList/TabsTrigger/TabsContent (variant default=pill | line=underline) (~/components/ui/tabs)
- Text input                → Input                                 (~/components/ui/input)
- Modal dialog              → Modal / ConfirmationModal             (~/components/ui/modal)
- Form-in-modal             → FormModal                             (~/components/ui/form-modal)
- Delete confirm            → ConfirmDeleteModal                    (~/components/ui/confirm-delete-modal)
- Slide-over / drawer       → slide-over / Drawer                   (~/components/ui/slide-over, ~/components/ui/drawer)
- Data / list table         → DataTable, DataTableStatusBadge       (~/components/ui/data-table)
- Info / stat card          → InfoCard, InfoItem                    (~/components/ui/info-card)
- Detail-panel fields       → DataField, InfoSection, DataGrid, InfoText (~/components/ui/detail-modal-components)
- Date / range picker       → DatePicker, date-range-picker         (~/components/ui/date-picker, date-range-picker)
- Loading / skeleton        → Loading, Skeleton                     (~/components/ui/loading, skeleton)
- Avatar                    → Avatar
- Location/presence badge    → LocationBadge, PresenceBadge, StudentPresenceBadge
- Kebab / overflow menu     → OverflowMenu                          (~/components/ui/page-header/OverflowMenu)
- API error message text     → getApiErrorMessage                   (~/lib/api-error-message)

CANONICAL VALUES:
- Card / panel surface = "rounded-2xl border border-gray-200 bg-white shadow-sm" (24px radius). Inconsistent card radii (rounded-lg/rounded-xl for a card surface), bare "rounded", ad-hoc "rounded-[Npx]", or a square border-b-only strip are VIOLATIONS.
- Radius tokens: button & input = rounded-md (8px); card = rounded-2xl (24px); badge/pill = rounded-full. bare "rounded" = 4px = off-scale.
- Padding: card p-6 (p-4 compact / p-10 large); button px-5 py-2 (md) / px-3 py-1 (sm); input px-4 py-3.
- Shadows: shadow-sm (cards/surfaces, default), shadow-md (hover), shadow-lg/xl (overlays/modals).

COLORS — route every brand-semantic color through LOCATION_COLORS hex (arbitrary value syntax bg-[#83CD2D]) or a kit component:
  green #83CD2D (primary, hover #74b827, active #669f21) | blue #5080D8 | red #FF3130 | orange #F78C10 | magenta #D946EF | amber #EAB308 | purple #7C3AED | gray #6B7280.
  VIOLATION: generic Tailwind brand hues (text-green-500, bg-blue-500, border-red-400 ...). Neutral gray-* is fine.
  VIOLATION: importing COMPONENTS from @moto-nrw/design-system (tokens/CSS only), or using sage/steel/--color-brand-primary palette (wrong green).

KNOWN KIT GAPS (per the rule — flag, do not silently inline a one-off):
- NO generic Select / Dropdown component exists. Native <select>/<option> is a VIOLATION but its fix is BLOCKED on a kit gap → set blockedByKitGap=true, kitTarget="KIT GAP: needs ui/select".
- NO compact / ghost / icon-only Button variant exists. Dense toolbar chrome that needs one → blockedByKitGap=true where relevant.
`

phase('Audit')

const FINDING_SCHEMA = {
  type: 'object',
  additionalProperties: false,
  properties: {
    file: { type: 'string' },
    overallAssessment: { type: 'string', description: 'One-paragraph verdict: how kit-aligned is this file overall?' },
    alreadyCorrect: { type: 'array', items: { type: 'string' }, description: 'Kit components/patterns this file already uses correctly' },
    findings: {
      type: 'array',
      items: {
        type: 'object',
        additionalProperties: false,
        properties: {
          category: { type: 'string', enum: ['page-header', 'border-radius', 'button', 'dropdown-select', 'color', 'card-surface', 'modal', 'slide-over', 'spacing', 'tabs', 'badge', 'table', 'input', 'icon', 'other'] },
          severity: { type: 'string', enum: ['high', 'medium', 'low'] },
          lines: { type: 'string', description: 'e.g. "1108-1109"' },
          problem: { type: 'string' },
          currentSnippet: { type: 'string', description: 'short verbatim snippet of the offending code' },
          recommendedFix: { type: 'string', description: 'concrete change, naming the exact kit component/class' },
          kitTarget: { type: 'string', description: 'the kit component to use, or "KIT GAP: needs ui/<x>"' },
          effort: { type: 'string', enum: ['trivial', 'moderate', 'significant'] },
          blockedByKitGap: { type: 'boolean' },
        },
        required: ['category', 'severity', 'lines', 'problem', 'recommendedFix', 'kitTarget', 'effort', 'blockedByKitGap'],
      },
    },
  },
  required: ['file', 'overallAssessment', 'alreadyCorrect', 'findings'],
}

const audits = await parallel(
  FILES.map((f) => () =>
    agent(
      `You are a frontend design-consistency auditor for the MOTO "Project Phoenix" Next.js app.

TASK: Audit ONE file against the shared UI kit and report every deviation. Be precise and conservative — only report REAL violations of the kit rules below, with exact line numbers. Do NOT report subjective preferences that the kit does not mandate. Do NOT propose behavior changes — this is a visual/structural consistency audit only.

FILE TO AUDIT: ${f.path}
FILE ROLE: ${f.role}

STEPS:
1. Read the file in full.
2. Read .claude/rules/frontend-ui-kit.md and frontend/src/lib/location-helper.ts (for exact LOCATION_COLORS hex). Read any specific kit component file you need to confirm its real props/API before recommending it (e.g. ~/components/ui/button.tsx, tabs.tsx, modal.tsx, info-card.tsx, page-header/PageHeader.tsx).
3. For every deviation, produce a finding. Look specifically for: raw <header>/<h1> page titles instead of PageHeader; raw <button> used as buttons/CTAs instead of Button; native <select>/<option> instead of a kit dropdown (KIT GAP); ad-hoc/inconsistent border radii on card surfaces (rounded-lg / rounded-xl / bare rounded / rounded-[Npx] where the canonical card is rounded-2xl border border-gray-200 bg-white shadow-sm); generic Tailwind brand colors (text-green-500 etc.) instead of LOCATION_COLORS hex; hand-rolled cards/panels/modals/tabs/badges/tables that a kit component already covers; @moto-nrw/design-system component imports or sage/steel palette; inconsistent padding/shadow vs the canonical values.
4. If the file already uses kit components correctly, list those in alreadyCorrect — credit is informative.

${KIT_REFERENCE}

Return the structured audit. Your entire output IS the data object (no prose outside it).`,
      { label: `audit:${f.path.split('/').pop()}`, phase: 'Audit', schema: FINDING_SCHEMA, model: 'sonnet' },
    ),
  ),
)

const valid = audits.filter(Boolean)
log(`Audited ${valid.length}/${FILES.length} files; ${valid.reduce((n, a) => n + (a.findings?.length ?? 0), 0)} raw findings`)

phase('Synthesize')

const SYNTH_SCHEMA = {
  type: 'object',
  additionalProperties: false,
  properties: {
    executiveSummary: { type: 'string', description: '3-6 sentences: state of timetable UI consistency and the biggest themes' },
    crossCuttingThemes: {
      type: 'array',
      items: {
        type: 'object',
        additionalProperties: false,
        properties: {
          theme: { type: 'string' },
          description: { type: 'string' },
          affectedFiles: { type: 'array', items: { type: 'string' } },
          severity: { type: 'string', enum: ['high', 'medium', 'low'] },
        },
        required: ['theme', 'description', 'affectedFiles', 'severity'],
      },
    },
    kitGaps: {
      type: 'array',
      items: {
        type: 'object',
        additionalProperties: false,
        properties: {
          component: { type: 'string', description: 'e.g. "ui/select"' },
          why: { type: 'string' },
          affectedFiles: { type: 'array', items: { type: 'string' } },
          recommendation: { type: 'string', description: 'build it / reuse existing pattern X / defer' },
        },
        required: ['component', 'why', 'affectedFiles', 'recommendation'],
      },
    },
    fixPlan: {
      type: 'array',
      description: 'One entry per file, ordered by priority',
      items: {
        type: 'object',
        additionalProperties: false,
        properties: {
          file: { type: 'string' },
          priority: { type: 'string', enum: ['P0', 'P1', 'P2'] },
          summary: { type: 'string' },
          fixes: {
            type: 'array',
            items: {
              type: 'object',
              additionalProperties: false,
              properties: {
                lines: { type: 'string' },
                change: { type: 'string' },
                kitTarget: { type: 'string' },
                effort: { type: 'string', enum: ['trivial', 'moderate', 'significant'] },
                blockedByKitGap: { type: 'boolean' },
              },
              required: ['lines', 'change', 'kitTarget', 'effort', 'blockedByKitGap'],
            },
          },
        },
        required: ['file', 'priority', 'summary', 'fixes'],
      },
    },
    decisionsNeeded: {
      type: 'array',
      description: 'Genuine choices the human must make before/while fixing',
      items: {
        type: 'object',
        additionalProperties: false,
        properties: {
          question: { type: 'string' },
          options: { type: 'array', items: { type: 'string' } },
          recommendation: { type: 'string' },
        },
        required: ['question', 'options', 'recommendation'],
      },
    },
    quickWins: { type: 'array', items: { type: 'string' }, description: 'Trivial mechanical fixes safe to batch immediately' },
    statistics: {
      type: 'object',
      additionalProperties: true,
      properties: {
        totalFindings: { type: 'number' },
        bySeverity: { type: 'object', additionalProperties: true },
        byCategory: { type: 'object', additionalProperties: true },
      },
      required: ['totalFindings'],
    },
  },
  required: ['executiveSummary', 'crossCuttingThemes', 'kitGaps', 'fixPlan', 'decisionsNeeded', 'quickWins', 'statistics'],
}

const synthesis = await agent(
  `You are the lead frontend reviewer synthesizing a UI-consistency audit of the MOTO timetable (Stundenplan) feature into ONE actionable plan.

Below are per-file audit results (JSON). Your job:
1. Dedupe and consolidate — the same theme appears across many files (e.g. inconsistent card radii, hand-rolled buttons, native selects). Group them into crossCuttingThemes.
2. Identify KIT GAPS that block fixes (notably: no generic Select/Dropdown component; no compact/ghost/icon-only Button variant). For each, recommend whether to build the missing kit component (and where), reuse an existing pattern, or defer. Be concrete.
3. Build a prioritized fixPlan, one entry per file. P0 = high-visibility unification (page header, toolbar, card radii, obvious hand-rolled buttons). P1 = native selects / dropdowns and modal/slide-over chrome. P2 = polish (spacing, shadows, minor color). Each fix names the exact kit target and effort, and flags blockedByKitGap.
4. List decisionsNeeded — only GENUINE choices a human must make (e.g. "Build a ui/Select component now, or keep native selects styled to match?"). Give a recommendation for each.
5. List quickWins — trivial mechanical changes safe to apply in a batch immediately (e.g. fix a single card's radius to rounded-2xl).
6. Be skeptical: if a per-file finding looks like a false positive (kit does not actually mandate it, or the recommended kit component does not fit the use case), drop it or downgrade it and say so in the relevant theme/summary. Do not pad the plan.

Compute statistics (totalFindings, counts by severity and category) from the consolidated, deduped set.

PER-FILE AUDITS:
${JSON.stringify(valid)}

${KIT_REFERENCE}

Return the structured synthesis. Your entire output IS the data object.`,
  { label: 'synthesize-plan', phase: 'Synthesize', schema: SYNTH_SCHEMA },
)

return { fileCount: valid.length, audits: valid, synthesis }
