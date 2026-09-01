// Anhänge an Elternmitteilungen (#2890): einen Anhang entfernen. Nur solange
// die Mitteilung ein Entwurf ist — das Backend entscheidet und antwortet
// sonst mit 409.

import { proxyDelete } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const DELETE = proxyDelete(
  (p) =>
    `/api/announcement-attachments/${requirePathSegmentParam(p, "announcementId")}/${requirePathSegmentParam(p, "attachmentId")}`,
);
