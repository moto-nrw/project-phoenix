import { tenant } from "~/lib/route-wrapper.server";
import { MAX_MESSAGE_ATTACHMENT_MB } from "~/lib/message-attachments";

// Fotos und Dateien für Nachrichten an Eltern (#6685). Das Backend prüft
// Größe, Endung und Inhalt und entfernt die Metadaten eines Bildes.
export const POST = tenant.proxyUpload(
  "/api/messages/attachments",
  MAX_MESSAGE_ATTACHMENT_MB * 1024 * 1024,
  {
    method: "POST",
    tooLargeCode: "files.file_too_large",
    tooLargeDetails: { max_mb: MAX_MESSAGE_ATTACHMENT_MB },
  },
);
