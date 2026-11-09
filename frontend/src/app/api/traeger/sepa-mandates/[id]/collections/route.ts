import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Die Einzüge eines SEPA-Mandats je Rechnung (#4697), ohne Bankdaten.
export const GET = proxyGet(
  (params) =>
    `/traeger/sepa-mandates/${requirePathSegmentParam(params)}/collections`,
);
