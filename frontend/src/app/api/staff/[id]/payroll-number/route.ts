import { proxyGet, proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const GET = proxyGet(
  (p) => `/api/staff/${requirePathSegmentParam(p)}/payroll-number`,
);

export const PUT = proxyPut(
  (p) => `/api/staff/${requirePathSegmentParam(p)}/payroll-number`,
);
