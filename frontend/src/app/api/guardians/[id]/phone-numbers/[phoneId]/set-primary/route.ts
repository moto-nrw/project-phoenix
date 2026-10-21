import { proxyPost } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// POST /api/guardians/[id]/phone-numbers/[phoneId]/set-primary - Set as primary
export const POST = proxyPost(
  (p) =>
    `/api/guardians/${requirePathSegmentParam(p)}/phone-numbers/${requirePathSegmentParam(p, "phoneId")}/set-primary`,
);
