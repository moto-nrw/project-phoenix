import { parent } from "~/lib/parent/route-wrapper.server";
import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Anhang aus einer Unterhaltung der Eltern (#6685). Eine fremde Familie
// bekommt 404.
export const GET = withPrivateDownloadNoStore(
  parent.proxyFile(
    (p) =>
      `/parent/me/messages/children/${requirePathSegmentParam(p, "studentId")}/attachments/${requirePathSegmentParam(p, "attachmentId")}/download`,
    { forwardQuery: ["inline"] },
  ),
);
