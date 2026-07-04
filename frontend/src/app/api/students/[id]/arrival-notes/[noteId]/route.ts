import { proxyPut, proxyDelete } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// PUT /api/students/[id]/arrival-notes/[noteId] - Update an arrival note
export const PUT = proxyPut(
  (p) =>
    `/api/students/${requirePathSegmentParam(p)}/arrival-notes/${requirePathSegmentParam(p, "noteId")}`,
);

// DELETE /api/students/[id]/arrival-notes/[noteId] - Delete an arrival note
export const DELETE = proxyDelete(
  (p) =>
    `/api/students/${requirePathSegmentParam(p)}/arrival-notes/${requirePathSegmentParam(p, "noteId")}`,
);
