import { proxyGet, proxyPost } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const GET = proxyGet(
  (p) => `/api/students/${requirePathSegmentParam(p)}/status-days`,
);

export const POST = proxyPost(
  (p) => `/api/students/${requirePathSegmentParam(p)}/status-days`,
);
