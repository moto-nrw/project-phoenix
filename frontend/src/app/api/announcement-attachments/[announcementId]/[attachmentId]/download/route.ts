import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { proxyFile } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Anhänge an Elternmitteilungen (#2890): Personal-Vorschau der angehängten
// Datei. ?inline=1 fragt die Ansicht im Browser an; das Backend entscheidet
// je Dateityp und antwortet mit einer abschottenden CSP.
export const GET = withPrivateDownloadNoStore(
  proxyFile(
    (p) =>
      `/api/announcement-attachments/${requirePathSegmentParam(p, "announcementId")}/${requirePathSegmentParam(p, "attachmentId")}/download`,
    { forwardQuery: ["inline"], uploadedFile: true },
  ),
);
