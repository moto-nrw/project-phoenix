import { tenant } from "~/lib/route-wrapper.server";
import { MAX_MESSAGE_ATTACHMENT_MB } from "~/lib/message-attachments";

// Fotos und Dateien für den Team-Chat (#6685).
export const POST = tenant.proxyUpload(
  "/api/staff-messages/attachments",
  MAX_MESSAGE_ATTACHMENT_MB * 1024 * 1024,
  {
    method: "POST",
    tooLargeCode: "files.file_too_large",
    tooLargeDetails: { max_mb: MAX_MESSAGE_ATTACHMENT_MB },
  },
);
