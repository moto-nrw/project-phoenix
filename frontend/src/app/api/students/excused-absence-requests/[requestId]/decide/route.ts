import { proxyPost } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Approves or rejects one parent „entschuldigt“ request. 409
// students.change_request_not_pending: already decided or withdrawn;
// students.guardian_access_revoked: the guardian lost access to the child.
export const POST = proxyPost(
  (p) =>
    `/api/students/excused-absence-requests/${requirePathSegmentParam(p, "requestId")}/decide`,
);
