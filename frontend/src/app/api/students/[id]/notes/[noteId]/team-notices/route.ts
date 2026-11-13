import { proxyPost } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// POST /api/students/[id]/notes/[noteId]/team-notices - Das Team über eine
// gespeicherte Notiz informieren (#6854)
export const POST = proxyPost(
  (p) =>
    `/api/students/${requirePathSegmentParam(p)}/notes/${requirePathSegmentParam(p, "noteId")}/team-notices`,
);
