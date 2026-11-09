import { parent } from "~/lib/parent/route-wrapper.server";
import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Anhänge einer Elternmitteilung (#2890), Elternseite: die Datei selbst. Der
// Empfängerkreis der Mitteilung entscheidet im Backend; wer nicht dazugehört,
// bekommt 404 — nicht 403, sonst ließe sich an der Antwort ablesen, dass es
// die Mitteilung gibt.
export const GET = withPrivateDownloadNoStore(
  parent.proxyFile(
    (params) =>
      `/parent-news-attachments/${requirePathSegmentParam(params, "announcementId")}/${requirePathSegmentParam(params, "attachmentId")}/download`,
    {
      forwardQuery: ["student_id", "inline"],
      backendCacheControl: true,
    },
  ),
);
