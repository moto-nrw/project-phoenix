import { proxyGet, proxyPost } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// GET /api/guardians/[id]/phone-numbers - List guardian phone numbers
export const GET = proxyGet(
  (p) => `/api/guardians/${requirePathSegmentParam(p)}/phone-numbers`,
);

// POST /api/guardians/[id]/phone-numbers - Add phone number
export const POST = proxyPost(
  (p) => `/api/guardians/${requirePathSegmentParam(p)}/phone-numbers`,
);
