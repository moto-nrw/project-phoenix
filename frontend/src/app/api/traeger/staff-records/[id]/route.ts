import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyDelete, proxyGet } from "~/lib/traeger/route-wrapper.server";

// Eine Personalakte lesen oder auflösen (#4705).
export const GET = proxyGet(
  (params) => `/traeger/staff-records/${requirePathSegmentParam(params)}`,
);

export const DELETE = proxyDelete(
  (params) => `/traeger/staff-records/${requirePathSegmentParam(params)}`,
);
