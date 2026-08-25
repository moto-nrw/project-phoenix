// Dateiablage (#2596): audited soft delete of one file; the backend removes
// the stored bytes after the metadata transaction commits.
import { proxyDelete } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const DELETE = proxyDelete(
  (p) =>
    `/api/files/folders/${requirePathSegmentParam(p, "folderId")}/files/${requirePathSegmentParam(p, "fileId")}`,
);
