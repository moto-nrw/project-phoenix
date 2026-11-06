import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPut } from "~/lib/traeger/route-wrapper.server";

// Die Vertragsvorlage eines Angebots speichern (#4678).
export const PUT = proxyPut(
  (params) =>
    `/traeger/contract-templates/${requirePathSegmentParam(params, "schoolId")}/${requirePathSegmentParam(params, "offeringId")}`,
);
