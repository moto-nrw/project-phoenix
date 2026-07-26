import { proxyPost } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// POST /api/students/[id]/pickup-exceptions - Create a pickup exception
export const POST = proxyPost(
  (p) => `/api/students/${requirePathSegmentParam(p)}/pickup-exceptions`,
);
