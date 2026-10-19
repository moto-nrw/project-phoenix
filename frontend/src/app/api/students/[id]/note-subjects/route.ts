import { proxyGet } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// GET /api/students/[id]/note-subjects - What a new note may refer to
export const GET = proxyGet(
  (p) => `/api/students/${requirePathSegmentParam(p)}/note-subjects`,
);
