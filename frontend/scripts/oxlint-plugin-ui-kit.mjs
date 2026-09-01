// UI-kit drift ratchet (issue #1629).
//
// Five rules that stop drift away from the shared UI kit. Hand-rolled card
// surfaces tolerate existing stock via a shrink-only baseline; generic brand
// colors, hand-rolled overlays, rounded-3xl and unlabeled checkboxes are
// hard-zero:
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
//                                     moto-content-surface or a kit surface
//                                     component (issue #2933)
//   ui-kit/no-rounded-3xl           — off-scale surface radius; cards are
//                                     rounded-2xl (moto-content-surface)
//   ui-kit/require-checkbox-label   — every shared Checkbox is wrapped by a
//                                     label so its visible box is clickable
//
// The surface baseline below is SHRINK-ONLY: matches may be removed when a
// file is migrated, but never added. Test/stories files are exempt from the
// class-string rules.

// Filled by the ratchet introduction (issue #2933): per-file count of
// hand-rolled surfaces existing at rule-introduction time. Shrink-only.
const SURFACE_BASELINE = new Map([
  ["src/app/[tenant]/(protected)/activities/page.tsx", 1],
  ["src/app/[tenant]/(protected)/database/page-skeleton.tsx", 1],
  ["src/app/[tenant]/(protected)/database/page.tsx", 1],
  ["src/app/[tenant]/(protected)/database/personal/import/page.tsx", 4],
  ["src/app/[tenant]/(protected)/database/personal/opening-balances/page.tsx", 7],
  ["src/app/[tenant]/(protected)/database/students/class-list/import/page.tsx", 1],
  ["src/app/[tenant]/(protected)/database/students/import/page.tsx", 4],
  ["src/app/[tenant]/(protected)/day-log/page.tsx", 1],
  ["src/app/[tenant]/(protected)/info-displays/page.tsx", 1],
  ["src/app/[tenant]/(protected)/lists/page.tsx", 5],
  ["src/app/[tenant]/(protected)/meal-plan/page.tsx", 1],
  ["src/app/[tenant]/(protected)/messages/[threadId]/page.tsx", 1],
  ["src/app/[tenant]/(protected)/messages/page-skeleton.tsx", 1],
  ["src/app/[tenant]/(protected)/messages/page.tsx", 1],
  ["src/app/[tenant]/(protected)/reminders/page.tsx", 2],
  ["src/app/[tenant]/(protected)/rooms/page.tsx", 2],
  ["src/app/[tenant]/(protected)/staff/page-skeleton.tsx", 1],
  ["src/app/[tenant]/(protected)/staff/page.tsx", 1],
  ["src/app/[tenant]/(protected)/students/[id]/page-skeleton.tsx", 1],
  ["src/app/[tenant]/(protected)/time-tracking/page.tsx", 1],
  ["src/app/[tenant]/(public)/enroll/page.tsx", 1],
  ["src/app/help/nfc/erste-schritte/page.tsx", 1],
  ["src/app/operator/announcements/page.tsx", 1],
  ["src/app/operator/email-confirm/email-confirm-content.tsx", 3],
  ["src/app/operator/operators/page.tsx", 3],
  ["src/app/operator/organizations/[slug]/page.tsx", 1],
  ["src/app/operator/organizations/[slug]/schools/[schoolSlug]/page.tsx", 2],
  ["src/app/operator/organizations/page.tsx", 1],
  ["src/app/operator/persons/page.tsx", 1],
  ["src/app/operator/schools/[id]/settings/page.tsx", 1],
  ["src/app/operator/settings/page.tsx", 2],
  ["src/app/parents/accept-guardian-invite/[token]/page.tsx", 1],
  ["src/app/start/page.tsx", 2],
  ["src/components/active-supervisions/planned-now-section.tsx", 1],
  ["src/components/active-supervisions/spontaneous-activity-start.tsx", 1],
  ["src/components/activities/quick-create-modal.tsx", 2],
  ["src/components/admin/guardian-approval-queue.tsx", 2],
  ["src/components/admin/invitation-form.tsx", 1],
  ["src/components/admin/pending-invitations-list.tsx", 2],
  ["src/components/auth/auth-shell.tsx", 3],
  ["src/components/auth/role-permission-management-modal.tsx", 1],
  ["src/components/calendar/personal-calendar.tsx", 1],
  ["src/components/class-day/student-row.tsx", 1],
  ["src/components/dashboard/header/profile-dropdown.tsx", 1],
  ["src/components/dashboard/header/reminders-bell.tsx", 1],
  ["src/components/database/grade-transitions/transition-preview-modal.tsx", 3],
  ["src/components/database/master-detail-layout.tsx", 3],
  ["src/components/display/activities-panel.tsx", 2],
  ["src/components/display/pickup-times-panel.tsx", 1],
  ["src/components/display/room-occupancy-panel.tsx", 1],
  ["src/components/enrollment/admin-enrollment-deletion-modal.tsx", 1],
  ["src/components/enrollment/admin-enrollment-phase-detail.tsx", 3],
  ["src/components/enrollment/admin-enrollments-list.tsx", 1],
  ["src/components/enrollment/care-offerings-editor.tsx", 1],
  ["src/components/enrollment/enrollment-form-editor.tsx", 7],
  ["src/components/enrollment/enrollment-form.tsx", 3],
  ["src/components/enrollment/enrollment-status-view.tsx", 7],
  ["src/components/enrollment/phase-enrollment-actions.tsx", 1],
  ["src/components/enrollment/phases-editor.tsx", 1],
  ["src/components/enrollment/rollover-form.tsx", 1],
  ["src/components/groups/group-transfer-modal.tsx", 1],
  ["src/components/guardians/student-guardian-manager.tsx", 1],
  ["src/components/help/guide-components.tsx", 8],
  ["src/components/help/help-search.tsx", 2],
  ["src/components/import/student-row-card.tsx", 1],
  ["src/components/import/upload-section.tsx", 1],
  ["src/components/messaging/team-chat-inbox.tsx", 1],
  ["src/components/messaging/team-chat-skeletons.tsx", 1],
  ["src/components/operator/account-tenant-access-modal.tsx", 2],
  ["src/components/operator/entity-header-card.tsx", 1],
  ["src/components/parent/child/child-day-card.tsx", 1],
  ["src/components/parent/child/weekly-schedule-section.tsx", 1],
  ["src/components/parent/language-switcher.tsx", 1],
  ["src/components/parent/messages/parent-messages-page.tsx", 1],
  ["src/components/parent/news/news-components.tsx", 4],
  ["src/components/parent/news/parent-news-page.tsx", 1],
  ["src/components/parent/ogs-conversation.tsx", 2],
  ["src/components/parent/parent-enroll-picker.tsx", 2],
  ["src/components/parent/parent-page.tsx", 1],
  ["src/components/parent/start/parent-start-page.tsx", 1],
  ["src/components/rooms/room-detail-content.tsx", 1],
  ["src/components/rooms/students-in-room-section.tsx", 3],
  ["src/components/rooms/transit-students-section.tsx", 3],
  ["src/components/school/supervisions/supervisions-overview.tsx", 1],
  ["src/components/settings/settings-field.tsx", 2],
  ["src/components/settings/trusted-devices-section.tsx", 1],
  ["src/components/staff/abwesenheiten-tab.tsx", 6],
  ["src/components/staff/arbeitszeitmodell-tab.tsx", 2],
  ["src/components/staff/month-close-modal.tsx", 1],
  ["src/components/staff/staff-export-button.tsx", 1],
  ["src/components/staff/stundenkonto-panel.tsx", 4],
  ["src/components/staff/use-absence-type-select.tsx", 1],
  ["src/components/students/care-exit-modal.tsx", 1],
  ["src/components/students/care-plan-editor-modal.tsx", 2],
  ["src/components/students/care-plan-view.tsx", 1],
  ["src/components/students/care-resume-modal.tsx", 1],
  ["src/components/students/care-schedule-manager.tsx", 1],
  ["src/components/students/care-weekly-plan-modal.tsx", 1],
  ["src/components/students/requests/conflict-decision-group.tsx", 1],
  ["src/components/students/school-checkin-mode-mobile.tsx", 1],
  ["src/components/students/student-card-skeleton.tsx", 1],
  ["src/components/students/student-card.tsx", 1],
  ["src/components/students/student-deletion-modal.tsx", 1],
  ["src/components/students/student-export-modal.tsx", 2],
  ["src/components/students/student-form-fields.tsx", 1],
  ["src/components/time-tracking/leave-requests-card.tsx", 1],
  ["src/components/time-tracking/vacation-request-modal.tsx", 4],
  ["src/components/timetable/betreuungsplan-skeleton.tsx", 2],
  ["src/components/timetable/bulk-substitution-modal.tsx", 1],
  ["src/components/timetable/calendar-period-modal.tsx", 1],
  ["src/components/timetable/event-form/weekday-roster-section.tsx", 1],
  ["src/components/timetable/staff-pool-slide-over.tsx", 1],
  ["src/components/timetable/substitution-person-card.tsx", 1],
  ["src/components/timetable/substitution-slide-over.tsx", 2],
  ["src/components/timetable/tagesplan-view.tsx", 1],
  ["src/components/timetable/timetable-style.ts", 3],
  ["src/components/timetable/weekly-calendar-grid.tsx", 1],
]);

const BRAND_COLOR_RE =
  /\b(?:text|bg|border(?:-[trblxyse])?|ring|outline|fill|stroke|from|via|to|divide|accent|caret|decoration|shadow)-(?:red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-\d+(?:\/(?:\d+|\[[^\]\s]+\]))?(?![\w-])/g;
const FIXED_RE = /\bfixed\b/;
const INSET_0_RE = /\binset-0\b/;
const SURFACE_ROUNDED_RE = /\brounded-(?:xl|2xl)\b/;
const SURFACE_BORDER_RE = /\bborder\b/;
const SURFACE_BG_RE = /\bbg-white\b/;
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
 * Builds a hard-zero rule that reports string/template-literal chunks matching
 * `regex`.
 */
function makeClassStringRule({ regex, skipUiKit, docs, messageId, message }) {
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

      const check = (node, text) => {
        regex.lastIndex = 0;
        for (const match of text.matchAll(regex)) {
          context.report({ node, messageId, data: { match: match[0] } });
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
        "Disallow hand-built card surfaces (`rounded-xl/2xl` + `border` + `bg-white`) outside the UI kit; use moto-content-surface or a kit surface component (InfoCard, SectionCard).",
    },
    messages: {
      handRolledSurface:
        "Hand-rolled card surface ('{{match}}'). Use the moto-content-surface utility or a kit surface component (InfoCard, SectionCard) instead. The baseline in scripts/oxlint-plugin-ui-kit.mjs is shrink-only (issue #2933).",
    },
    schema: [],
  },
  create(context) {
    const key = fileKey(context);
    if (!key.startsWith("src/")) return {};
    if (EXEMPT_FILE_RE.test(key)) return {};
    if (key.startsWith(UI_KIT_DIR)) return {};

    const baseline = SURFACE_BASELINE.get(key) ?? 0;
    let seenMatches = 0;

    const check = (node, text) => {
      if (!isSurfaceCombo(text)) return;

      seenMatches += 1;
      if (seenMatches > baseline) {
        context.report({
          node,
          messageId: "handRolledSurface",
          data: { match: "rounded-xl/2xl … border … bg-white" },
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

function jsxName(node) {
  return node?.type === "JSXIdentifier" ? node.name : null;
}

function hasLabelAncestor(node) {
  for (let ancestor = node.parent; ancestor; ancestor = ancestor.parent) {
    if (
      ancestor.type === "JSXElement" &&
      jsxName(ancestor.openingElement.name) === "label"
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
    "require-checkbox-label": requireCheckboxLabel,
  },
};
