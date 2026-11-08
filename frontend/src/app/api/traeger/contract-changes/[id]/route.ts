import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Der offene Änderungswunsch der Eltern zum Kind eines Vertrags (#5979).
export const GET = proxyGet(
  (params) => `/traeger/contract-changes/${requirePathSegmentParam(params)}`,
);
