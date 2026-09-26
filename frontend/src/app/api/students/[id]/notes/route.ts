import { proxyGet, proxyPost } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// GET /api/students/[id]/notes - Read a child's note card (Kartei)
export const GET = proxyGet(
  (p) => `/api/students/${requirePathSegmentParam(p)}/notes`,
);

// POST /api/students/[id]/notes - Write one note
export const POST = proxyPost(
  (p) => `/api/students/${requirePathSegmentParam(p)}/notes`,
);
