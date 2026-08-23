import { proxyGet } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// GET /api/students/class-arrival-times/[schoolClass] - Unterrichtsschluss of one school class
export const GET = proxyGet(
  (p) =>
    `/api/students/class-arrival-times/${requirePathSegmentParam(p, "schoolClass")}`,
);
