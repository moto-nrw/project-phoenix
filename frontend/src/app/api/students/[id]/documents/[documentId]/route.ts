import { proxyDelete } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * DELETE /api/students/[id]/documents/[documentId]
 * Audited soft delete of one child document (#777); the backend removes the
 * stored bytes after the metadata transaction commits.
 */
export const DELETE = proxyDelete(
  (p) =>
    `/api/students/${requirePathSegmentParam(p)}/documents/${requirePathSegmentParam(p, "documentId")}`,
);
