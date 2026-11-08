import "server-only";
import { bffError } from "./api-response-error.server";

/** The request and child a flattened enrollment admin route names in its body. */
export function enrollmentChildPath(
  body: { request_id?: string | number; child_id?: string | number },
  action: string,
): string {
  const requestID = body.request_id == null ? "" : String(body.request_id);
  const childID = body.child_id == null ? "" : String(body.child_id);
  if (!requestID || !childID) {
    throw bffError(400, "request_id and child_id are required");
  }
  return `/api/enrollment/admin/requests/${encodeURIComponent(requestID)}/children/${encodeURIComponent(childID)}/${action}`;
}
