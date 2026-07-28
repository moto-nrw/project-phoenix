import { proxyPost } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// POST /api/students/[id]/arrival-exceptions/bulk - Apply one arrival override
// to several dates at once
export const POST = proxyPost(
  (p) => `/api/students/${requirePathSegmentParam(p)}/arrival-exceptions/bulk`,
);
