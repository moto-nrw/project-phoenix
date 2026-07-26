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
  "src/components/database/configs/permissions.config.tsx",
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
  "src/components/import/stats-cards.tsx",
  "src/components/import/student-row-card.tsx",
  "src/components/import/upload-section.tsx",
  "src/components/operator/announcement-views-accordion.tsx",
  "src/components/operator/status-dropdown.tsx",
  "src/components/permissions/permission-selector.tsx",
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
  "src/components/students/personal-info-form-modal.tsx",
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
  "src/components/ui/page-header/DesktopFilters.tsx",
  "src/components/ui/page-header/FilterButton.tsx",
  "src/components/ui/page-header/FilterPanel.tsx",
  "src/components/ui/page-header/OverflowMenu.tsx",
  "src/components/ui/page-header/PageHeader.tsx",
  "src/components/ui/page-header/PageHeaderWithSearch.tsx",
  "src/components/ui/page-header/StatusIndicator.tsx",
  "src/components/ui/password-change-modal.tsx",
  "src/components/ui/textarea.tsx",
  "src/lib/activity-helpers.ts",
  "src/lib/dashboard-helpers.ts",
  "src/lib/database/themes.tsx",
  "src/lib/iot-helpers.ts",
  "src/lib/operator/suggestions-helpers.ts",
  "src/lib/staff-helpers.ts",
  "src/lib/suggestions-helpers.ts",
]);

const GENERIC_BRAND_COLOR_BASELINE = parseMatchBaseline(`
src/app/[tenant]/(protected)/active-supervisions/page.tsx|bg-red-100:2 bg-red-50:2 border-red-200:2 from-blue-50/80 text-red-600:2 to-cyan-100/80
src/app/[tenant]/(protected)/activities/page.tsx|bg-red-50 border-red-200 text-red-800
src/app/[tenant]/(protected)/dashboard/page.tsx|bg-red-50 border-red-200 from-amber-50/80 from-blue-50/80 from-emerald-50/80 from-emerald-500 from-green-50/80 from-indigo-50/80 from-indigo-500 from-orange-50/80 from-orange-500 from-purple-50/80:2 from-purple-500 from-red-50/80 from-yellow-400 from-yellow-50/80 ring-amber-200/60 ring-blue-200/60 ring-emerald-200/60 ring-green-200/60 ring-indigo-200/60 ring-indigo-500:2 ring-orange-200/60 ring-purple-200/60:2 ring-red-200/60 ring-yellow-200/60 text-red-800 to-blue-100/80 to-cyan-100/80 to-green-100/80 to-green-600 to-indigo-600 to-lime-100/80 to-orange-100/80 to-orange-600 to-purple-600 to-rose-100/80 to-violet-100/80:2 to-yellow-100/80:2 to-yellow-500
src/app/[tenant]/(protected)/database/activities/page.tsx|bg-red-50 bg-red-600 bg-red-700 border-red-200 text-red-800
src/app/[tenant]/(protected)/database/devices/page.tsx|bg-red-50 bg-red-600 bg-red-700 border-red-200 text-red-800
src/app/[tenant]/(protected)/database/groups/page.tsx|bg-red-50 bg-red-600 bg-red-700 border-red-200 text-red-800
src/app/[tenant]/(protected)/database/permissions/page.tsx|bg-red-50 bg-red-600 bg-red-700 border-red-200 text-red-800
src/app/[tenant]/(protected)/database/personal/import/page.tsx|bg-amber-100 bg-blue-100 bg-blue-50/30 bg-green-100 bg-red-100 border-blue-100 text-amber-700 text-blue-600:2 text-blue-700 text-green-700 text-red-600:2 text-red-700 text-red-800
src/app/[tenant]/(protected)/database/personal/page.tsx|bg-red-50 bg-red-600 bg-red-700 border-red-200 text-red-800
src/app/[tenant]/(protected)/database/roles/page.tsx|bg-amber-50 bg-red-50 bg-red-600 bg-red-700 border-amber-200 border-red-200 text-amber-600 text-amber-700 text-amber-800 text-red-800
src/app/[tenant]/(protected)/database/rooms/page.tsx|bg-red-50 bg-red-600 bg-red-700 border-red-200 text-red-800
src/app/[tenant]/(protected)/database/students/import/page.tsx|bg-blue-50/30 border-blue-100 text-blue-600:2 text-red-600 text-red-800
src/app/[tenant]/(protected)/database/students/page.tsx|bg-red-100 bg-red-50:2 bg-red-600 bg-red-700 border-red-200:2 text-red-700 text-red-800
src/app/[tenant]/(protected)/ogs-groups/page.tsx|bg-orange-50:2 border-orange-200:2 from-amber-50/80 from-blue-50/80 from-emerald-50/80 from-fuchsia-50/80 from-red-50/80 text-orange-600:2 text-orange-900 to-cyan-100/80 to-green-100/80 to-pink-100/80 to-rose-100/80 to-yellow-100/80
src/app/[tenant]/(protected)/rooms/page.tsx|text-blue-400
src/app/[tenant]/(protected)/staff/[id]/page.tsx|bg-red-500
src/app/[tenant]/(protected)/staff/page.tsx|bg-red-50 bg-red-500 border-red-200 text-red-800
src/app/[tenant]/(protected)/students/[id]/feedback-history/page.tsx|bg-blue-100 bg-blue-200 bg-green-500:3 bg-red-500:3 bg-yellow-400:3 text-blue-800
src/app/[tenant]/(protected)/students/[id]/page.tsx|bg-blue-100 bg-blue-200 text-amber-600 text-blue-800
src/app/[tenant]/(protected)/students/[id]/room-history/page.tsx|bg-amber-50:3 bg-blue-50/50:2 border-amber-200 text-amber-700:3 text-amber-800:3 text-amber-900
src/app/[tenant]/(protected)/students/search/page.tsx|text-red-400
src/app/[tenant]/(protected)/substitutions/page.tsx|bg-orange-100 bg-orange-50 bg-orange-50/70 bg-orange-500:5 bg-purple-100:2 bg-purple-50 bg-purple-50/70 bg-purple-500:5 border-orange-200 border-orange-300 border-purple-200 border-purple-300 text-orange-500 text-orange-700 text-purple-500 text-purple-600 text-purple-700:2
src/app/[tenant]/(protected)/suggestions/page.tsx|bg-red-500 bg-red-600
src/app/[tenant]/(protected)/time-tracking/page.tsx|bg-amber-100:3 bg-amber-50:2 bg-amber-500 bg-red-50 border-amber-200 border-amber-400:2 border-red-400:2 text-amber-300 text-amber-500:6 text-amber-600:6 text-amber-700:2 text-red-500:4 text-red-600 text-red-700
src/app/operator/announcements/page.tsx|bg-amber-100 bg-red-500 bg-red-600 border-blue-500:3 ring-blue-500:3 text-amber-700
src/app/operator/devices/page.tsx|bg-red-50 bg-red-600:2 bg-red-700:2 text-red-600:2
src/app/operator/email-confirm/email-confirm-content.tsx|bg-blue-100 bg-green-100 bg-red-100 text-blue-600 text-green-600 text-red-600
src/app/operator/invite/invite-content.tsx|bg-red-50:2 border-red-200 text-red-500 text-red-600 text-red-700
src/app/operator/operators/page.tsx|bg-green-100 bg-red-100 bg-red-200 bg-red-50 bg-red-500 bg-red-600 bg-violet-100 bg-violet-200 border-red-200 border-violet-500:2 ring-violet-500:2 text-green-700 text-red-600:2 text-red-700:2 text-violet-700
src/app/operator/persons/page.tsx|bg-amber-50 bg-blue-50 bg-green-50 bg-purple-50 bg-red-50:2 bg-red-600 bg-red-700 border-red-500 ring-red-500 text-amber-800 text-blue-700 text-green-700 text-purple-700 text-red-600:2
src/app/operator/provisioning/create-account-modal.tsx|bg-green-50 border-blue-500 ring-blue-500:2 text-blue-600 text-green-600 text-green-800
src/app/operator/provisioning/create-device-modal.tsx|bg-green-50 border-blue-500:3 border-green-200 ring-blue-500:3 text-blue-600:2 text-green-700 text-green-800
src/app/operator/provisioning/create-organization-modal.tsx|border-blue-500:2 ring-blue-500:2
src/app/operator/provisioning/create-school-modal.tsx|border-blue-500:8 ring-blue-500:8
src/app/operator/provisioning/edit-organization-modal.tsx|border-blue-500:2 ring-blue-500:2
src/app/operator/provisioning/edit-school-modal.tsx|border-blue-500:8 ring-blue-500:8
src/app/operator/provisioning/invite-admin-modal.tsx|bg-green-50 border-blue-500 ring-blue-500:2 text-blue-600 text-green-600 text-green-800 text-red-600
src/app/operator/provisioning/provisioning-shared.tsx|bg-amber-50 bg-blue-600 bg-green-100:2 bg-red-100 bg-red-50 bg-yellow-100 ring-blue-500 text-amber-700 text-green-700:2 text-red-500 text-red-600 text-red-700 text-yellow-700
src/app/operator/provisioning/set-api-key-modal.tsx|bg-amber-50 bg-green-50 bg-red-600 bg-red-700 border-amber-200 border-blue-500 border-green-200 ring-blue-500 text-amber-800 text-blue-600:2 text-green-800
src/app/operator/provisioning/soft-delete-shared.tsx|bg-amber-50 bg-green-100 bg-green-200 bg-red-100 bg-red-50:4 bg-red-50/50 bg-red-600 bg-red-700 border-red-100/50 border-red-500 ring-red-500 text-amber-800 text-green-700 text-red-600:4 text-red-700
src/app/operator/schools/page.tsx|bg-red-100 bg-red-200 text-red-700
src/app/operator/settings/page.tsx|bg-red-50 text-red-700
src/app/operator/suggestions/[id]/page.tsx|bg-blue-200 bg-blue-50 bg-green-200 bg-green-50 bg-red-50 bg-red-500:2 bg-red-600:2 bg-yellow-50 border-blue-200 border-green-200 border-yellow-200 text-blue-600 text-blue-800 text-green-800 text-red-500:3 text-yellow-800
src/app/operator/suggestions/page.tsx|bg-blue-100 bg-blue-500 bg-yellow-100 text-blue-700 text-red-500 text-yellow-700
src/app/page.tsx|bg-red-50 border-red-200 text-red-700
src/components/active-supervisions/states.tsx|bg-red-50/50 bg-red-600 bg-red-700 border-red-100 text-red-500
src/components/active/unclaimed-rooms.tsx|bg-amber-100 bg-amber-50/80 bg-red-50 border-amber-200 border-red-200 text-amber-600 text-red-700
src/components/activities/activity-management-modal.tsx|border-red-200/30 from-red-50/60 text-red-600:2 text-red-700 to-rose-50/60
src/components/activities/quick-create-modal.tsx|from-blue-100/10 ring-red-400 text-red-600:3 to-indigo-100/10
src/components/admin/guardian-approval-queue.tsx|bg-red-100 bg-red-200 bg-red-50 bg-red-600 bg-red-700 border-red-200 text-red-700:2
src/components/admin/invitation-form.tsx|bg-red-50/50 border-red-200/50 ring-red-400 text-red-600:2 text-red-700
src/components/admin/pending-invitations-list.tsx|bg-red-100:2 bg-red-200 bg-red-50:2 bg-red-50/50 bg-red-600 bg-red-700 border-red-200/50 text-red-600 text-red-700:4
src/components/auth/invitation-accept-form.tsx|bg-red-50/50 border-red-200/50 ring-red-300:4 text-red-600:5 text-red-700
src/components/auth/invitation-page-content.tsx|bg-red-50 border-red-200 text-red-600 text-red-700
src/components/auth/mfa-admin-override-modal.tsx|bg-amber-50 bg-red-50:3 border-amber-200 text-amber-900 text-red-700:3
src/components/auth/reset-password-page-content.tsx|bg-red-50 border-red-200 text-red-600 text-red-700
src/components/auth/role-permission-management-modal.tsx|bg-purple-50:2 bg-purple-50/30 bg-purple-600:2 bg-purple-700 border-purple-400:2 ring-purple-500:2 text-purple-600:2
src/components/dashboard/header/profile-dropdown.tsx|bg-red-50 bg-red-600 text-red-600 text-red-700
src/components/dashboard/header/session-warning.tsx|bg-red-50 border-red-200 text-red-600:2 text-red-800
src/components/dashboard/sidebar.tsx|text-amber-500 text-indigo-500 text-pink-500 text-sky-500 text-teal-500:2 text-violet-500:2
src/components/database/configs/activities.config.tsx|bg-blue-100:2 bg-purple-100 bg-purple-50/30 bg-red-100:2 bg-red-50/30 text-blue-700:2 text-purple-700 text-purple-800 text-red-700:2 text-red-800
src/components/database/configs/devices.config.tsx|bg-blue-100 bg-blue-50 bg-blue-600 bg-blue-700 bg-green-500 bg-yellow-50 bg-yellow-50/30 bg-yellow-600 bg-yellow-700 border-blue-200 border-yellow-200 text-blue-600 text-blue-800:2 text-yellow-700:2 text-yellow-800
src/components/database/configs/groups.config.tsx|bg-blue-100 bg-blue-400/80 bg-green-100 bg-green-400/80 bg-green-50/30 bg-indigo-400/80 text-blue-700 text-green-700 text-green-800
src/components/database/configs/permissions.config.tsx|bg-indigo-400/80 bg-pink-50/30 text-indigo-800
src/components/database/configs/roles.config.tsx|bg-blue-400/80 bg-purple-50/30
src/components/database/configs/rooms.config.tsx|bg-green-100 bg-indigo-100 bg-indigo-50/30 bg-red-100 text-green-800:2 text-indigo-800 text-red-800
src/components/database/configs/students.config.tsx|bg-blue-100 bg-blue-50 bg-fuchsia-100 bg-fuchsia-400/80 bg-fuchsia-500 bg-green-100 bg-green-400/80 bg-green-50 bg-green-500 bg-orange-100:2 bg-orange-400/80 bg-orange-500 bg-purple-100 bg-purple-50 bg-red-400/80 bg-yellow-100 bg-yellow-400/80 bg-yellow-50 bg-yellow-500 text-blue-700 text-blue-800 text-fuchsia-800 text-green-800:2 text-orange-700 text-orange-800 text-purple-700 text-purple-800 text-yellow-800:2
src/components/database/configs/teachers.config.tsx|bg-blue-50 bg-green-50 bg-indigo-50 bg-purple-100 text-blue-800 text-indigo-800 text-purple-800:2
src/components/database/detail-delete-button.tsx|bg-red-100 bg-red-50 border-red-200 text-red-700
src/components/devices/devices-master-detail.tsx|bg-yellow-50 border-yellow-200 text-yellow-800
src/components/groups/group-transfer-modal.tsx|bg-blue-50/50 bg-orange-50 bg-red-100 bg-red-50:2 border-blue-100 border-orange-200 border-red-200:2 text-blue-600 text-orange-600 text-orange-900 text-red-600 text-red-700 text-red-800
src/components/guardians/guardian-form-modal.tsx|bg-blue-50/30:4 bg-red-50:4 border-red-200 border-red-400:4 text-blue-600:4 text-red-500:2 text-red-600:6 text-red-800 text-yellow-400 text-yellow-500
src/components/guardians/guardian-list.tsx|bg-blue-100 bg-green-100 bg-purple-100:2 bg-red-100 decoration-blue-600:2 text-blue-600:2 text-blue-800 text-green-800 text-purple-700 text-purple-800 text-red-800
src/components/guardians/guardian-picker-panel.tsx|bg-blue-50/40:2 bg-red-50 border-red-200 text-blue-600:2 text-red-800
src/components/guardians/guardian-relationship-fields.tsx|bg-blue-50/30 bg-red-50 border-red-200 ring-green-600 ring-purple-600 ring-red-600 text-blue-600 text-green-600 text-purple-600 text-red-600:2 text-red-900
src/components/guardians/student-guardian-manager.tsx|bg-purple-100 bg-red-50 border-red-200 text-purple-600 text-red-700
src/components/import/stats-cards.tsx|from-blue-50/80 from-green-50/80 from-red-50/80 to-cyan-100/80 to-lime-100/80 to-rose-100/80
src/components/import/student-row-card.tsx|bg-amber-100 bg-blue-100 bg-green-100 bg-red-100 text-amber-700 text-blue-700 text-green-700 text-red-600 text-red-700
src/components/import/upload-section.tsx|bg-green-50 border-green-500 ring-green-500 text-green-500 text-green-600:2
src/components/operator/announcement-views-accordion.tsx|bg-green-100 text-green-500 text-green-600 text-red-500
src/components/operator/status-dropdown.tsx|ring-blue-500
src/components/permissions/permission-selector.tsx|text-pink-600
src/components/roles/roles-master-detail.tsx|bg-amber-50 bg-purple-100 bg-purple-50:2 border-purple-200 text-amber-700 text-purple-700:2
src/components/settings/passkey-settings-section.tsx|bg-red-50 text-red-700
src/components/settings/personalization-tab.tsx|bg-green-50 bg-red-50 border-green-500 border-red-200 ring-green-500 text-green-500 text-red-600
src/components/settings/settings-field.tsx|bg-amber-50 bg-yellow-50 border-amber-200 text-amber-700 text-amber-800 text-red-600:4 text-yellow-700
src/components/settings/settings-page.tsx|text-red-600:2 text-red-800
src/components/shared/base-comment-accordion.tsx|bg-blue-100 bg-red-500:2 bg-red-600 text-blue-600:2 text-red-500:2
src/components/sse/SSEErrorBoundary.tsx|bg-red-50 border-red-200 text-red-700
src/components/staff/abwesenheiten-tab.tsx|bg-red-50 bg-red-500 text-amber-600 text-red-600
src/components/staff/admin-session-edit-modal.tsx|bg-amber-50 bg-red-50 text-amber-800 text-red-700
src/components/staff/arbeitszeitmodell-tab.tsx|bg-amber-50/60 border-amber-200
src/components/staff/school-overview-section.tsx|bg-amber-500 text-amber-600 text-red-600:2
src/components/staff/shift-edit-modal.tsx|bg-amber-50:2 bg-red-50 text-amber-800:2 text-red-700
src/components/staff/staff-pending-inbox.tsx|bg-red-500
src/components/staff/staff-session-table.tsx|bg-amber-100 bg-amber-50 bg-amber-50/40 bg-red-50 text-amber-600 text-amber-700 text-amber-800 text-green-600 text-red-600 text-red-700
src/components/staff/staff-time-accounts-table.tsx|bg-red-50 border-red-200 text-red-600 text-red-800
src/components/staff/staff-time-views.tsx|bg-amber-500 bg-red-500 text-amber-600 text-red-600
src/components/staff/stundenkonto-panel.tsx|bg-red-50 text-red-600
src/components/students/arrival-day-edit-modal.tsx|bg-red-50:3 border-red-200 text-red-600:2 text-red-700
src/components/students/arrival-schedule-form-modal.tsx|bg-red-50 border-red-200 text-red-700
src/components/students/arrival-schedule-manager.tsx|bg-orange-100:2 bg-red-50 border-red-200 text-orange-600:2 text-red-700
src/components/students/class-bulk-arrival-modal.tsx|bg-red-50 border-red-300 text-red-600
src/components/students/personal-info-form-modal.tsx|ring-blue-500:3
src/components/students/pickup-day-edit-modal.tsx|bg-red-50:3 border-blue-500:3 border-red-200 ring-blue-500:3 text-red-500 text-red-600 text-red-700
src/components/students/pickup-schedule-form-modal.tsx|bg-red-50 border-blue-500:2 border-red-200 ring-blue-500:2 text-red-700
src/components/students/pickup-schedule-manager.tsx|bg-orange-100:2 bg-red-50 border-red-200 text-orange-600:2 text-red-700
src/components/students/privacy-consent-section.tsx|bg-yellow-100 text-green-600 text-red-600 text-yellow-800
src/components/students/student-card.tsx|text-orange-500
src/components/students/student-checkout-section.tsx|border-amber-500 text-amber-500
src/components/students/student-create-modal.tsx|bg-blue-50/30:2 bg-red-50:3 border-red-200 text-blue-600:2 text-red-600:2 text-red-800
src/components/students/student-detail-components.tsx|bg-orange-400
src/components/students/student-form-fields.tsx|bg-blue-50/30:6 bg-red-50:2 border-red-300:2 text-blue-600 text-red-500 text-red-600:2
src/components/students/student-historie-tab.tsx|bg-amber-50 bg-red-50 border-amber-200 border-red-200 text-amber-800 text-red-800
src/components/students/student-stammdaten-tab.tsx|bg-red-50:2 border-red-200:2 text-red-800:2
src/components/suggestions/suggestion-card.tsx|bg-blue-100 border-blue-200/50 text-blue-700
src/components/suggestions/suggestion-form.tsx|text-red-500:2 text-red-600
src/components/suggestions/vote-buttons.tsx|text-red-500:2 text-red-600
src/components/teachers/caregiver-blocker-resolution-modal.tsx|bg-orange-100 bg-red-50:4 border-red-200:4 text-orange-700 text-red-700:4
src/components/teachers/caregiver-capability-modal.tsx|bg-amber-100 bg-amber-200 bg-red-50 border-amber-300 border-red-200 text-amber-500 text-amber-800 text-amber-900 text-red-700
src/components/teachers/teacher-form.tsx|bg-orange-50/30:2 bg-red-50:6 border-red-200 border-red-300:5 text-orange-600:2 text-red-500:6 text-red-600:6 text-red-800
src/components/time-tracking/edit-history-accordion.tsx|text-red-400:2
src/components/time-tracking/leave-requests-card.tsx|bg-red-600 bg-red-700 text-amber-600 text-red-600 text-red-700
src/components/time-tracking/monatskarte.tsx|bg-amber-50 border-amber-200 text-amber-900 text-red-600:2
src/components/time-tracking/vacation-request-modal.tsx|bg-amber-200 bg-amber-50:2 bg-red-50 border-amber-200 border-red-200 ring-amber-200 text-amber-700 text-amber-800 text-red-600 text-red-700
src/components/ui/alert.tsx|bg-blue-50 bg-red-50 bg-yellow-50 border-blue-100 border-red-100 border-yellow-100 text-blue-700 text-red-700 text-yellow-700
src/components/ui/button.tsx|bg-green-600 bg-green-700 bg-red-100 bg-red-50 bg-red-600 bg-red-700 ring-red-300 ring-red-400 text-red-600
src/components/ui/database/accents.ts|border-amber-300 border-amber-500 border-blue-300 border-blue-500 border-green-300 border-green-500 border-indigo-300 border-indigo-500 border-orange-300 border-orange-500 border-pink-300 border-pink-500 border-purple-300 border-purple-500 border-red-300 border-red-500 border-yellow-300 border-yellow-500 from-amber-500 from-amber-600 from-blue-500 from-blue-600 from-green-500 from-green-600 from-indigo-500 from-indigo-600 from-orange-500 from-orange-600 from-pink-500 from-pink-600 from-purple-500 from-purple-600 from-red-500 from-red-600 from-yellow-500 from-yellow-600 ring-amber-500 ring-blue-500 ring-green-500 ring-indigo-500 ring-orange-500 ring-pink-500 ring-purple-500 ring-red-500 ring-yellow-500 text-amber-500 text-amber-600:2 text-blue-500 text-blue-600:2 text-green-500 text-green-600:2 text-indigo-500 text-indigo-600:2 text-orange-500 text-orange-600:2 text-pink-500 text-pink-600:2 text-purple-500 text-purple-600:2 text-red-500 text-red-600:2 text-yellow-500 text-yellow-600:2 to-amber-600 to-amber-700 to-blue-600 to-blue-700 to-green-600 to-green-700 to-indigo-600 to-indigo-700 to-orange-600 to-orange-700 to-purple-600 to-purple-700 to-red-600 to-red-700 to-rose-600 to-rose-700 to-yellow-600 to-yellow-700
src/components/ui/database/database-form.tsx|bg-blue-100 bg-blue-200 bg-blue-300 border-red-400 text-blue-600 text-blue-700 text-blue-800 text-red-600:2
src/components/ui/database/database-select.tsx|text-red-600
src/components/ui/date-picker.tsx|text-blue-600:2
src/components/ui/detail-modal-components.tsx|bg-amber-50/30 bg-blue-50/30 bg-green-50/30 bg-indigo-50/30 bg-orange-50/30 bg-purple-50/30 bg-red-50/30 text-amber-600 text-blue-600 text-green-600 text-indigo-600 text-orange-600 text-purple-600 text-red-600
src/components/ui/forbidden-page.tsx|bg-red-50/50 border-red-200/50 text-red-600 text-red-700 text-red-900
src/components/ui/input.tsx|text-red-600
src/components/ui/page-header/ActiveFilterChips.tsx|bg-blue-100 text-blue-600 text-blue-700:2 text-blue-900
src/components/ui/page-header/DesktopFilters.tsx|ring-blue-500
src/components/ui/page-header/FilterButton.tsx|bg-blue-500:2 border-blue-500 ring-blue-500
src/components/ui/page-header/FilterPanel.tsx|bg-blue-50 ring-blue-200 text-blue-500 text-blue-700
src/components/ui/page-header/OverflowMenu.tsx|ring-blue-500/50 text-red-600
src/components/ui/page-header/PageHeader.tsx|bg-green-500 bg-red-500 bg-yellow-500
src/components/ui/page-header/PageHeaderWithSearch.tsx|bg-blue-50:2 ring-blue-200:2 text-blue-700:2
src/components/ui/page-header/StatusIndicator.tsx|bg-green-500 bg-red-500 bg-yellow-500
src/components/ui/password-change-modal.tsx|bg-blue-50 border-blue-200 border-red-400:3 text-red-600:3
src/components/ui/textarea.tsx|text-red-600
src/lib/activity-helpers.ts|from-blue-500 from-green-500 from-green-600 from-orange-500 from-pink-500 from-purple-500 from-red-500 from-yellow-500 to-amber-600 to-emerald-600 to-indigo-600 to-orange-600 to-pink-600:2 to-rose-600 to-teal-600
src/lib/dashboard-helpers.ts|bg-amber-500:2 bg-blue-500 bg-green-500:2 bg-orange-500
src/lib/database/themes.tsx|bg-amber-50 bg-blue-50 bg-green-50 bg-indigo-50 bg-orange-50 bg-purple-50 border-amber-200 border-blue-200 border-green-200 border-indigo-200 border-orange-200 border-purple-200 from-amber-400 from-amber-500 from-green-400 from-green-500 from-indigo-500:2 from-orange-500 from-pink-500 from-purple-400:2 from-purple-500 from-teal-400 from-teal-500 text-amber-800 text-blue-800 text-green-800 text-indigo-800 text-orange-800 text-purple-800 text-red-800 to-blue-500 to-blue-600 to-emerald-500 to-emerald-600 to-indigo-500:2 to-indigo-600:2 to-orange-600 to-purple-600 to-red-600 to-rose-600 to-yellow-500
src/lib/iot-helpers.ts|bg-green-100:2 bg-red-100:2 bg-yellow-100 text-green-800:2 text-red-800:2 text-yellow-800
src/lib/operator/suggestions-helpers.ts|bg-purple-500 border-purple-500
src/lib/staff-helpers.ts|from-green-50/80 from-red-50/80 from-sky-50/80 to-emerald-100/80 to-rose-100/80 to-sky-100/80
src/lib/suggestions-helpers.ts|bg-blue-100 bg-green-100 bg-purple-100 bg-red-100 bg-yellow-100 text-blue-700 text-green-700 text-purple-700 text-red-700 text-yellow-800
`);

const OVERLAY_BASELINE_FILES = new Set([
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

const ROUNDED_3XL_BASELINE_FILES = new Set([
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

const OVERLAY_BASELINE = new Map([
  ["src/app/[tenant]/(protected)/time-tracking/page.tsx", 1],
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
  ["src/app/[tenant]/(protected)/dashboard/page-skeleton.tsx", 2],
  ["src/app/[tenant]/(protected)/dashboard/page.tsx", 10],
  ["src/app/[tenant]/(protected)/rooms/page-skeleton.tsx", 2],
  ["src/app/[tenant]/(protected)/staff/[id]/page.tsx", 1],
  ["src/app/[tenant]/(protected)/students/[id]/change-history/page.tsx", 1],
  [
    "src/app/[tenant]/(protected)/students/[id]/feedback-history/page-skeleton.tsx",
    1,
  ],
  ["src/app/[tenant]/(protected)/students/[id]/feedback-history/page.tsx", 1],
  [
    "src/app/[tenant]/(protected)/students/[id]/room-history/page-skeleton.tsx",
    2,
  ],
  ["src/app/[tenant]/(protected)/students/[id]/room-history/page.tsx", 3],
  ["src/app/[tenant]/(protected)/suggestions/page-skeleton.tsx", 1],
  ["src/app/[tenant]/(protected)/time-tracking/page.tsx", 3],
  ["src/app/[tenant]/(public)/enroll/[phaseId]/page.tsx", 4],
  ["src/app/[tenant]/(public)/enroll/page.tsx", 2],
  ["src/app/[tenant]/(public)/enroll/preview/page.tsx", 4],
  ["src/app/[tenant]/(public)/enroll/submitted/page.tsx", 1],
  ["src/app/help/nfc/erste-schritte/page.tsx", 3],
  ["src/app/operator/announcements/page.tsx", 2],
  ["src/app/operator/provisioning/provisioning-shared.tsx", 1],
  ["src/app/operator/provisioning/soft-delete-shared.tsx", 1],
  ["src/app/operator/suggestions/[id]/page.tsx", 3],
  ["src/app/operator/suggestions/page.tsx", 2],
  ["src/components/enrollment/enrollment-status-view.tsx", 1],
  ["src/components/parent/child-detail.tsx", 2],
  ["src/components/rooms/room-detail-content.tsx", 1],
  ["src/components/rooms/students-in-room-section.tsx", 1],
  ["src/components/rooms/transit-students-section.tsx", 1],
  ["src/components/staff/abwesenheiten-tab.tsx", 3],
  ["src/components/staff/arbeitszeitmodell-tab.tsx", 2],
  ["src/components/staff/staff-time-views.tsx", 1],
  ["src/components/staff/stundenkonto-panel.tsx", 1],
  ["src/components/staff/uebersicht-tab-skeleton.tsx", 3],
  ["src/components/staff/uebersicht-tab.tsx", 3],
  ["src/components/staff/zeiterfassung-tab.tsx", 1],
  ["src/components/students/student-checkout-section.tsx", 5],
  ["src/components/suggestions/suggestion-card.tsx", 1],
  ["src/components/time-tracking/leave-requests-card.tsx", 1],
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

export default {
  meta: { name: "ui-kit" },
  rules: {
    "no-generic-brand-colors": noGenericBrandColors,
    "no-hand-rolled-overlay": noHandRolledOverlay,
    "no-rounded-3xl": noRounded3xl,
  },
};
