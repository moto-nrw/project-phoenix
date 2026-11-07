import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { proxyFile } from "~/lib/tenant-file-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// One proof file of an enrollment request (#4783). The backend checks
// enrollment_uploads:read, audits every call and sends the file inline with
// a sandbox CSP.
export const GET = withPrivateDownloadNoStore(
  proxyFile(
    (p) =>
      `/api/enrollment/admin/requests/${requirePathSegmentParam(p)}/uploads/${requirePathSegmentParam(p, "uploadId")}`,
  ),
);
