import { proxyPost } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Neues Betreuungsangebot einer Anmeldephase des Trägers (#4704).
export const POST = proxyPost(
  (params) =>
    `/traeger/catalog/phases/${requirePathSegmentParam(params)}/offerings`,
);
