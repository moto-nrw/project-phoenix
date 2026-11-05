import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Füllt eine Vorlage mit Beispieldaten, ohne zu speichern (#4677).
export const POST = proxyPost(
  (params) =>
    `/traeger/master-data/document-templates/${requirePathSegmentParam(params, "kind")}/preview`,
);
