import { proxyPost } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// POST /api/students/[id]/pickup-exceptions/bulk - Apply one pickup override to
// several dates at once
export const POST = proxyPost(
  (p) => `/api/students/${requirePathSegmentParam(p)}/pickup-exceptions/bulk`,
);
