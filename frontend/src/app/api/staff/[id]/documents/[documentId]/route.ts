import { proxyDelete } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * DELETE /api/staff/[id]/documents/[documentId]
 * Audited soft delete of one staff document (#1424); the backend removes the
 * stored file bytes after the metadata transaction commits.
 */
export const DELETE = proxyDelete(
  (p) =>
    `/api/staff/${requirePathSegmentParam(p)}/documents/${requirePathSegmentParam(p, "documentId")}`,
);
