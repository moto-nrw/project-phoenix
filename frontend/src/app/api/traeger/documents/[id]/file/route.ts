import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyFile } from "~/lib/traeger/route-wrapper.server";

// Die Datei eines Dokuments des Trägers (#4692), immer als Download und nie
// aus einem Cache.
export const GET = withPrivateDownloadNoStore(
  proxyFile(
    (params) => `/traeger/documents/${requirePathSegmentParam(params)}/file`,
  ),
);
