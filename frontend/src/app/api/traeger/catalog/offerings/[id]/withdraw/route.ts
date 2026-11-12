import { proxyPost } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Ein Betreuungsangebot des Trägers zurückziehen (#4704). Aus den Schulen
// verschwindet es erst mit dem nächsten Übertragen.
export const POST = proxyPost(
  (params) =>
    `/traeger/catalog/offerings/${requirePathSegmentParam(params)}/withdraw`,
);
