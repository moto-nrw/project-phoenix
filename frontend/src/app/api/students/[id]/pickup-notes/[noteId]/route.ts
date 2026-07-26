import { proxyPut, proxyDelete } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// PUT /api/students/[id]/pickup-notes/[noteId] - Update a pickup note
export const PUT = proxyPut(
  (p) =>
    `/api/students/${requirePathSegmentParam(p)}/pickup-notes/${requirePathSegmentParam(p, "noteId")}`,
);

// DELETE /api/students/[id]/pickup-notes/[noteId] - Delete a pickup note
export const DELETE = proxyDelete(
  (p) =>
    `/api/students/${requirePathSegmentParam(p)}/pickup-notes/${requirePathSegmentParam(p, "noteId")}`,
);
