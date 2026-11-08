import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { proxyFile } from "~/lib/route-wrapper.server";
import { requireNumericParam } from "~/lib/route-wrapper-utils.server";

// Wochenprotokoll (#4769): das PDF eines abgeschlossenen Protokolls. Das
// Backend prüft die Reichweite der Person und schreibt vor der Auslieferung
// den Eintrag ins Zugriffsprotokoll; 403/404/409 laufen unverändert durch.
export const GET = withPrivateDownloadNoStore(
  proxyFile(
    (params) =>
      `/api/week-protocols/${requireNumericParam(params, "protocolId")}/pdf`,
  ),
);
