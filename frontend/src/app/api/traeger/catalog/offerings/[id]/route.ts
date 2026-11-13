import { proxyPut } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Ein Betreuungsangebot des Trägers ändern (#4704).
export const PUT = proxyPut(
  (params) => `/traeger/catalog/offerings/${requirePathSegmentParam(params)}`,
);
