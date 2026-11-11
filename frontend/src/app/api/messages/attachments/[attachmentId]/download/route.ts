import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { proxyFile } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Anhang einer Nachricht an Eltern (#6685). ?inline=1 zeigt ein Bild oder
// PDF im Browser; wer die Unterhaltung nicht liest, bekommt 404.
export const GET = withPrivateDownloadNoStore(
  proxyFile(
    (p) =>
      `/api/messages/attachments/${requirePathSegmentParam(p, "attachmentId")}/download`,
    { forwardQuery: ["inline"] },
  ),
);
