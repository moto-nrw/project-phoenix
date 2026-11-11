import { school } from "~/lib/school/route-wrapper.server";
import { MAX_MESSAGE_ATTACHMENT_MB } from "~/lib/message-attachments";

// Fotos und Dateien für den Team-Chat im Schulportal (#6685). Die Route
// liegt im Backend neben den übrigen Schulportal-Strömen an der Wurzel.
export const POST = school.proxyUpload(
  "/school-staff-messages/attachments",
  MAX_MESSAGE_ATTACHMENT_MB * 1024 * 1024,
  {
    method: "POST",
    tooLargeCode: "files.file_too_large",
    tooLargeDetails: { max_mb: MAX_MESSAGE_ATTACHMENT_MB },
  },
);
