import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Die Vorlage einer Dokumentart mit Platzhaltern und Versionen (#4677).
export const GET = proxyGet(
  (params) =>
    `/traeger/master-data/document-templates/${requirePathSegmentParam(params, "kind")}`,
);
