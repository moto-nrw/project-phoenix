import { proxyGet, proxyPut } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Ein Ferienzeitraum des Trägers (#4659): lesen und mit Wochen und
// Zeitmodellen ändern.
export const GET = proxyGet(
  (params) => `/traeger/holiday-periods/${requirePathSegmentParam(params)}`,
);

export const PUT = proxyPut(
  (params) => `/traeger/holiday-periods/${requirePathSegmentParam(params)}`,
);
