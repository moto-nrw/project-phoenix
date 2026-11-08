import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyDelete } from "~/lib/traeger/route-wrapper.server";

// Eine ausgeschlossene Rechnung wieder in den Lauf nehmen (#5916).
export const DELETE = proxyDelete(
  (params) =>
    `/traeger/care-billing-runs/${requirePathSegmentParam(params)}/exclusions/${requirePathSegmentParam(params, "contractId")}`,
);
