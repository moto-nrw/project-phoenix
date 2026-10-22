import { proxyPost } from "~/lib/operator/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const POST = proxyPost(
  (p) => `/operator/announcements/${requirePathSegmentParam(p)}/publish`,
);
