import { proxyPost } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// POST /api/students/[id]/arrival-notes - Create an arrival note
export const POST = proxyPost(
  (p) => `/api/students/${requirePathSegmentParam(p)}/arrival-notes`,
);
