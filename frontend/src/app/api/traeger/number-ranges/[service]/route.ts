import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPut } from "~/lib/traeger/route-wrapper.server";

// Das Muster der Rechnungsnummern einer Leistung festlegen (#4668).
export const PUT = proxyPut(
  (params) =>
    `/traeger/number-ranges/${requirePathSegmentParam(params, "service")}`,
);
