import { proxyGet, proxyPut } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Anmeldeformular eines Ferienzeitraums (#5198): die aktuelle Version lesen
// und als neue Version speichern.
export const GET = proxyGet(
  (params) =>
    `/traeger/holiday-periods/${requirePathSegmentParam(params)}/form`,
);

export const PUT = proxyPut(
  (params) =>
    `/traeger/holiday-periods/${requirePathSegmentParam(params)}/form`,
);
