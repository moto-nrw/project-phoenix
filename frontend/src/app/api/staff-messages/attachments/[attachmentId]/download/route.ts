import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { proxyFile } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Anhang einer Nachricht im Team-Chat (#6685), nur für die Teilnehmenden.
export const GET = withPrivateDownloadNoStore(
  proxyFile(
    (p) =>
      `/api/staff-messages/attachments/${requirePathSegmentParam(p, "attachmentId")}/download`,
    { forwardQuery: ["inline"] },
  ),
);
