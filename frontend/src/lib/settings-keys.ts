// Keys whose value rides on /auth/tenant/resolve. The settings PUT/DELETE
// proxies (tenant-side `/api/settings/values/[key]` and operator-side
// `/api/operator/provisioning/schools/[id]/settings/values/[key]`) consult
// this set after a successful write and purge the cached layout fetch via
// `revalidateTag('tenant-${slug}')` + `revalidatePath('/${slug}', 'layout')`.
// The settings page also calls `router.refresh()` so the RSC tree pulls the
// fresh layout data.
export const TENANT_RESOLVE_AFFECTING_KEYS: ReadonlySet<string> = new Set([
  "operations.student_photos_enabled",
  // presence_mode is served via /auth/tenant/resolve and drives the
  // binary-vs-detailed kiosk + UI branch. Without revalidation, an
  // operator flipping it leaves reloads/new tabs on the old mode for
  // up to the layout cache TTL (300s).
  "operations.presence_mode",
  // nfc_enabled is also served through /auth/tenant/resolve so every staff
  // user can hide NFC-only navigation without config:read.
  "attendance.nfc_enabled",
  // parent_notes_enabled rides on /auth/tenant/resolve too (as
  // parent_messaging_enabled) so non-admin staff can hide the "Neue Nachricht"
  // compose entry points without config:read. Without revalidation, toggling it
  // leaves reloads/new tabs on the stale messagingEnabled value for up to the
  // layout cache TTL (300s).
  "operations.parent_notes_enabled",
  // Approved-child offering corrections consume this through tenant shell
  // metadata so staff without config:read never see a save action that the
  // authoritative enrollment service will reject.
  "enrollment.care_offerings_enabled",
  "attendance.web_enabled",
  "operations.group_mode",
  // The school-wide operational overview scope (#2380) travels in tenant
  // shell metadata: it decides which supervision endpoint the client asks
  // for, so a stale value costs every caregiver a guaranteed 403 per refresh.
  "operations.operational_overview_scope",
  "timetable.show_expected_children_count",
  "enrollment.waitlist_enabled",
  // grade_level_max is exposed by tenant resolve and drives every enrollment
  // form's client-side upper bound. Purge the layout cache immediately so a
  // saved setting cannot leave new forms enforcing the old cap for five
  // minutes.
  "enrollment.grade_level_max",
  // The Notfall page describes what the printed list contains, including the
  // health column. Without revalidation, an admin switching the column off
  // leaves the page promising health data the PDF no longer carries for up to
  // the layout cache TTL (300s).
  "operations.emergency_list_health_info",
]);
