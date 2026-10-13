import { proxyPut, proxyDelete } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// PUT /api/students/[id]/departure-exceptions/[date] - Abholart eines Tages setzen (#3945)
export const PUT = proxyPut(
  (p) =>
    `/api/students/${requirePathSegmentParam(p)}/departure-exceptions/${requirePathSegmentParam(p, "date")}`,
);

// DELETE /api/students/[id]/departure-exceptions/[date] - Tag wieder auf den Wochenplan setzen
export const DELETE = proxyDelete(
  (p) =>
    `/api/students/${requirePathSegmentParam(p)}/departure-exceptions/${requirePathSegmentParam(p, "date")}`,
);
