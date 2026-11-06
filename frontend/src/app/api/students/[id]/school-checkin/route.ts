import { proxyPost } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// The backend checks users:checkin and a verified staff identity (#2329).
export const POST = proxyPost(
  (p) => `/api/students/${requirePathSegmentParam(p)}/school-checkin`,
);
