import { proxyPost } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Einen Gruppenchat verlassen (#4776).
export const POST = proxyPost(
  (p) =>
    `/api/staff-messages/threads/${requirePathSegmentParam(p, "threadID")}/leave`,
);
