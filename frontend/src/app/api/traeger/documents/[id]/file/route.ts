import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyFile } from "~/lib/traeger/route-wrapper.server";

// Die Datei eines Dokuments des Trägers (#4692), immer als Download.
export const GET = proxyFile(
  (params) => `/traeger/documents/${requirePathSegmentParam(params)}/file`,
);
