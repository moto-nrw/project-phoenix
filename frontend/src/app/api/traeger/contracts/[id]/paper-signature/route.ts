import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyFile, proxyUpload } from "~/lib/traeger/route-wrapper.server";

// Ein auf Papier unterschriebener Vertrag (#4679): das Träger-Büro lädt das
// PDF hoch und nennt, wer wann unterschrieben hat. Das Backend prüft Größe,
// Endung und Inhalt. GET gibt das hochgeladene PDF zurück.
const PAPER_MAX_MB = 10;

const path = (params: Record<string, unknown>) =>
  `/traeger/contracts/${requirePathSegmentParam(params)}/paper-signature`;

export const POST = proxyUpload(path, PAPER_MAX_MB * 1024 * 1024, {
  method: "POST",
  tooLargeCode: "care_contract.paper_document_invalid",
});

export const GET = withPrivateDownloadNoStore(proxyFile(path));
