import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { proxyFile } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Ein Nachweis für den Leistungsbezug einer Ferienanmeldung (#5238). Das
// Backend protokolliert jeden Abruf und schickt die Datei inline mit
// Sandbox und `no-store`.
export const GET = withPrivateDownloadNoStore(
  proxyFile(
    (params) =>
      `/traeger/holiday-periods/${requirePathSegmentParam(params)}/registrations/${requirePathSegmentParam(params, "registrationId")}/proofs/${requirePathSegmentParam(params, "proofId")}`,
  ),
);
