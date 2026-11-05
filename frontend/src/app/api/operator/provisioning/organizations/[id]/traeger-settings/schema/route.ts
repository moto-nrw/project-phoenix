import { proxyGet } from "~/lib/operator/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Einstellungen eines Trägers im Operator-Dashboard (#5748): dieselben
// Einstellungen wie im Träger-Portal, jede änderbar.
export const GET = proxyGet(
  (params) =>
    `/operator/organizations/${requirePathSegmentParam(params)}/traeger-settings/schema`,
);
