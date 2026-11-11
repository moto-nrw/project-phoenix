import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { proxyFile } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Die Ferienanmeldungen eines Zeitraums als ZIP mit einem PDF je Anmeldung
// (#6691). Das Backend protokolliert den Abruf und schickt das Archiv Datei
// für Datei.
export const GET = withPrivateDownloadNoStore(
  proxyFile(
    (params) =>
      `/traeger/holiday-periods/${requirePathSegmentParam(params)}/registrations/pdf`,
  ),
);
