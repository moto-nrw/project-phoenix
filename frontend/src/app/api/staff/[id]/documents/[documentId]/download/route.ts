import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { proxyFile } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Staff document bytes (#1424).
export const GET = withPrivateDownloadNoStore(
  proxyFile(
    (p) =>
      `/api/staff/${requirePathSegmentParam(p)}/documents/${requirePathSegmentParam(p, "documentId")}/download`,
    { uploadedFile: true },
  ),
);
