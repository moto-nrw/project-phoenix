import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// One care-schedule request of any status, read from a message-thread pill
// (#3135). The backend checks users:update and the review scope of the child.
export const GET = proxyGet(
  (p) =>
    `/api/students/care-schedule-change-requests/${requirePathSegmentParam(p, "requestId")}`,
);
