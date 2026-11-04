import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyDelete, proxyPut } from "~/lib/traeger/route-wrapper.server";

// Eine aus der stornierten Rechnung übernommene Korrektur ändern (#5663).
export const PUT = proxyPut(
  (params: Record<string, unknown>) =>
    `/traeger/billing-runs/${requirePathSegmentParam(params)}/carried-corrections/${requirePathSegmentParam(params, "positionId")}`,
);

// Eine aus der stornierten Rechnung übernommene Korrektur entfernen (#5663).
export const DELETE = proxyDelete(
  (params: Record<string, unknown>) =>
    `/traeger/billing-runs/${requirePathSegmentParam(params)}/carried-corrections/${requirePathSegmentParam(params, "positionId")}`,
);
