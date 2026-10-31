import { ApiError } from "~/lib/api-error";
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
 * Some write routes answer a business state with the same code as a
 * refusal: the caller may still read, but not write. Those codes are not
 * refusals. They are listed under `RECHECK`: the action rolls back and the
 * view loads the resource again, so a read that is refused too shows as
 * the reader's load error (#5423).
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
 * PATCH/DELETE /api/displays/{id} (`api/display/api.go`, `renderError`;
 * `api/display_http.go`, `renderDisplayFailure`): none for the list. The
 * list is readable with `display:read` or `display:manage`; a write needs
 * `display:manage`. Its 403 `general.permission` therefore also comes for an
 * account that may still read the list, and 404 `general.input` names one
 * display, not the list. `display.disabled` is a business state.
 */
export const DISPLAY_REFUSALS: readonly ErrorCode[] = [];
export const DISPLAY_RECHECK: readonly ErrorCode[] = [
  "general.permission",
  "general.input",
];

/**
 * PUT/DELETE /api/parent/me/children/{id}/meal-participation[/{date},
 * /weeks/{date}] (`workflows/parentportal/http/child_write_errors.go`,
 * `renderParentCareWriteError`): the relationship to the child is gone
 * (`care.child_not_linked`, also on the read) or the parent token has no
 * account (401, class code).
 *
 * `care.guardian_permission_denied` is not a refusal: the write needs
 * `meal_participation.manage`, the read only `parent_portal.access`, and both
 * answer with this code. `care.child_care_ended`, `meals.registration_disabled`
 * and `meals.plan_disabled` are business states.
 */
export const MEAL_PARTICIPATION_REFUSALS: readonly ErrorCode[] = [
  "general.permission",
  "care.child_not_linked",
];
export const MEAL_PARTICIPATION_RECHECK: readonly ErrorCode[] = [
  "care.guardian_permission_denied",
];

const RECHECK_STATUSES = new Set([401, 403, 404]);

/** Whether a failed write leaves it open if the resource is still readable. */
export function needsRecheck(
  error: unknown,
  codes: readonly ErrorCode[],
): boolean {
  return (
    error instanceof ApiError &&
    error.status !== undefined &&
    RECHECK_STATUSES.has(error.status) &&
    error.code !== undefined &&
    codes.includes(error.code)
  );
}
