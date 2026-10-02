import { proxyPut, proxyDelete } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// PUT /api/students/[id]/notes/[noteId] - Correct one note (author only)
export const PUT = proxyPut(
  (p) =>
    `/api/students/${requirePathSegmentParam(p)}/notes/${requirePathSegmentParam(p, "noteId")}`,
);

// DELETE /api/students/[id]/notes/[noteId] - Remove one note (group leads)
export const DELETE = proxyDelete(
  (p) =>
    `/api/students/${requirePathSegmentParam(p)}/notes/${requirePathSegmentParam(p, "noteId")}`,
);
