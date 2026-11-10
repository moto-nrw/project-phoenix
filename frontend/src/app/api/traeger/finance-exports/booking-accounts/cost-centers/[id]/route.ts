import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyDelete } from "~/lib/traeger/route-wrapper.server";

// Kostenstelle löschen (#4821); ausgestellte Positionen behalten ihre Werte.
export const DELETE = proxyDelete(
  (params) =>
    `/traeger/finance-exports/booking-accounts/cost-centers/${requirePathSegmentParam(params)}`,
);
