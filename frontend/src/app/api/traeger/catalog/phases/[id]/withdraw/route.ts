import { proxyPost } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Anmeldephase des Trägers zurückziehen (#4704). Aus den Schulen verschwindet
// sie erst mit dem nächsten Übertragen.
export const POST = proxyPost(
  (params) =>
    `/traeger/catalog/phases/${requirePathSegmentParam(params)}/withdraw`,
);
