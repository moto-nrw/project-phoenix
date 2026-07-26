import { proxyPost } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// POST /api/students/[id]/pickup-notes - Create a pickup note
export const POST = proxyPost(
  (p) => `/api/students/${requirePathSegmentParam(p)}/pickup-notes`,
);
