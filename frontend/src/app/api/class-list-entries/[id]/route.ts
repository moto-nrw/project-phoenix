import { proxyDelete, proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Klassenlisteneintrag bearbeiten/löschen (#2382): Ändern users:update,
// Löschen users:delete — beides erzwingt das Backend.
export const PUT = proxyPut(
  (p) => `/api/class-list-entries/${requirePathSegmentParam(p)}`,
);

export const DELETE = proxyDelete(
  (p) => `/api/class-list-entries/${requirePathSegmentParam(p)}`,
);
