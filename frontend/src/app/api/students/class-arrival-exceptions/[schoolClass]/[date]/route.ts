import { proxyPut, proxyDelete } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// PUT /api/students/class-arrival-exceptions/[schoolClass]/[date] - set the class-wide arrival time of one date (#2962)
export const PUT = proxyPut(
  (p) =>
    `/api/students/class-arrival-exceptions/${requirePathSegmentParam(p, "schoolClass")}/${requirePathSegmentParam(p, "date")}`,
);

// DELETE /api/students/class-arrival-exceptions/[schoolClass]/[date] - remove it again
export const DELETE = proxyDelete(
  (p) =>
    `/api/students/class-arrival-exceptions/${requirePathSegmentParam(p, "schoolClass")}/${requirePathSegmentParam(p, "date")}`,
);
