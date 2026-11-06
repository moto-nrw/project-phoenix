import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Speichert eine Vorlage als neue Version (#4677).
export const POST = proxyPost(
  (params) =>
    `/traeger/master-data/document-templates/${requirePathSegmentParam(params, "kind")}/versions`,
);
