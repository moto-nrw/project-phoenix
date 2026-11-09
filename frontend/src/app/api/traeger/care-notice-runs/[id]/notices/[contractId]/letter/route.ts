import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Seite 1 eines Jahresbescheids mit dem Briefkopf des Trägers (#6219).
export const GET = proxyGet(
  (params) =>
    `/traeger/care-notice-runs/${requirePathSegmentParam(params)}/notices/${requirePathSegmentParam(params, "contractId")}/letter`,
);
