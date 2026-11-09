import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { proxyFile } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Die Datei eines Dokuments, das der Träger für die Schule freigab (#4692).
// Das Backend zeichnet das Öffnen auf.
export const GET = withPrivateDownloadNoStore(
  proxyFile((p) => `/api/traeger-documents/${requirePathSegmentParam(p)}/file`),
);
