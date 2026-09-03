// UI-kit drift ratchet (issue #1629).
//
// Five hard-zero rules that stop drift away from the shared UI kit. The
// hand-rolled surface rule started with a shrink-only baseline (#2934) that
// was brought to zero and deleted in #2933; the first production match of
// any rule now fails `pnpm run check`:
//
//   ui-kit/no-generic-brand-colors  — generic Tailwind hues (bg-green-500 …)
//                                     used for brand semantics; use the brand
//                                     hex from LOCATION_COLORS or a kit
//                                     component (.claude/rules/frontend-ui-kit.md)
//   ui-kit/no-hand-rolled-overlay   — `fixed inset-0` overlays outside
//                                     src/components/ui/; use Modal / Drawer /
//                                     OverflowMenu from the kit
//   ui-kit/no-hand-rolled-surface   — hand-built card surfaces
//                                     (`rounded-xl/2xl` + `border` + `bg-white`)
//                                     outside src/components/ui/; use
//                                     moto-content-surface (cards),
//                                     moto-popover-surface (floating menus),
//                                     ChoiceTile (selectable rows) or a kit
//                                     surface component (issue #2933)
//   ui-kit/no-rounded-3xl           — off-scale surface radius; cards are
//                                     rounded-2xl (moto-content-surface)
//   ui-kit/require-checkbox-label   — every shared Checkbox is wrapped by a
//                                     label so its visible box is clickable
//
// Four more rules belong to the tenant page scaffold (#2619):
//
//   ui-kit/no-tiny-text             — sub-12px text utilities; shrink-only
//                                     baseline
//   ui-kit/no-tenant-kicker         — no blue mini-heading in the tenant portal
//   ui-kit/no-handrolled-surface    — no `moto-content-surface … rounded-2xl`
//                                     spelled out in a tenant page/loading
//                                     file; use the kit cards
//   ui-kit/no-tabs-as-value-switcher — ui/Tabs is for content panels; values
//                                     go on SegmentedControl
//
// The tiny-text baseline below is SHRINK-ONLY: matches may be removed when
// a file is migrated, but never added.

// Test/stories files are exempt from the class-string rules. A deliberate
// non-card match stays behind `oxlint-disable-next-line` with a reason; there
// is no baseline to grow.

const BRAND_COLOR_RE =
  /\b(?:text|bg|border(?:-[trblxyse])?|ring|outline|fill|stroke|from|via|to|divide|accent|caret|decoration|shadow)-(?:red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-\d+(?:\/(?:\d+|\[[^\]\s]+\]))?(?![\w-])/g;
const FIXED_RE = /\bfixed\b/;
const INSET_0_RE = /\binset-0\b/;
// A surface is three UNPREFIXED tokens: `print:border`, `sm:rounded-2xl` and
// `focus:bg-white` belong to another state, `border-0` / `border-none` /
// `border-b` draw no frame, and `bg-white/80` is not the solid card fill.
// `\b` accepted all of those because `-` and `:` are word boundaries.
const SURFACE_ROUNDED_RE = /(?:^|\s)rounded-(?:xl|2xl)(?=\s|$)/;
const SURFACE_BORDER_RE = /(?:^|\s)border(?:-[1-9]\d*)?(?=\s|$)/;
const SURFACE_BG_RE = /(?:^|\s)bg-white(?=\s|$)/;
const ROUNDED_3XL_RE = /\brounded-3xl\b/g;

const EXEMPT_FILE_RE = /(?:\.(?:test|stories)\.)|(?:\.d\.ts$)/;
const UI_KIT_DIR = "src/components/ui/";

/** Repo-relative posix path ("src/…"), or "" when unavailable. */
function fileKey(context) {
  const raw = String(context.physicalFilename ?? context.filename ?? "");
  const posix = raw.replaceAll("\\", "/");
  const idx = posix.lastIndexOf("/src/");
  return idx === -1 ? posix : posix.slice(idx + 1);
}

/** Parse `token@line` rows into immutable per-file occurrence snapshots. */
function parseLocationBaseline(source) {
  return new Map(
    source
      .trim()
      .split("\n")
      .map((row) => {
        const [file, encoded] = row.split("|");
        const occurrences = new Map();
        for (const location of encoded.split(" ")) {
          occurrences.set(location, (occurrences.get(location) ?? 0) + 1);
        }
        return [file, occurrences];
      }),
  );
}

function isBaselineMatch(baseline, key, match, line, seenOccurrences) {
  const location = `${match}@${line}`;
  const seen = seenOccurrences.get(location) ?? 0;
  seenOccurrences.set(location, seen + 1);
  return seen < (baseline.get(key)?.get(location) ?? 0);
}

function restrictMatchBaseline(baselineFiles, baseline) {
  return new Map([...baseline].filter(([file]) => baselineFiles.has(file)));
}

function collectStaticStrings(node, chunks, seen = new WeakSet()) {
  if (!node || typeof node !== "object" || seen.has(node)) return;
  seen.add(node);

  if (node.type === "Literal" && typeof node.value === "string") {
    chunks.push(node.value);
    return;
  }
  if (node.type === "TemplateElement") {
    chunks.push(node.value.raw);
    return;
  }

  for (const [key, value] of Object.entries(node)) {
    if (key === "parent") continue;
    if (Array.isArray(value)) {
      for (const child of value) collectStaticStrings(child, chunks, seen);
    } else {
      collectStaticStrings(value, chunks, seen);
    }
  }
}

const MAX_CLASS_COMBINATIONS = 64;

function crossJoin(left, right) {
  const out = [];
  for (const a of left) {
    for (const b of right) {
      out.push(`${a} ${b}`);
      if (out.length > MAX_CLASS_COMBINATIONS) return null;
    }
  }
  return out;
}

/**
 * Possible rendered class strings of a className expression. Conditional and
 * logical branches contribute ALTERNATIVES instead of being joined, so
 * mutually exclusive literals (`cond ? "fixed …" : "… inset-0"`) cannot
 * combine into a phantom overlay, and strings inside a branch test
 * (`pos === "fixed" ? … : …`) never count as rendered classes. Returns null
 * when the combination count overflows; callers fall back to the joined
 * over-approximation.
 */
function classCombinations(node, seen = new WeakSet()) {
  if (!node || typeof node !== "object" || seen.has(node)) return [""];
  seen.add(node);

  if (node.type === "Literal") {
    return [typeof node.value === "string" ? node.value : ""];
  }
  if (node.type === "TemplateElement") return [node.value.raw];
  if (node.type === "ConditionalExpression") {
    const consequent = classCombinations(node.consequent, seen);
    const alternate = classCombinations(node.alternate, seen);
    if (!consequent || !alternate) return null;
    const merged = [...consequent, ...alternate];
    return merged.length > MAX_CLASS_COMBINATIONS ? null : merged;
  }
  if (node.type === "LogicalExpression") {
    const right = classCombinations(node.right, seen);
    if (!right) return null;
    if (node.operator === "&&") return ["", ...right];
    const left = classCombinations(node.left, seen);
    if (!left) return null;
    const merged = [...left, ...right];
    return merged.length > MAX_CLASS_COMBINATIONS ? null : merged;
  }

  let combos = [""];
  for (const [key, value] of Object.entries(node)) {
    if (key === "parent") continue;
    const children = Array.isArray(value) ? value : [value];
    for (const child of children) {
      if (!child || typeof child !== "object") continue;
      const childCombos = classCombinations(child, seen);
      if (!childCombos) return null;
      combos = crossJoin(combos, childCombos);
      if (!combos) return null;
    }
  }
  return combos;
}

/**
 * Builds a rule that reports string/template-literal chunks matching `regex`.
 * Hard-zero unless a shrink-only per-file `baseline` is given; then the
 * individual legacy matches recorded there are tolerated.
 */
function makeClassStringRule({
  regex,
  baseline = new Map(),
  skipUiKit,
  docs,
  messageId,
  message,
}) {
  return {
    meta: {
      type: "problem",
      docs: { description: docs },
      messages: { [messageId]: message },
      schema: [],
    },
    create(context) {
      const key = fileKey(context);
      if (!key.startsWith("src/")) return {};
      if (EXEMPT_FILE_RE.test(key)) return {};
      if (skipUiKit && key.startsWith(UI_KIT_DIR)) return {};
      const seenBaselineOccurrences = new Map();
      const check = (node, text) => {
        regex.lastIndex = 0;
        for (const match of text.matchAll(regex)) {
          // Nur die Tiny-Text-Regel hat eine Standort-Baseline. Die übrigen
          // Hard-zero-Regeln brauchen keine Quellposition und bleiben damit
          // auch mit den absichtlich schlanken AST-Knoten ihrer Unit-Tests
          // prüfbar.
          const baselineMatches = baseline.get(key);
          const line =
            baselineMatches && node.loc
              ? node.loc.start.line +
                (text.slice(0, match.index).match(/\n/g)?.length ?? 0)
              : undefined;
          if (
            !isBaselineMatch(
              baseline,
              key,
              match[0],
              line,
              seenBaselineOccurrences,
            )
          ) {
            context.report({ node, messageId, data: { match: match[0] } });
          }
        }
      };

      return {
        Literal(node) {
          if (typeof node.value === "string") check(node, node.value);
        },
        TemplateLiteral(node) {
          for (const quasi of node.quasis) {
            check(quasi, quasi.value.raw);
          }
        },
      };
    },
  };
}

const noGenericBrandColors = makeClassStringRule({
  regex: BRAND_COLOR_RE,
  skipUiKit: false,
  docs: "Disallow generic Tailwind brand-color utilities; brand semantics use the LOCATION_COLORS hexes or a kit component.",
  messageId: "genericBrandColor",
  message:
    "Generic Tailwind hue '{{match}}…' is not a brand color. Use a moto-* semantic token, LOCATION_COLORS (~/lib/location-helper), or a kit component (see .claude/rules/frontend-ui-kit.md).",
});

const noHandRolledOverlay = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Disallow hand-rolled full-screen overlays (`fixed inset-0`) outside the UI kit; use Modal, Drawer, ConfirmationModal, or OverflowMenu.",
    },
    messages: {
      handRolledOverlay:
        "Hand-rolled full-screen overlay ('{{match}}'). Use the kit Modal / Drawer / OverflowMenu instead of a bespoke fixed inset-0 layer.",
    },
    schema: [],
  },
  create(context) {
    const key = fileKey(context);
    if (!key.startsWith("src/")) return {};
    if (EXEMPT_FILE_RE.test(key)) return {};
    if (key.startsWith(UI_KIT_DIR)) return {};

    const check = (node, text) => {
      if (!FIXED_RE.test(text) || !INSET_0_RE.test(text)) return;
      context.report({
        node,
        messageId: "handRolledOverlay",
        data: { match: "fixed … inset-0" },
      });
    };

    return {
      Literal(node) {
        if (typeof node.value === "string") check(node, node.value);
      },
      TemplateLiteral(node) {
        for (const quasi of node.quasis) {
          check(node, quasi.value.raw);
        }
      },
      JSXAttribute(node) {
        if (
          node.name.type !== "JSXIdentifier" ||
          node.name.name !== "className"
        )
          return;

        const chunks = [];
        collectStaticStrings(node.value, chunks);
        if (
          chunks.some((chunk) => FIXED_RE.test(chunk) && INSET_0_RE.test(chunk))
        ) {
          return; // already reported by the Literal/TemplateLiteral visitors
        }
        const combos = classCombinations(node.value) ?? [chunks.join(" ")];
        const rendered = combos.find(
          (combo) => FIXED_RE.test(combo) && INSET_0_RE.test(combo),
        );
        if (rendered !== undefined) check(node, rendered);
      },
    };
  },
};

const isSurfaceCombo = (text) =>
  SURFACE_ROUNDED_RE.test(text) &&
  SURFACE_BORDER_RE.test(text) &&
  SURFACE_BG_RE.test(text);

const noHandRolledSurface = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Disallow hand-built card surfaces (`rounded-xl/2xl` + `border` + `bg-white`) outside the UI kit; use moto-content-surface, moto-popover-surface, ChoiceTile or a kit surface component (InfoCard, SectionCard).",
    },
    messages: {
      handRolledSurface:
        "Hand-rolled card surface ('{{match}}'). Use moto-content-surface (cards), moto-popover-surface (floating menus), ChoiceTile (selectable rows and tiles) or a kit surface component (InfoCard, SectionCard) instead. A deliberate non-card match may stay behind `// oxlint-disable-next-line ui-kit/no-hand-rolled-surface` with the reason recorded in the PR description (issue #2933).",
    },
    schema: [],
  },
  create(context) {
    const key = fileKey(context);
    if (!key.startsWith("src/")) return {};
    if (EXEMPT_FILE_RE.test(key)) return {};
    if (key.startsWith(UI_KIT_DIR)) return {};

    const check = (node, text) => {
      if (!isSurfaceCombo(text)) return;
      context.report({
        node,
        messageId: "handRolledSurface",
        data: { match: "rounded-xl/2xl … border … bg-white" },
      });
    };

    return {
      Literal(node) {
        if (typeof node.value === "string") check(node, node.value);
      },
      TemplateLiteral(node) {
        for (const quasi of node.quasis) {
          check(node, quasi.value.raw);
        }
      },
      JSXAttribute(node) {
        if (
          node.name.type !== "JSXIdentifier" ||
          node.name.name !== "className"
        )
          return;

        const chunks = [];
        collectStaticStrings(node.value, chunks);
        if (chunks.some((chunk) => isSurfaceCombo(chunk))) {
          return; // already reported by the Literal/TemplateLiteral visitors
        }
        const combos = classCombinations(node.value) ?? [chunks.join(" ")];
        const rendered = combos.find((combo) => isSurfaceCombo(combo));
        if (rendered !== undefined) check(node, rendered);
      },
    };
  },
};

const noRounded3xl = makeClassStringRule({
  regex: ROUNDED_3XL_RE,
  skipUiKit: false,
  docs: "Disallow rounded-3xl surfaces; the canonical card radius is rounded-2xl (moto-content-surface).",
  messageId: "rounded3xl",
  message:
    "rounded-3xl is off the brand radius scale. Cards/panels use rounded-2xl via moto-content-surface (see .claude/rules/frontend-ui-kit.md).",
});

// ui-kit/no-tiny-text — Schrift unter 12 px (text-[9px], text-[10px],
// text-[11px]) ist unter dem Typo-Boden des Portals: text-xs (12 px) ist die
// Untergrenze und selbst die ist Versalien-Labels vorbehalten, nie Werten,
// Saetzen oder Bedienelementen (TENANT-PAGE-SPEC, "Typo-Boden"). Der Bestand
// ist als Shrink-only-Baseline eingefroren; neue Winzschrift kommt nicht dazu.
const TINY_TEXT_RE = /\btext-\[(?:9|10|11)px\]/g;

const TINY_TEXT_BASELINE_FILES = new Set([
  "src/app/[tenant]/(protected)/calendar/page.tsx",
  "src/app/[tenant]/(protected)/database/personal/opening-balances/page.tsx",
  "src/app/[tenant]/(protected)/day-log/page.tsx",
  "src/app/[tenant]/(protected)/meal-plan/page.tsx",
  "src/app/[tenant]/(protected)/time-tracking/page.tsx",
  "src/app/help/nfc/erste-schritte/page.tsx",
  "src/app/operator/provisioning/soft-delete-shared.tsx",
  "src/components/active-supervisions/planned-now-section.tsx",
  "src/components/active-supervisions/timetable-roster.tsx",
  "src/components/activities/activity-management-modal.tsx",
  "src/components/auth/role-permission-management-modal.tsx",
  "src/components/calendar/personal-calendar.tsx",
  "src/components/dashboard/header/reminders-bell.tsx",
  "src/components/dashboard/sidebar.tsx",
  "src/components/enrollment/admin-enrollments-list.tsx",
  "src/components/enrollment/enrollment-form-editor.tsx",
  "src/components/enrollment/enrollment-form.tsx",
  "src/components/enrollment/phases-editor.tsx",
  "src/components/files/files-page.tsx",
  "src/components/guardians/guardian-contact-actions.tsx",
  "src/components/guardians/guardian-delete-modal.tsx",
  "src/components/guardians/guardian-list.tsx",
  "src/components/help/help-search.tsx",
  "src/components/parent/calendar/parent-calendar-page.tsx",
  "src/components/parent/parent-enroll-picker.tsx",
  "src/components/parent/parent-meal-plan-page.tsx",
  "src/components/planning/closing-day-marker.tsx",
  "src/components/staff/absence-request-row.tsx",
  "src/components/staff/arbeitszeitmodell-tab.tsx",
  "src/components/staff/dienstplan-resource-grid.tsx",
  "src/components/staff/staff-session-table.tsx",
  "src/components/students/care-schedule-manager.tsx",
  "src/components/students/planned-status-days-modal.tsx",
  "src/components/students/school-checkin-fab.tsx",
  "src/components/time-tracking/edit-history-accordion.tsx",
  "src/components/time-tracking/leave-requests-card.tsx",
  "src/components/time-tracking/vacation-request-modal.tsx",
  "src/components/timetable/bulk-substitution-modal.tsx",
  "src/components/timetable/event-form/multi-select-field.tsx",
  "src/components/timetable/event-form/step-termin.tsx",
  "src/components/timetable/gap-jump-list.tsx",
  "src/components/timetable/instance-block.tsx",
  "src/components/timetable/instance-detail-modal.tsx",
  "src/components/timetable/month-planner-grid.tsx",
  "src/components/timetable/period-switcher-dropdown.tsx",
  "src/components/timetable/staff-pool-slide-over.tsx",
  "src/components/timetable/substitution-person-card.tsx",
  "src/components/timetable/substitution-slide-over.tsx",
  "src/components/timetable/template-card.tsx",
  "src/components/timetable/timetable-add-menu.tsx",
  "src/components/timetable/vertretung-day-list.tsx",
  "src/components/timetable/vertretung-week-list.tsx",
  "src/components/timetable/weekly-calendar-grid.tsx",
  "src/components/ui/avatar.tsx",
  "src/components/ui/coverage-indicator.tsx",
  "src/components/ui/location-badge.tsx",
  "src/components/ui/multi-checkbox-select.tsx",
  "src/components/ui/notification-badge.tsx",
  "src/components/ui/origin-chip.tsx",
  "src/components/ui/page-header/FilterButton.tsx",
  "src/components/ui/page-header/OverflowMenu.tsx",
  "src/components/ui/parent-visible-badge.tsx",
  "src/components/ui/plan-block.tsx",
  "src/components/ui/plan-legend.tsx",
  "src/components/ui/presence-badge.tsx",
]);

const TINY_TEXT_BASELINE = parseLocationBaseline(`
src/app/[tenant]/(protected)/database/personal/opening-balances/page.tsx|text-[11px]@188 text-[11px]@225
src/app/[tenant]/(protected)/day-log/page.tsx|text-[11px]@164
src/app/[tenant]/(protected)/time-tracking/page.tsx|text-[10px]@1343 text-[11px]@1343 text-[10px]@1347 text-[10px]@1356 text-[11px]@1356 text-[10px]@2020
src/app/help/nfc/erste-schritte/page.tsx|text-[11px]@235 text-[11px]@276
src/app/operator/provisioning/soft-delete-shared.tsx|text-[11px]@218
src/components/active-supervisions/planned-now-section.tsx|text-[11px]@425
src/components/active-supervisions/timetable-roster.tsx|text-[11px]@92
src/components/activities/activity-management-modal.tsx|text-[10px]@330 text-[10px]@359 text-[10px]@396
src/components/auth/role-permission-management-modal.tsx|text-[10px]@330 text-[11px]@375
src/components/calendar/personal-calendar.tsx|text-[11px]@786 text-[11px]@814 text-[11px]@901 text-[11px]@962 text-[11px]@966 text-[11px]@972 text-[11px]@1018 text-[11px]@1024 text-[10px]@1044 text-[11px]@1097 text-[11px]@1177 text-[11px]@1183 text-[11px]@1188
src/components/dashboard/header/reminders-bell.tsx|text-[11px]@40 text-[10px]@94
src/components/dashboard/sidebar.tsx|text-[10px]@1320
src/components/enrollment/admin-enrollments-list.tsx|text-[11px]@915 text-[11px]@934
src/components/enrollment/enrollment-form-editor.tsx|text-[11px]@1296 text-[11px]@2973 text-[11px]@3165 text-[11px]@3170 text-[11px]@3175 text-[11px]@3329 text-[11px]@3921 text-[10px]@4138 text-[11px]@4150 text-[11px]@4185 text-[10px]@4199 text-[10px]@4243
src/components/enrollment/enrollment-form.tsx|text-[11px]@2090
src/components/enrollment/phases-editor.tsx|text-[11px]@663 text-[11px]@668
src/components/files/files-page.tsx|text-[11px]@158 text-[11px]@275
src/components/guardians/guardian-contact-actions.tsx|text-[10px]@141
src/components/guardians/guardian-delete-modal.tsx|text-[10px]@218
src/components/guardians/guardian-list.tsx|text-[10px]@334
src/components/help/help-search.tsx|text-[11px]@362
src/components/parent/calendar/parent-calendar-page.tsx|text-[11px]@676 text-[11px]@741
src/components/parent/parent-enroll-picker.tsx|text-[11px]@205 text-[11px]@209
src/components/parent/parent-meal-plan-page.tsx|text-[11px]@461 text-[11px]@546
src/components/planning/closing-day-marker.tsx|text-[10px]@53
src/components/staff/absence-request-row.tsx|text-[11px]@107
src/components/staff/arbeitszeitmodell-tab.tsx|text-[10px]@227 text-[10px]@240 text-[10px]@322 text-[11px]@1011
src/components/staff/dienstplan-resource-grid.tsx|text-[11px]@469 text-[11px]@565 text-[11px]@574 text-[11px]@581 text-[10px]@715
src/components/staff/staff-session-table.tsx|text-[11px]@1178
src/components/students/care-schedule-manager.tsx|text-[10px]@1121 text-[11px]@1276 text-[11px]@1365 text-[11px]@1375 text-[11px]@1405
src/components/students/planned-status-days-modal.tsx|text-[10px]@899 text-[11px]@899
src/components/students/school-checkin-fab.tsx|text-[10px]@198
src/components/time-tracking/edit-history-accordion.tsx|text-[10px]@118 text-[10px]@139 text-[10px]@150
src/components/time-tracking/leave-requests-card.tsx|text-[10px]@475
src/components/time-tracking/vacation-request-modal.tsx|text-[11px]@375
src/components/timetable/bulk-substitution-modal.tsx|text-[11px]@429 text-[11px]@471
src/components/timetable/event-form/multi-select-field.tsx|text-[11px]@147 text-[10px]@295 text-[11px]@301
src/components/timetable/event-form/step-termin.tsx|text-[10px]@167 text-[11px]@247 text-[11px]@280 text-[11px]@341 text-[11px]@347 text-[11px]@395
src/components/timetable/gap-jump-list.tsx|text-[10px]@109 text-[11px]@132 text-[11px]@147
src/components/timetable/instance-block.tsx|text-[10px]@198 text-[10px]@204 text-[10px]@211 text-[10px]@219 text-[10px]@240
src/components/timetable/instance-detail-modal.tsx|text-[10px]@251 text-[9px]@367 text-[9px]@946 text-[11px]@1211 text-[11px]@1237 text-[11px]@1413 text-[10px]@1483 text-[11px]@1506
src/components/timetable/month-planner-grid.tsx|text-[11px]@65 text-[11px]@141 text-[11px]@150 text-[11px]@171 text-[9px]@203 text-[10px]@237
src/components/timetable/period-switcher-dropdown.tsx|text-[11px]@203 text-[10px]@219 text-[11px]@228 text-[10px]@270 text-[10px]@294 text-[10px]@376 text-[10px]@390 text-[10px]@402 text-[11px]@410 text-[11px]@417
src/components/timetable/staff-pool-slide-over.tsx|text-[10px]@379 text-[11px]@444
src/components/timetable/substitution-person-card.tsx|text-[11px]@53 text-[11px]@331 text-[11px]@338 text-[11px]@345 text-[11px]@353 text-[11px]@365
src/components/timetable/substitution-slide-over.tsx|text-[10px]@546 text-[9px]@551 text-[11px]@704 text-[11px]@708 text-[10px]@748 text-[10px]@752 text-[10px]@756 text-[10px]@760 text-[11px]@1196
src/components/timetable/template-card.tsx|text-[11px]@113 text-[11px]@119 text-[10px]@162
src/components/timetable/timetable-add-menu.tsx|text-[10px]@61 text-[11px]@81 text-[11px]@103
src/components/timetable/vertretung-day-list.tsx|text-[11px]@343 text-[11px]@377 text-[11px]@384 text-[11px]@405 text-[11px]@411
src/components/timetable/vertretung-week-list.tsx|text-[10px]@157
src/components/timetable/weekly-calendar-grid.tsx|text-[10px]@253 text-[9px]@269 text-[10px]@325 text-[11px]@325 text-[11px]@330 text-[10px]@353 text-[10px]@389 text-[11px]@389
src/components/ui/avatar.tsx|text-[10px]@32
src/components/ui/coverage-indicator.tsx|text-[11px]@63 text-[11px]@165
src/components/ui/location-badge.tsx|text-[11px]@121 text-[11px]@122 text-[10px]@352 text-[10px]@386
src/components/ui/multi-checkbox-select.tsx|text-[11px]@246
src/components/ui/notification-badge.tsx|text-[10px]@15
src/components/ui/origin-chip.tsx|text-[11px]@25
src/components/ui/page-header/FilterButton.tsx|text-[10px]@61
src/components/ui/page-header/OverflowMenu.tsx|text-[10px]@344 text-[11px]@407
src/components/ui/parent-visible-badge.tsx|text-[11px]@44
src/components/ui/plan-block.tsx|text-[11px]@128 text-[11px]@135
src/components/ui/plan-legend.tsx|text-[11px]@107
src/components/ui/presence-badge.tsx|text-[11px]@78 text-[11px]@79 text-[10px]@259
`);

const noTinyText = makeClassStringRule({
  regex: TINY_TEXT_RE,
  baseline: restrictMatchBaseline(TINY_TEXT_BASELINE_FILES, TINY_TEXT_BASELINE),
  skipUiKit: false,
  docs: "Disallow sub-12px text utilities; the portal type floor is text-xs, and only for uppercase labels.",
  messageId: "tinyText",
  message:
    "'{{match}}' liegt unter dem Typo-Boden (12 px). Werte, Saetze und Bedienelemente sind text-sm oder groesser; text-xs nur fuer Versalien-Labels. Die Baseline in scripts/oxlint-plugin-ui-kit.mjs ist shrink-only.",
});
// ui-kit/no-tenant-kicker — die blaue Mini-Überschrift über einer Überschrift
// ist im Tenant-Portal abgeschafft (sie trug sechs verschiedene Bedeutungen im
// selben Slot). Kein Baseline-Eintrag: der Bestand ist in derselben PR auf
// null gebracht. Eltern-, Schul- und Operator-Portal sind nicht Teil dieser
// Umstellung und bekommen ihren eigenen Durchgang.
const OTHER_PORTALS_RE =
  /(^|\/)(components\/parent|app\/parents|components\/school|app\/school|components\/class-day|components\/operator|app\/operator)\//;

const noTenantKicker = {
  meta: {
    docs: {
      description:
        "Disallow the blue mini-heading (kicker) outside the parents portal.",
    },
    messages: {
      kicker:
        "Keine Mini-Überschrift über dem Titel. Wo man ist, sagen Brotkrumen und Seitenleiste (siehe .claude/rules/frontend-ui-kit.md, Abschnitt Page scaffolding).",
    },
  },
  create(context) {
    const filename = context.filename ?? context.getFilename?.() ?? "";
    if (OTHER_PORTALS_RE.test(filename.replace(/\\/g, "/"))) return {};
    if (/\.(test|stories)\.[jt]sx?$/.test(filename)) return {};

    return {
      JSXAttribute(node) {
        if (node.name?.name === "kicker") {
          context.report({ node, messageId: "kicker" });
        }
      },
    };
  },
};

// ui-kit/no-handrolled-surface — eine Seite baut keine Kartenfläche mehr aus
// einer eigenen Klassenkette. Für jede Form gibt es ein Kit-Bauteil:
// SectionCard (Fläche mit oder ohne Kopf), TileCard (anklickbare Kachel),
// StatCard (Kennzahl), TenantPageHeaderSkeleton (Ladezustand). Die Regel
// greift nur in Seitendateien des Tenant-Portals — Kit- und
// Komponentendateien dürfen die Fläche definieren, sie sind die Quelle.
const TENANT_PAGE_FILE_RE =
  /app\/\[tenant\]\/\(protected\)\/.*\/(page|loading)\.tsx$/;
// Nur die KARTE ist gemeint: `moto-content-surface` zusammen mit dem
// Kartenradius. Dieselbe Klasse an einer Pille oder einem Auswahlfeld
// (rounded-full, h-9) ist eine Bedienfläche und bleibt erlaubt.
const HANDROLLED_SURFACE_RE =
  /moto-content-surface[^"'`]*rounded-2xl|rounded-2xl[^"'`]*moto-content-surface/;

const noHandrolledSurface = {
  meta: {
    docs: {
      description:
        "Disallow hand-rolled card surfaces in tenant page files; use the kit cards.",
    },
    messages: {
      surface:
        "Keine eigene Kartenfläche in einer Seite. SectionCard (Fläche), TileCard (anklickbare Kachel), StatCard (Kennzahl) oder TenantPageHeaderSkeleton nehmen (siehe .claude/rules/frontend-ui-kit.md, Abschnitt Page scaffolding).",
    },
  },
  create(context) {
    const filename = (
      context.filename ??
      context.getFilename?.() ??
      ""
    ).replace(/\\/g, "/");
    if (!TENANT_PAGE_FILE_RE.test(filename)) return {};
    if (/\.(test|stories)\.[jt]sx?$/.test(filename)) return {};

    return {
      Literal(node) {
        if (
          typeof node.value === "string" &&
          HANDROLLED_SURFACE_RE.test(node.value)
        ) {
          context.report({ node, messageId: "surface" });
        }
      },
      TemplateElement(node) {
        const raw = node.value?.cooked ?? node.value?.raw ?? "";
        if (HANDROLLED_SURFACE_RE.test(raw)) {
          context.report({ node, messageId: "surface" });
        }
      },
    };
  },
};

// ui-kit/no-tabs-as-value-switcher — `ui/Tabs` ist Radix und schaltet
// INHALTE; eine Wertauswahl (Woche/Monat, A/B, Umfang) gehört auf
// SegmentedControl. Beides nebeneinander erzeugt zwei Bauarten für dieselbe
// Geste, und die Radix-Variante schaltet auf mousedown, was Klick-Tests still
// ins Leere laufen lässt.
const VALUE_SWITCHER_LABELS = new Set([
  "Woche",
  "Monat",
  "Tag",
  "Halbjahr",
  "Serien",
  "Woche A",
  "Woche B",
  "Ganzer Tag",
  "Dieser Block",
]);

const noTabsAsValueSwitcher = {
  meta: {
    docs: {
      description:
        "Disallow ui/Tabs for value choices; those belong on SegmentedControl.",
    },
    messages: {
      valueSwitcher:
        "Wertauswahl gehört auf SegmentedControl, nicht auf ui/Tabs (siehe .claude/rules/frontend-ui-kit.md, Abschnitt Page scaffolding).",
    },
  },
  create(context) {
    const filename = (
      context.filename ??
      context.getFilename?.() ??
      ""
    ).replace(/\\/g, "/");
    if (OTHER_PORTALS_RE.test(filename)) return {};
    if (/\.(test|stories)\.[jt]sx?$/.test(filename)) return {};

    return {
      JSXElement(node) {
        const name = node.openingElement?.name?.name;
        if (name !== "TabsTrigger") return;
        const child = node.children?.find((c) => c.type === "JSXText");
        const label = child?.value?.trim();
        if (label && VALUE_SWITCHER_LABELS.has(label)) {
          context.report({ node, messageId: "valueSwitcher" });
        }
      },
    };
  },
};

function jsxName(node) {
  return node?.type === "JSXIdentifier" ? node.name : null;
}

/**
 * `<ChoiceTile>` renders a native `<label>` unless its `as` prop says
 * otherwise, so a Checkbox inside the default tile already owns the whole
 * hit area. `as="button"` / `as="div"` tiles are not labels.
 */
function isLabelChoiceTile(openingElement) {
  if (jsxName(openingElement.name) !== "ChoiceTile") return false;
  const asAttribute = openingElement.attributes.find(
    (attribute) =>
      attribute.type === "JSXAttribute" &&
      attribute.name.type === "JSXIdentifier" &&
      attribute.name.name === "as",
  );
  if (!asAttribute) return true;
  return (
    asAttribute.value?.type === "Literal" && asAttribute.value.value === "label"
  );
}

function hasLabelAncestor(node) {
  for (let ancestor = node.parent; ancestor; ancestor = ancestor.parent) {
    if (ancestor.type !== "JSXElement") continue;
    if (
      jsxName(ancestor.openingElement.name) === "label" ||
      isLabelChoiceTile(ancestor.openingElement)
    ) {
      return true;
    }
  }
  return false;
}

const requireCheckboxLabel = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Require the shared Checkbox and its visible text to share one native label hit area.",
    },
    messages: {
      missingLabel:
        "Wrap Checkbox and its visible text in one <label>. A sibling label names the hidden input but leaves the visible box unclickable.",
    },
    schema: [],
  },
  create(context) {
    return {
      JSXOpeningElement(node) {
        if (jsxName(node.name) !== "Checkbox" || hasLabelAncestor(node)) {
          return;
        }
        context.report({ node, messageId: "missingLabel" });
      },
    };
  },
};

export default {
  meta: { name: "ui-kit" },
  rules: {
    "no-generic-brand-colors": noGenericBrandColors,
    "no-hand-rolled-overlay": noHandRolledOverlay,
    "no-hand-rolled-surface": noHandRolledSurface,
    "no-rounded-3xl": noRounded3xl,
    "no-tenant-kicker": noTenantKicker,
    "no-handrolled-surface": noHandrolledSurface,
    "no-tabs-as-value-switcher": noTabsAsValueSwitcher,
    "no-tiny-text": noTinyText,
    "require-checkbox-label": requireCheckboxLabel,
  },
};
