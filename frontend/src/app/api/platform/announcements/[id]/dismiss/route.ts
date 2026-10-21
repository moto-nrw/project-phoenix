import { proxyPost } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const POST = proxyPost(
  (p) => `/api/platform/announcements/${requirePathSegmentParam(p)}/dismiss`,
);
