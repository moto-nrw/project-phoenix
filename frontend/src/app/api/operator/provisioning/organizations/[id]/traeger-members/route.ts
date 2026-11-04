import { proxyGet, proxyPost } from "~/lib/operator/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Träger-Büro eines Trägers (#4649): Personen mit Zugang zum Träger-Portal
// lesen und eine Person mit einer oder mehreren Träger-Rollen zuordnen.
export const GET = proxyGet(
  (params) =>
    `/operator/organizations/${requirePathSegmentParam(params)}/traeger-members`,
);

export const POST = proxyPost(
  (params) =>
    `/operator/organizations/${requirePathSegmentParam(params)}/traeger-members`,
);
