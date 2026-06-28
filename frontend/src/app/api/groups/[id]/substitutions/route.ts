import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const GET = proxyGet(
  (p) => `/api/groups/${requirePathSegmentParam(p)}/substitutions`,
);
