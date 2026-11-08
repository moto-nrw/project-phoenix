import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPut } from "~/lib/traeger/route-wrapper.server";

// Das Rechnungsdatum eines Monatslaufs in der Prüfung (#5916).
export const PUT = proxyPut(
  (params) =>
    `/traeger/care-billing-runs/${requirePathSegmentParam(params)}/invoice-date`,
);
