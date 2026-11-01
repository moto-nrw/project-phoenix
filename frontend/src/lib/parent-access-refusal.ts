import { ApiError } from "~/lib/api-error";

// renderParentCareWriteError: unrelated business refusals must not evict a child.
export const PARENT_ACCESS_REFUSAL_CODES = [
  "care.child_not_linked",
  "care.guardian_permission_denied",
] as const;
export const PARENT_ACCESS_REFUSED_EVENT = "parent-access-refused";

// Action-specific read or write refusals do not prove loss of portal access.
// Only readers that require parent_portal.access alone may opt in.
export function isParentAccessRefusal(
  error: unknown,
  method = "GET",
  checksPortalAccess = false,
): error is ApiError {
  return (
    error instanceof ApiError &&
    (error.status === 401 || error.status === 403 || error.status === 404) &&
    (error.code === "care.child_not_linked" ||
      (checksPortalAccess &&
        method === "GET" &&
        error.code === "care.guardian_permission_denied"))
  );
}
