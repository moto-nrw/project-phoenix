import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyFile } from "~/lib/school/route-wrapper.server";

// Anhang einer Nachricht im Team-Chat des Schulportals (#6685).
export const GET = withPrivateDownloadNoStore(
  proxyFile(
    (p) =>
      `/school-staff-messages/attachments/${requirePathSegmentParam(p, "attachmentId")}/download`,
    { forwardQuery: ["inline"] },
  ),
);
