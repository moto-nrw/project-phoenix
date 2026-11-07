import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { proxyFile } from "~/lib/tenant-file-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Abholerlaubnis (#4780): Nachweis einer unterschriebenen Elternanfrage. Das
// Backend prüft users:update und den Prüfbereich für das Kind.
export const GET = withPrivateDownloadNoStore(
  proxyFile(
    (p) =>
      `/api/students/care-schedule-change-requests/${requirePathSegmentParam(p, "requestId")}/proof`,
  ),
);
