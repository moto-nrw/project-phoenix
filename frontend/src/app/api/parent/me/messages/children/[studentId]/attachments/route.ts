import { MAX_MESSAGE_ATTACHMENT_MB } from "~/lib/message-attachments";
import { parent } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Fotos und Dateien, die Eltern an eine Nachricht hängen (#6685). Das
// Backend prüft das Recht je Kind sowie Größe, Endung und Inhalt.
export const POST = parent.proxyUpload(
  (p) =>
    `/parent/me/messages/children/${requirePathSegmentParam(p, "studentId")}/attachments`,
  MAX_MESSAGE_ATTACHMENT_MB * 1024 * 1024,
  {
    method: "POST",
    tooLargeCode: "files.file_too_large",
    tooLargeDetails: { max_mb: MAX_MESSAGE_ATTACHMENT_MB },
  },
);
