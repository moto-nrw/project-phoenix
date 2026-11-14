import { proxyPut } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Rollen einer Person im Träger-Büro ändern (#4650). Neue Rollen wirken bei
// der nächsten Erneuerung ihrer Sitzung; eine entzogene Rolle beendet ihre
// Sitzungen sofort (#6850).
export const PUT = proxyPut(
  (params) =>
    `/traeger/accounts/${requirePathSegmentParam(params, "accountId")}/roles`,
);
