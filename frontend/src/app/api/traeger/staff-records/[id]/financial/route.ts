import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyGet, proxyPut } from "~/lib/traeger/route-wrapper.server";

// Bank- und Steuerdaten einer Akte: maskiert lesen und ersetzen (#4705).
export const GET = proxyGet(
  (params) =>
    `/traeger/staff-records/${requirePathSegmentParam(params)}/financial`,
);

export const PUT = proxyPut(
  (params) =>
    `/traeger/staff-records/${requirePathSegmentParam(params)}/financial`,
);
