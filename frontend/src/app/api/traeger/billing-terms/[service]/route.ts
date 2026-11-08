import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPut } from "~/lib/traeger/route-wrapper.server";

// Zahlungsweg, Zahlungsziel und Bündelung einer Leistung festlegen (#4689).
export const PUT = proxyPut(
  (params) =>
    `/traeger/billing-terms/${requirePathSegmentParam(params, "service")}`,
);
