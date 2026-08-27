// UI-kit drift ratchet (issue #1629).
//
// Three rules that stop NEW drift away from the shared UI kit while tolerating
// the existing stock via shrink-only, per-match baselines:
//
//   ui-kit/no-generic-brand-colors  — generic Tailwind hues (bg-green-500 …)
//                                     used for brand semantics; use the brand
//                                     hex from LOCATION_COLORS or a kit
//                                     component (.claude/rules/frontend-ui-kit.md)
//   ui-kit/no-hand-rolled-overlay   — `fixed inset-0` overlays outside
//                                     src/components/ui/; use Modal / Drawer /
//                                     OverflowMenu from the kit
//   ui-kit/no-rounded-3xl           — off-scale surface radius; cards are
//                                     rounded-2xl (moto-content-surface)
//
// The baselines below are SHRINK-ONLY: matches may be removed when a file is
// migrated, but never added. Every existing utility is tracked by value and
// count, so baselined files cannot accumulate more drift. Test/stories files
// are exempt — several assert on the legacy classes and are governed by the
// no-test-modifications rule.

const GENERIC_BRAND_COLOR_BASELINE_FILES = new Set([
  "src/app/[tenant]/(protected)/time-tracking/page.tsx",
  "src/app/operator/schools/page.tsx",
  "src/app/operator/settings/page.tsx",
  "src/app/page.tsx",
  "src/components/activities/activity-management-modal.tsx",
  "src/components/activities/quick-create-modal.tsx",
  "src/components/auth/invitation-accept-form.tsx",
  "src/components/auth/invitation-page-content.tsx",
  "src/components/auth/mfa-admin-override-modal.tsx",
  "src/components/auth/reset-password-page-content.tsx",
  "src/components/auth/role-permission-management-modal.tsx",
  "src/components/dashboard/header/profile-dropdown.tsx",
  "src/components/dashboard/header/session-warning.tsx",
  "src/components/dashboard/sidebar.tsx",
  "src/components/database/configs/devices.config.tsx",
  "src/components/devices/devices-master-detail.tsx",
  "src/components/guardians/guardian-form-modal.tsx",
  "src/components/guardians/guardian-picker-panel.tsx",
  "src/components/guardians/guardian-relationship-fields.tsx",
  "src/components/import/upload-section.tsx",
  "src/components/permissions/permission-selector.tsx",
  "src/components/settings/passkey-settings-section.tsx",
  "src/components/settings/personalization-tab.tsx",
  "src/components/settings/settings-field.tsx",
  "src/components/staff/staff-session-table.tsx",
  "src/components/students/class-bulk-arrival-modal.tsx",
  "src/components/students/personal-info-form-modal.tsx",
  "src/components/students/student-create-modal.tsx",
  "src/components/students/student-form-fields.tsx",
  "src/components/time-tracking/leave-requests-card.tsx",
  "src/components/ui/database/database-form.tsx",
  "src/components/ui/database/database-select.tsx",
  "src/components/ui/date-picker.tsx",
  "src/components/ui/page-header/ActiveFilterChips.tsx",
  "src/components/ui/page-header/DesktopFilters.tsx",
  "src/components/ui/page-header/FilterPanel.tsx",
  "src/components/ui/page-header/OverflowMenu.tsx",
  "src/components/ui/page-header/PageHeaderWithSearch.tsx",
  "src/components/ui/password-change-modal.tsx",
  "src/lib/activity-helpers.ts",
  "src/lib/iot-helpers.ts",
  "src/lib/staff-helpers.ts",
]);

const GENERIC_BRAND_COLOR_BASELINE = parseMatchBaseline(`
src/app/[tenant]/(protected)/time-tracking/page.tsx|text-amber-600
src/app/operator/schools/page.tsx|bg-red-100 bg-red-200 text-red-700
src/app/operator/settings/page.tsx|bg-red-50 text-red-700
src/app/page.tsx|bg-red-50 border-red-200 text-red-700
src/components/activities/activity-management-modal.tsx|text-red-600
src/components/activities/quick-create-modal.tsx|ring-red-400 text-red-600:3
src/components/auth/invitation-accept-form.tsx|bg-red-50/50 border-red-200/50 ring-red-300:4 text-red-600:5 text-red-700
src/components/auth/invitation-page-content.tsx|bg-red-50 border-red-200 text-red-600 text-red-700
src/components/auth/mfa-admin-override-modal.tsx|bg-amber-50 bg-red-50:3 border-amber-200 text-amber-900 text-red-700:3
src/components/auth/reset-password-page-content.tsx|bg-red-50 border-red-200 text-red-600 text-red-700
src/components/auth/role-permission-management-modal.tsx|bg-purple-50:2 bg-purple-50/30 bg-purple-600:2 bg-purple-700 border-purple-400:2 ring-purple-500:2 text-purple-600:2
src/components/dashboard/header/profile-dropdown.tsx|bg-red-50 bg-red-600 text-red-600 text-red-700
src/components/dashboard/header/session-warning.tsx|bg-red-50 border-red-200 text-red-600:2 text-red-800
src/components/dashboard/sidebar.tsx|text-violet-500:2
src/components/database/configs/devices.config.tsx|bg-green-500
src/components/devices/devices-master-detail.tsx|bg-yellow-50 border-yellow-200 text-yellow-800
src/components/guardians/guardian-form-modal.tsx|bg-blue-50/30:4 bg-red-50:4 border-red-200 border-red-400:4 text-blue-600:4 text-red-500:2 text-red-600:6 text-red-800 text-yellow-400 text-yellow-500
src/components/guardians/guardian-picker-panel.tsx|bg-blue-50/40:2 bg-red-50 border-red-200 text-blue-600 text-red-800
src/components/guardians/guardian-relationship-fields.tsx|bg-blue-50/30 bg-red-50 border-red-200 ring-green-600 ring-purple-600 ring-red-600 text-blue-600 text-green-600 text-purple-600 text-red-600:2 text-red-900
src/components/import/upload-section.tsx|text-green-600
src/components/permissions/permission-selector.tsx|text-pink-600
src/components/settings/passkey-settings-section.tsx|bg-red-50 text-red-700
src/components/settings/personalization-tab.tsx|bg-green-50 bg-red-50 border-green-500 border-red-200 ring-green-500 text-green-500 text-red-600
src/components/settings/settings-field.tsx|bg-amber-50 bg-yellow-50 border-amber-200 text-amber-700 text-amber-800 text-red-600:4 text-yellow-700
src/components/staff/staff-session-table.tsx|text-amber-600
src/components/students/class-bulk-arrival-modal.tsx|bg-red-50 border-red-300 text-red-600
src/components/students/personal-info-form-modal.tsx|ring-blue-500:2
src/components/students/student-create-modal.tsx|bg-red-50:3 border-red-200 text-red-600:2 text-red-800
src/components/students/student-form-fields.tsx|bg-blue-50/30 bg-red-50:2 border-red-300:2 text-red-500 text-red-600:2
src/components/time-tracking/leave-requests-card.tsx|text-red-600 text-red-700
src/components/ui/database/database-form.tsx|bg-blue-100 bg-blue-200 bg-blue-300 text-blue-600 text-blue-700 text-blue-800 text-red-600:2
src/components/ui/database/database-select.tsx|text-red-600
src/components/ui/date-picker.tsx|text-blue-600:2
src/components/ui/page-header/ActiveFilterChips.tsx|bg-blue-100 text-blue-600 text-blue-700:2 text-blue-900
src/components/ui/page-header/DesktopFilters.tsx|ring-blue-500
src/components/ui/page-header/FilterPanel.tsx|bg-blue-50 ring-blue-200 text-blue-700
src/components/ui/page-header/OverflowMenu.tsx|ring-blue-500/50 text-red-600
src/components/ui/page-header/PageHeaderWithSearch.tsx|bg-blue-50:2 ring-blue-200:2 text-blue-700:2
src/components/ui/password-change-modal.tsx|bg-blue-50 border-blue-200 border-red-400:3 text-red-600:3
src/lib/activity-helpers.ts|from-blue-500 from-green-500 from-green-600 from-orange-500 from-pink-500 from-purple-500 from-red-500 from-yellow-500 to-amber-600 to-emerald-600 to-indigo-600 to-orange-600 to-pink-600:2 to-rose-600 to-teal-600
src/lib/iot-helpers.ts|bg-green-100:2 bg-red-100:2 bg-yellow-100 text-green-800:2 text-red-800:2 text-yellow-800
src/lib/staff-helpers.ts|from-sky-50/80 to-sky-100/80
`);

const OVERLAY_BASELINE_FILES = new Set([
  "src/app/operator/devices/page.tsx",
  "src/app/operator/persons/page.tsx",
  "src/app/operator/provisioning/soft-delete-shared.tsx",
  "src/app/operator/unregistered-tags/page.tsx",
  "src/components/auth/mfa-admin-override-modal.tsx",
  "src/components/background-wrapper.tsx",
  "src/components/dashboard/header/profile-dropdown.tsx",
  "src/components/enrollment/admin-enrollment-detail.tsx",
  "src/contexts/ToastContext.tsx",
]);

const ROUNDED_3XL_BASELINE_FILES = new Set([
  "src/app/[tenant]/(protected)/students/[id]/change-history/page.tsx",
  "src/app/[tenant]/(protected)/students/[id]/feedback-history/page-skeleton.tsx",
  "src/app/[tenant]/(protected)/students/[id]/feedback-history/page.tsx",
  "src/app/[tenant]/(protected)/students/[id]/room-history/page-skeleton.tsx",
  "src/app/[tenant]/(protected)/students/[id]/room-history/page.tsx",
  "src/app/[tenant]/(public)/enroll/[phaseId]/page.tsx",
  "src/app/[tenant]/(public)/enroll/page.tsx",
  "src/app/[tenant]/(public)/enroll/preview/page.tsx",
  "src/app/[tenant]/(public)/enroll/submitted/page.tsx",
  "src/app/help/nfc/erste-schritte/page.tsx",
  "src/app/operator/announcements/page.tsx",
  "src/app/operator/provisioning/provisioning-shared.tsx",
  "src/app/operator/provisioning/soft-delete-shared.tsx",
  "src/components/enrollment/enrollment-status-view.tsx",
  "src/components/parent/child-detail.tsx",
  "src/components/rooms/room-detail-content.tsx",
  "src/components/rooms/students-in-room-section.tsx",
  "src/components/rooms/transit-students-section.tsx",
  "src/components/students/student-checkout-section.tsx",
]);

const OVERLAY_BASELINE = new Map([
  ["src/app/operator/devices/page.tsx", 1],
  ["src/app/operator/persons/page.tsx", 1],
  ["src/app/operator/provisioning/soft-delete-shared.tsx", 1],
  ["src/app/operator/unregistered-tags/page.tsx", 1],
  ["src/components/auth/mfa-admin-override-modal.tsx", 1],
  ["src/components/background-wrapper.tsx", 1],
  ["src/components/dashboard/header/profile-dropdown.tsx", 1],
  ["src/components/enrollment/admin-enrollment-detail.tsx", 1],
  ["src/contexts/ToastContext.tsx", 2],
]);

const ROUNDED_3XL_BASELINE = new Map([
  ["src/app/[tenant]/(public)/enroll/[phaseId]/page.tsx", 4],
  ["src/app/[tenant]/(public)/enroll/page.tsx", 2],
  ["src/app/[tenant]/(public)/enroll/preview/page.tsx", 4],
  ["src/app/[tenant]/(public)/enroll/submitted/page.tsx", 1],
  ["src/app/help/nfc/erste-schritte/page.tsx", 3],
  ["src/app/operator/announcements/page.tsx", 2],
  ["src/app/operator/provisioning/provisioning-shared.tsx", 1],
  ["src/app/operator/provisioning/soft-delete-shared.tsx", 1],
  ["src/components/enrollment/enrollment-status-view.tsx", 1],
  ["src/components/rooms/room-detail-content.tsx", 1],
  ["src/components/rooms/students-in-room-section.tsx", 1],
  ["src/components/rooms/transit-students-section.tsx", 1],
  ["src/components/students/student-checkout-section.tsx", 5],
]);

const BRAND_COLOR_RE =
  /\b(?:text|bg|border|ring|outline|fill|stroke|from|via|to|divide|accent|caret|decoration|shadow)-(?:red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-\d+(?:\/(?:\d+|\[[^\]\s]+\]))?(?![\w-])/g;
const FIXED_RE = /\bfixed\b/;
const INSET_0_RE = /\binset-0\b/;
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

/**
 * Parse compact `token[:count]` rows into a per-file match baseline.
 */
function parseMatchBaseline(source) {
  return new Map(
    source
      .trim()
      .split("\n")
      .map((row) => {
        const [file, encoded] = row.split("|");
        const matches = new Map(
          encoded.split(" ").map((entry) => {
            const parsed = /^(.*?)(?::(\d+))?$/.exec(entry);
            return [parsed[1], Number(parsed[2] ?? 1)];
          }),
        );
        return [file, matches];
      }),
  );
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

function consumeBaselineMatch(seenMatches, baseline, key, match) {
  const nextCount = (seenMatches.get(match) ?? 0) + 1;
  seenMatches.set(match, nextCount);
  return nextCount <= (baseline.get(key)?.get(match) ?? 0);
}

function restrictMatchBaseline(baselineFiles, baseline) {
  return new Map([...baseline].filter(([file]) => baselineFiles.has(file)));
}

function expandCountBaseline(baselineFiles, baseline, match) {
  return new Map(
    [...baseline]
      .filter(([file]) => baselineFiles.has(file))
      .map(([file, count]) => [file, new Map([[match, count]])]),
  );
}

/**
 * Builds a rule that reports string/template-literal chunks matching `regex`,
 * except for individual legacy matches recorded in the shrink-only baseline.
 */
function makeClassStringRule({
  regex,
  baseline,
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
      const seenMatches = new Map();

      const check = (node, text) => {
        regex.lastIndex = 0;
        for (const match of text.matchAll(regex)) {
          if (!consumeBaselineMatch(seenMatches, baseline, key, match[0])) {
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
            check(node, quasi.value.raw);
          }
        },
      };
    },
  };
}

const noGenericBrandColors = makeClassStringRule({
  regex: BRAND_COLOR_RE,
  baseline: restrictMatchBaseline(
    GENERIC_BRAND_COLOR_BASELINE_FILES,
    GENERIC_BRAND_COLOR_BASELINE,
  ),
  skipUiKit: false,
  docs: "Disallow generic Tailwind brand-color utilities; brand semantics use the LOCATION_COLORS hexes or a kit component.",
  messageId: "genericBrandColor",
  message:
    "Generic Tailwind hue '{{match}}…' is not a brand color. Use the hex from LOCATION_COLORS (~/lib/location-helper) via bg-[#…], or a kit component (see .claude/rules/frontend-ui-kit.md). The baseline in scripts/oxlint-plugin-ui-kit.mjs is shrink-only.",
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
        "Hand-rolled full-screen overlay ('{{match}}'). Use the kit Modal / Drawer / OverflowMenu instead of a bespoke fixed inset-0 layer. The baseline in scripts/oxlint-plugin-ui-kit.mjs is shrink-only.",
    },
    schema: [],
  },
  create(context) {
    const key = fileKey(context);
    if (!key.startsWith("src/")) return {};
    if (EXEMPT_FILE_RE.test(key)) return {};
    if (key.startsWith(UI_KIT_DIR)) return {};

    const baseline = OVERLAY_BASELINE_FILES.has(key)
      ? (OVERLAY_BASELINE.get(key) ?? 0)
      : 0;
    let seenMatches = 0;

    const check = (node, text) => {
      if (!FIXED_RE.test(text) || !INSET_0_RE.test(text)) return;

      seenMatches += 1;
      if (seenMatches > baseline) {
        context.report({
          node,
          messageId: "handRolledOverlay",
          data: { match: "fixed … inset-0" },
        });
      }
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

const noRounded3xl = makeClassStringRule({
  regex: ROUNDED_3XL_RE,
  baseline: expandCountBaseline(
    ROUNDED_3XL_BASELINE_FILES,
    ROUNDED_3XL_BASELINE,
    "rounded-3xl",
  ),
  skipUiKit: false,
  docs: "Disallow rounded-3xl surfaces; the canonical card radius is rounded-2xl (moto-content-surface).",
  messageId: "rounded3xl",
  message:
    "rounded-3xl is off the brand radius scale. Cards/panels use rounded-2xl via moto-content-surface (see .claude/rules/frontend-ui-kit.md). The baseline in scripts/oxlint-plugin-ui-kit.mjs is shrink-only.",
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
const TENANT_PAGE_FILE_RE = /app\/\[tenant\]\/\(protected\)\/.*\/(page|loading)\.tsx$/;
// Nur die KARTE ist gemeint: `moto-content-surface` zusammen mit dem
// Kartenradius. Dieselbe Klasse an einer Pille oder einem Auswahlfeld
// (rounded-full, h-9) ist eine Bedienfläche und bleibt erlaubt.
const HANDROLLED_SURFACE_RE = /moto-content-surface[^"'`]*rounded-2xl|rounded-2xl[^"'`]*moto-content-surface/;

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
    const filename = (context.filename ?? context.getFilename?.() ?? "").replace(
      /\\/g,
      "/",
    );
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
    const filename = (context.filename ?? context.getFilename?.() ?? "").replace(
      /\\/g,
      "/",
    );
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

export default {
  meta: { name: "ui-kit" },
  rules: {
    "no-generic-brand-colors": noGenericBrandColors,
    "no-hand-rolled-overlay": noHandRolledOverlay,
    "no-rounded-3xl": noRounded3xl,
    "no-tenant-kicker": noTenantKicker,
    "no-handrolled-surface": noHandrolledSurface,
    "no-tabs-as-value-switcher": noTabsAsValueSwitcher,
  },
};
