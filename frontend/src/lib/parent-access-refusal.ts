import { ApiError } from "~/lib/api-error";

// renderParentCareWriteError: unrelated business refusals must not evict a child.
export const PARENT_ACCESS_REFUSAL_CODES = [
  "care.child_not_linked",
  "care.guardian_permission_denied",
] as const;
export const PARENT_ACCESS_REFUSED_EVENT = "parent-access-refused";

export function isParentAccessRefusal(error: unknown): error is ApiError {
  return (
    error instanceof ApiError &&
    (error.status === 401 || error.status === 403 || error.status === 404) &&
    PARENT_ACCESS_REFUSAL_CODES.some((code) => error.code === code)
  );
}
