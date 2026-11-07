import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { proxyFile } from "~/lib/tenant-file-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Dateiablage (#2596): the bytes of one file. ?inline=1 asks for in-browser
// viewing (PDF, images); the backend decides per content type and answers
// with a sandboxing CSP.
export const GET = withPrivateDownloadNoStore(
  proxyFile(
    (p) =>
      `/api/files/folders/${requirePathSegmentParam(p, "folderId")}/files/${requirePathSegmentParam(p, "fileId")}/download`,
    { forwardQuery: ["inline"] },
  ),
);
