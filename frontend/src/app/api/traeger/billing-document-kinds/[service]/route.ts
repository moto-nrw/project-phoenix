import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPut } from "~/lib/traeger/route-wrapper.server";

// Die Dokumentart einer Leistung wählen (#4688).
export const PUT = proxyPut(
  (params) =>
    `/traeger/billing-document-kinds/${requirePathSegmentParam(params, "service")}`,
);
