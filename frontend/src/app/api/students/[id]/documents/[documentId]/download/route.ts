import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { proxyFile } from "~/lib/tenant-file-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Child document bytes (#777).
export const GET = withPrivateDownloadNoStore(
  proxyFile(
    (p) =>
      `/api/students/${requirePathSegmentParam(p)}/documents/${requirePathSegmentParam(p, "documentId")}/download`,
  ),
);
