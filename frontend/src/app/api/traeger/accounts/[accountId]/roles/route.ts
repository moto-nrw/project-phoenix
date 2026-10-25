import { proxyPut } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Rollen einer Person im Träger-Büro ändern (#4650). Sie wirken bei der
// nächsten Erneuerung ihrer Sitzung.
export const PUT = proxyPut(
  (params) =>
    `/traeger/accounts/${requirePathSegmentParam(params, "accountId")}/roles`,
);
