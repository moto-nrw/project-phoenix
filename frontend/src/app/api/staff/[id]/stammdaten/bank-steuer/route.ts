import { proxyGet, proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Bank & tax section (#1423): staff:financial only; the backend serves
// masked values here and writes a GDPR access-log row per read.
export const GET = proxyGet(
  (p) => `/api/staff/${requirePathSegmentParam(p)}/stammdaten/bank-steuer`,
);

export const PUT = proxyPut(
  (p) => `/api/staff/${requirePathSegmentParam(p)}/stammdaten/bank-steuer`,
);
