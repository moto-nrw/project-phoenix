import { proxyPut } from "~/lib/operator/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Hinweis und Wahl des Kunden an einem offenen Löschauftrag nachtragen, der
// ohne beides erfasst wurde (#4720), auch nach Beginn der Löschung.
export const PUT = proxyPut(
  (params) =>
    `/operator/organizations/${requirePathSegmentParam(params)}/erasure-order/records`,
);
