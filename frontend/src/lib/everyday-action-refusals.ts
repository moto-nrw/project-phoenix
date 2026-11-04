import type { ErrorCode } from "~/lib/error-codes.generated";

/**
 * Refusal codes of the everyday actions (#5012): stamping, the start page
 * layout, the info displays and the parents' meal registration.
 *
 * A code is listed as a refusal only if the backend uses it when the
 * caller may no longer read the cached resource. The optimistic helper then
 * drops every representation of it (status and code, ADR 0006). Session
 * codes (`identity.session_*`, handled by the session guards) and failures
 * without a response are never refusals.
 *
 * `general.permission` is the class code of a 401/403 without a code of its
 * own (`ErrorClassCode`, `RequiresPermission`); `general.input` with status
 * 404 is the class code of a not-found answer.
 *
 * Write-only denials have their own codes and only roll back the action.
 * They do not discard readable data or trigger another read (#5423).
 */

/**
 * POST /api/time-tracking/check-in and /check-out
 * (`modules/workforce/http/timetracking`): 403 from
 * `RequiresPermission(time_tracking:own)`, 401 when the account has no staff
 * record (`getStaffIDFromClaims`). GET /current and every own list need the
 * same permission and staff record. Business states carry their own codes
 * (`workforce.already_checked_in`, `workforce.no_active_session`, the
 * deviation and planned-start codes).
 */
export const STAMP_REFUSALS: readonly ErrorCode[] = ["general.permission"];

/**
 * PUT/DELETE /api/settings/home-layout and PUT …/policies
 * (`modules/settings/http/home_layout_api.go`): none. The read is open to
 * every signed-in account; 401 without account context is a session
 * failure. The 403 `general.permission` of the policy write (no
 * `config:update`) is a business state: the layout stays readable.
 */
export const HOME_LAYOUT_REFUSALS: readonly ErrorCode[] = [];

/**
 * Display writes need display:manage. Without display:read either, the list
 * is refused with general.permission. Read-only access instead answers with
 * display.manage_forbidden and keeps the confirmed list. A 404 general.input
 * names one deleted display, not the list; display.disabled is a state.
 */
export const DISPLAY_REFUSALS: readonly ErrorCode[] = ["general.permission"];

/**
 * Meal participation reads and writes both require parent_portal.access on
 * the child's relationship. Read denial is care.guardian_permission_denied
 * or care.child_not_linked when the link projection filters inaccessible links.
 * Missing only
 * meal_participation.manage answers with meals.participation_manage_denied:
 * the action rolls back, but the plan stays readable. Ended care and disabled
 * registration or meal plans also remain business states.
 */
export const MEAL_PARTICIPATION_REFUSALS: readonly ErrorCode[] = [
  "general.permission",
  "care.child_not_linked",
  "care.guardian_permission_denied",
];
