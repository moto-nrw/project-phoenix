import { MODULE_SETTING_KEYS } from "./modules";

// Keys whose value rides on /auth/tenant/resolve. The settings PUT/DELETE
// proxies (tenant-side `/api/settings/values/[key]` and operator-side
// `/api/operator/provisioning/schools/[id]/settings/values/[key]`) consult
// this set after a successful write and purge the cached layout lookup via
// `revalidateTag('tenant-${slug}')` + `revalidatePath('/${slug}', 'layout')`.
// The settings page also calls `router.refresh()` so the RSC tree pulls the
// fresh layout data.
export const TENANT_RESOLVE_AFFECTING_KEYS: ReadonlySet<string> = new Set([
  // The meal switches ride on /auth/tenant/resolve since #3873, so a kitchen
  // role without config:read sees the meal pages. Without revalidation,
  // switching them leaves the navigation stale for up to 300s.
  "operations.meal_plan_enabled",
  "operations.meal_registration_enabled",
  // Expire the analytics policy immediately so reloads and new tabs cannot
  // start capturing with a stale enabled value after the school opts out.
  "analytics.enabled",
  "calendar.caldav_enabled",
  "operations.student_photos_enabled",
  // presence_mode is served via /auth/tenant/resolve and drives the
  // binary-vs-detailed kiosk + UI branch. Without revalidation, an
  // operator flipping it leaves reloads/new tabs on the old mode for
  // up to the layout cache TTL (300s).
  "operations.presence_mode",
  // The module switches (#4892) ride on /auth/tenant/resolve as
  // booked_modules, so every staff member's navigation follows a booking
  // without config:read. The moto team switches them in the operator portal.
  ...MODULE_SETTING_KEYS,
  // parent_notes_enabled rides on /auth/tenant/resolve too (as
  // parent_messaging_enabled) so non-admin staff can hide the "Neue Nachricht"
  // compose entry points without config:read. Without revalidation, toggling it
  // leaves reloads/new tabs on the stale messagingEnabled value for up to the
  // layout cache TTL (300s).
  "operations.parent_notes_enabled",
  // staff_messaging_enabled rides on /auth/tenant/resolve too (as
  // staff_messaging_enabled) so every staff user can hide the Team-Chat entry
  // without config:read. Without revalidation, switching it leaves reloads and
  // new tabs on the stale visibility for up to the layout cache TTL (300s) -
  // and because the chat fails CLOSED, that stale value shows "ausgeschaltet"
  // on a school that just turned it on.
  "operations.staff_messaging_enabled",
  // child_search_only_navigation (#1321) decides every staff member's
  // navigation. A stale value would keep „Meine Gruppen“ and „Aktuelle
  // Aufsicht“ in the sidebar for five minutes after the school switched them
  // off, or hide them after it switched them back on.
  "operations.child_search_only_navigation",
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
  // The navigation and the planning pages read timetable.enabled from
  // /auth/tenant/resolve since #4996 instead of the settings schema. Without
  // revalidation a switch would reach reloads and new tabs only after the
  // layout cache TTL (300s). The signed-in switches of #4996 ride on
  // /api/me/navigation instead; useSettingsCacheBridge refreshes that.
  "timetable.enabled",
]);
