import { proxyDelete, proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Geplanter Austritt einer Person (#4809): letzten Arbeitstag eintragen oder
// ändern (PUT) und zurücknehmen (DELETE).
export const PUT = proxyPut(
  (p) => `/api/staff/${requirePathSegmentParam(p)}/planned-exit`,
);

export const DELETE = proxyDelete(
  (p) => `/api/staff/${requirePathSegmentParam(p)}/planned-exit`,
);
