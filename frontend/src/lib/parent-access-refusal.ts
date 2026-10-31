import { ApiError } from "~/lib/api-error";

// renderParentCareWriteError: unrelated business refusals must not evict a child.
export const PARENT_ACCESS_REFUSAL_CODES = [
  "care.child_not_linked",
  "care.guardian_permission_denied",
] as const;
export const PARENT_ACCESS_REFUSED_EVENT = "parent-access-refused";

// Denying a write may leave the child readable (#5425); do not suppress its action error.
export function isParentAccessRefusal(
  error: unknown,
  method = "GET",
): error is ApiError {
  return (
    error instanceof ApiError &&
    (error.status === 401 || error.status === 403 || error.status === 404) &&
    (error.code === "care.child_not_linked" ||
      (method === "GET" && error.code === "care.guardian_permission_denied"))
  );
}
