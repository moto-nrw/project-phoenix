import { proxyPut, proxyDelete } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// PUT /api/guardians/[id]/phone-numbers/[phoneId] - Update phone number
export const PUT = proxyPut(
  (p) =>
    `/api/guardians/${requirePathSegmentParam(p)}/phone-numbers/${requirePathSegmentParam(p, "phoneId")}`,
);

// DELETE /api/guardians/[id]/phone-numbers/[phoneId] - Delete phone number
export const DELETE = proxyDelete(
  (p) =>
    `/api/guardians/${requirePathSegmentParam(p)}/phone-numbers/${requirePathSegmentParam(p, "phoneId")}`,
);
