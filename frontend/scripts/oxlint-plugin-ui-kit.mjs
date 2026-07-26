// UI-kit drift ratchet (issue #1629).
//
// Three rules that stop NEW drift away from the shared UI kit while tolerating
// the existing stock via shrink-only allowlists:
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
// The allowlists below are SHRINK-ONLY: entries may be removed when a file is
// migrated, but never added. A new file (or a new violation in a clean file)
// must use the kit instead. Test/stories files are exempt — several assert on
// the legacy classes and are governed by the no-test-modifications rule.

const GENERIC_BRAND_COLOR_ALLOWLIST = new Set([
  "src/app/[tenant]/(protected)/active-supervisions/page.tsx",
  "src/app/[tenant]/(protected)/activities/page.tsx",
  "src/app/[tenant]/(protected)/dashboard/page.tsx",
  "src/app/[tenant]/(protected)/database/activities/page.tsx",
  "src/app/[tenant]/(protected)/database/devices/page.tsx",
  "src/app/[tenant]/(protected)/database/groups/page.tsx",
  "src/app/[tenant]/(protected)/database/permissions/page.tsx",
  "src/app/[tenant]/(protected)/database/personal/import/page.tsx",
  "src/app/[tenant]/(protected)/database/personal/page.tsx",
  "src/app/[tenant]/(protected)/database/roles/page.tsx",
  "src/app/[tenant]/(protected)/database/rooms/page.tsx",
  "src/app/[tenant]/(protected)/database/students/import/page.tsx",
  "src/app/[tenant]/(protected)/database/students/page.tsx",
  "src/app/[tenant]/(protected)/ogs-groups/page.tsx",
  "src/app/[tenant]/(protected)/rooms/page.tsx",
  "src/app/[tenant]/(protected)/staff/[id]/page.tsx",
  "src/app/[tenant]/(protected)/staff/page.tsx",
  "src/app/[tenant]/(protected)/students/[id]/feedback-history/page.tsx",
  "src/app/[tenant]/(protected)/students/[id]/page.tsx",
  "src/app/[tenant]/(protected)/students/[id]/room-history/page.tsx",
  "src/app/[tenant]/(protected)/students/search/page.tsx",
  "src/app/[tenant]/(protected)/substitutions/page.tsx",
  "src/app/[tenant]/(protected)/suggestions/page.tsx",
  "src/app/[tenant]/(protected)/time-tracking/page.tsx",
  "src/app/operator/announcements/page.tsx",
  "src/app/operator/devices/page.tsx",
  "src/app/operator/email-confirm/email-confirm-content.tsx",
  "src/app/operator/invite/invite-content.tsx",
  "src/app/operator/operators/page.tsx",
  "src/app/operator/persons/page.tsx",
  "src/app/operator/provisioning/create-account-modal.tsx",
  "src/app/operator/provisioning/create-device-modal.tsx",
  "src/app/operator/provisioning/create-organization-modal.tsx",
  "src/app/operator/provisioning/create-school-modal.tsx",
  "src/app/operator/provisioning/edit-organization-modal.tsx",
  "src/app/operator/provisioning/edit-school-modal.tsx",
  "src/app/operator/provisioning/invite-admin-modal.tsx",
  "src/app/operator/provisioning/provisioning-shared.tsx",
  "src/app/operator/provisioning/set-api-key-modal.tsx",
  "src/app/operator/provisioning/soft-delete-shared.tsx",
  "src/app/operator/schools/page.tsx",
  "src/app/operator/settings/page.tsx",
  "src/app/operator/suggestions/[id]/page.tsx",
  "src/app/operator/suggestions/page.tsx",
  "src/app/page.tsx",
  "src/components/active-supervisions/states.tsx",
  "src/components/active/unclaimed-rooms.tsx",
  "src/components/activities/activity-management-modal.tsx",
  "src/components/activities/quick-create-modal.tsx",
  "src/components/admin/guardian-approval-queue.tsx",
  "src/components/admin/invitation-form.tsx",
  "src/components/admin/pending-invitations-list.tsx",
  "src/components/auth/invitation-accept-form.tsx",
  "src/components/auth/invitation-page-content.tsx",
  "src/components/auth/mfa-admin-override-modal.tsx",
  "src/components/auth/reset-password-page-content.tsx",
  "src/components/auth/role-permission-management-modal.tsx",
  "src/components/dashboard/header/profile-dropdown.tsx",
  "src/components/dashboard/header/session-warning.tsx",
  "src/components/dashboard/sidebar.tsx",
  "src/components/database/configs/activities.config.tsx",
  "src/components/database/configs/devices.config.tsx",
  "src/components/database/configs/groups.config.tsx",
  "src/components/database/configs/roles.config.tsx",
  "src/components/database/configs/rooms.config.tsx",
  "src/components/database/configs/students.config.tsx",
  "src/components/database/configs/teachers.config.tsx",
  "src/components/database/detail-delete-button.tsx",
  "src/components/devices/devices-master-detail.tsx",
  "src/components/groups/group-transfer-modal.tsx",
  "src/components/guardians/guardian-form-modal.tsx",
  "src/components/guardians/guardian-list.tsx",
  "src/components/guardians/guardian-picker-panel.tsx",
  "src/components/guardians/guardian-relationship-fields.tsx",
  "src/components/guardians/student-guardian-manager.tsx",
  "src/components/import/student-row-card.tsx",
  "src/components/import/upload-section.tsx",
  "src/components/operator/announcement-views-accordion.tsx",
  "src/components/roles/roles-master-detail.tsx",
  "src/components/settings/passkey-settings-section.tsx",
  "src/components/settings/personalization-tab.tsx",
  "src/components/settings/settings-field.tsx",
  "src/components/settings/settings-page.tsx",
  "src/components/shared/base-comment-accordion.tsx",
  "src/components/sse/SSEErrorBoundary.tsx",
  "src/components/staff/abwesenheiten-tab.tsx",
  "src/components/staff/admin-session-edit-modal.tsx",
  "src/components/staff/arbeitszeitmodell-tab.tsx",
  "src/components/staff/school-overview-section.tsx",
  "src/components/staff/shift-edit-modal.tsx",
  "src/components/staff/staff-pending-inbox.tsx",
  "src/components/staff/staff-session-table.tsx",
  "src/components/staff/staff-time-accounts-table.tsx",
  "src/components/staff/staff-time-views.tsx",
  "src/components/staff/stundenkonto-panel.tsx",
  "src/components/students/arrival-day-edit-modal.tsx",
  "src/components/students/arrival-schedule-form-modal.tsx",
  "src/components/students/arrival-schedule-manager.tsx",
  "src/components/students/class-bulk-arrival-modal.tsx",
  "src/components/students/pickup-day-edit-modal.tsx",
  "src/components/students/pickup-schedule-form-modal.tsx",
  "src/components/students/pickup-schedule-manager.tsx",
  "src/components/students/privacy-consent-section.tsx",
  "src/components/students/student-card.tsx",
  "src/components/students/student-checkout-section.tsx",
  "src/components/students/student-create-modal.tsx",
  "src/components/students/student-detail-components.tsx",
  "src/components/students/student-form-fields.tsx",
  "src/components/students/student-historie-tab.tsx",
  "src/components/students/student-stammdaten-tab.tsx",
  "src/components/suggestions/suggestion-card.tsx",
  "src/components/suggestions/suggestion-form.tsx",
  "src/components/suggestions/vote-buttons.tsx",
  "src/components/teachers/caregiver-blocker-resolution-modal.tsx",
  "src/components/teachers/caregiver-capability-modal.tsx",
  "src/components/teachers/teacher-form.tsx",
  "src/components/time-tracking/edit-history-accordion.tsx",
  "src/components/time-tracking/leave-requests-card.tsx",
  "src/components/time-tracking/monatskarte.tsx",
  "src/components/time-tracking/vacation-request-modal.tsx",
  "src/components/ui/alert.tsx",
  "src/components/ui/button.tsx",
  "src/components/ui/database/accents.ts",
  "src/components/ui/database/database-form.tsx",
  "src/components/ui/database/database-select.tsx",
  "src/components/ui/date-picker.tsx",
  "src/components/ui/detail-modal-components.tsx",
  "src/components/ui/forbidden-page.tsx",
  "src/components/ui/input.tsx",
  "src/components/ui/page-header/ActiveFilterChips.tsx",
  "src/components/ui/page-header/FilterButton.tsx",
  "src/components/ui/page-header/FilterPanel.tsx",
  "src/components/ui/page-header/OverflowMenu.tsx",
  "src/components/ui/page-header/PageHeader.tsx",
  "src/components/ui/page-header/PageHeaderWithSearch.tsx",
  "src/components/ui/page-header/StatusIndicator.tsx",
  "src/components/ui/password-change-modal.tsx",
  "src/components/ui/textarea.tsx",
  "src/lib/dashboard-helpers.ts",
  "src/lib/database/themes.tsx",
  "src/lib/iot-helpers.ts",
  "src/lib/operator/suggestions-helpers.ts",
  "src/lib/suggestions-helpers.ts",
]);

const OVERLAY_ALLOWLIST = new Set([
  "src/app/[tenant]/(protected)/time-tracking/page.tsx",
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

const ROUNDED_3XL_ALLOWLIST = new Set([
  "src/app/[tenant]/(protected)/dashboard/page-skeleton.tsx",
  "src/app/[tenant]/(protected)/dashboard/page.tsx",
  "src/app/[tenant]/(protected)/rooms/page-skeleton.tsx",
  "src/app/[tenant]/(protected)/rooms/page.tsx",
  "src/app/[tenant]/(protected)/staff/[id]/page.tsx",
  "src/app/[tenant]/(protected)/students/[id]/change-history/page.tsx",
  "src/app/[tenant]/(protected)/students/[id]/feedback-history/page-skeleton.tsx",
  "src/app/[tenant]/(protected)/students/[id]/feedback-history/page.tsx",
  "src/app/[tenant]/(protected)/students/[id]/room-history/page-skeleton.tsx",
  "src/app/[tenant]/(protected)/students/[id]/room-history/page.tsx",
  "src/app/[tenant]/(protected)/suggestions/page-skeleton.tsx",
  "src/app/[tenant]/(protected)/time-tracking/page.tsx",
  "src/app/[tenant]/(public)/enroll/[phaseId]/page.tsx",
  "src/app/[tenant]/(public)/enroll/page.tsx",
  "src/app/[tenant]/(public)/enroll/preview/page.tsx",
  "src/app/[tenant]/(public)/enroll/submitted/page.tsx",
  "src/app/help/nfc/erste-schritte/page.tsx",
  "src/app/operator/announcements/page.tsx",
  "src/app/operator/provisioning/provisioning-shared.tsx",
  "src/app/operator/provisioning/soft-delete-shared.tsx",
  "src/app/operator/suggestions/[id]/page.tsx",
  "src/app/operator/suggestions/page.tsx",
  "src/components/enrollment/enrollment-status-view.tsx",
  "src/components/parent/child-detail.tsx",
  "src/components/rooms/room-detail-content.tsx",
  "src/components/rooms/students-in-room-section.tsx",
  "src/components/rooms/transit-students-section.tsx",
  "src/components/staff/abwesenheiten-tab.tsx",
  "src/components/staff/arbeitszeitmodell-tab.tsx",
  "src/components/staff/staff-time-views.tsx",
  "src/components/staff/stundenkonto-panel.tsx",
  "src/components/staff/uebersicht-tab-skeleton.tsx",
  "src/components/staff/uebersicht-tab.tsx",
  "src/components/staff/zeiterfassung-tab.tsx",
  "src/components/students/student-checkout-section.tsx",
  "src/components/suggestions/suggestion-card.tsx",
  "src/components/time-tracking/leave-requests-card.tsx",
]);

const BRAND_COLOR_RE =
  /\b(?:text|bg|border)-(?:red|green|blue|orange|purple|amber|yellow|emerald)-\d/;
const OVERLAY_RE = /(?:\bfixed\b.*\binset-0\b)|(?:\binset-0\b.*\bfixed\b)/;
const ROUNDED_3XL_RE = /\brounded-3xl\b/;

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
 * Builds a rule that reports string/template-literal chunks matching `regex`,
 * except in exempt files (tests/stories), allowlisted files, and (optionally)
 * the UI kit directory itself.
 */
function makeClassStringRule({ regex, allowlist, skipUiKit, docs, messageId, message }) {
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
      if (allowlist.has(key)) return {};

      const check = (node, text) => {
        const match = regex.exec(text);
        if (match) {
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
            if (regex.test(quasi.value.raw)) return; // one report per template
          }
        },
      };
    },
  };
}

const noGenericBrandColors = makeClassStringRule({
  regex: BRAND_COLOR_RE,
  allowlist: GENERIC_BRAND_COLOR_ALLOWLIST,
  skipUiKit: false,
  docs:
    "Disallow generic Tailwind brand-color utilities; brand semantics use the LOCATION_COLORS hexes or a kit component.",
  messageId: "genericBrandColor",
  message:
    "Generic Tailwind hue '{{match}}…' is not a brand color. Use the hex from LOCATION_COLORS (~/lib/location-helper) via bg-[#…], or a kit component (see .claude/rules/frontend-ui-kit.md). The allowlist in scripts/oxlint-plugin-ui-kit.mjs is shrink-only.",
});

const noHandRolledOverlay = makeClassStringRule({
  regex: OVERLAY_RE,
  allowlist: OVERLAY_ALLOWLIST,
  skipUiKit: true,
  docs:
    "Disallow hand-rolled full-screen overlays (`fixed inset-0`) outside the UI kit; use Modal, Drawer, ConfirmationModal, or OverflowMenu.",
  messageId: "handRolledOverlay",
  message:
    "Hand-rolled full-screen overlay ('{{match}}'). Use the kit Modal / Drawer / OverflowMenu instead of a bespoke fixed inset-0 layer. The allowlist in scripts/oxlint-plugin-ui-kit.mjs is shrink-only.",
});

const noRounded3xl = makeClassStringRule({
  regex: ROUNDED_3XL_RE,
  allowlist: ROUNDED_3XL_ALLOWLIST,
  skipUiKit: false,
  docs:
    "Disallow rounded-3xl surfaces; the canonical card radius is rounded-2xl (moto-content-surface).",
  messageId: "rounded3xl",
  message:
    "rounded-3xl is off the brand radius scale. Cards/panels use rounded-2xl via moto-content-surface (see .claude/rules/frontend-ui-kit.md). The allowlist in scripts/oxlint-plugin-ui-kit.mjs is shrink-only.",
});

export default {
  meta: { name: "ui-kit" },
  rules: {
    "no-generic-brand-colors": noGenericBrandColors,
    "no-hand-rolled-overlay": noHandRolledOverlay,
    "no-rounded-3xl": noRounded3xl,
  },
};
