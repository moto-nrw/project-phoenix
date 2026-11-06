import { proxyGet, proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// The backend requires config:manage and rejects a missing status or reason.
export const GET = proxyGet(
  (p) => `/api/students/${requirePathSegmentParam(p)}/family-protection`,
);

export const PUT = proxyPut(
  (p) => `/api/students/${requirePathSegmentParam(p)}/family-protection`,
);
