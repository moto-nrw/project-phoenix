import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyFile } from "~/lib/traeger/route-wrapper.server";

// Die Sollstellung eines ausgestellten Laufs (#4693). Jeder Abruf bekommt
// die nächste Dateinummer und steht im Exportprotokoll; niemand speichert
// die Datei zwischen.
export const GET = withPrivateDownloadNoStore(
  proxyFile(
    (params) =>
      `/traeger/finance-exports/runs/${requirePathSegmentParam(params, "kind")}/${requirePathSegmentParam(params)}/sollstellung`,
  ),
);
