import { proxyDelete } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Person aus dem Träger-Büro entfernen (#4650). Ihre Träger-Sitzungen enden
// sofort.
export const DELETE = proxyDelete(
  (params) =>
    `/traeger/accounts/${requirePathSegmentParam(params, "accountId")}`,
);
