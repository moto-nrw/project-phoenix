import { proxyGet, proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Klassen-Zuweisung einer Lehrkraft (#1772): Lesen users:read, Ersetzen
// users:manage — beides erzwingt das Backend.
export const GET = proxyGet(
  (p) => `/api/staff/${requirePathSegmentParam(p)}/school-classes`,
);

export const PUT = proxyPut(
  (p) => `/api/staff/${requirePathSegmentParam(p)}/school-classes`,
);
