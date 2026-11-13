import { proxyGet, proxyPut } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Eine Anmeldephase des Trägers mit ihren Angeboten (#4704): lesen und
// ändern.
export const GET = proxyGet(
  (params) => `/traeger/catalog/phases/${requirePathSegmentParam(params)}`,
);

export const PUT = proxyPut(
  (params) => `/traeger/catalog/phases/${requirePathSegmentParam(params)}`,
);
