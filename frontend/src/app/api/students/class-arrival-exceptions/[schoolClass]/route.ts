import { proxyGet } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// GET /api/students/class-arrival-exceptions/[schoolClass] - class-wide arrival day exceptions (#2962)
export const GET = proxyGet(
  (p) =>
    `/api/students/class-arrival-exceptions/${requirePathSegmentParam(p, "schoolClass")}`,
);
