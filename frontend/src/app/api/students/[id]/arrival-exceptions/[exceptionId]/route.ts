import { proxyPut, proxyDelete } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// PUT /api/students/[id]/arrival-exceptions/[exceptionId] - Update an arrival exception
export const PUT = proxyPut(
  (p) =>
    `/api/students/${requirePathSegmentParam(p)}/arrival-exceptions/${requirePathSegmentParam(p, "exceptionId")}`,
);

// DELETE /api/students/[id]/arrival-exceptions/[exceptionId] - Delete an arrival exception
export const DELETE = proxyDelete(
  (p) =>
    `/api/students/${requirePathSegmentParam(p)}/arrival-exceptions/${requirePathSegmentParam(p, "exceptionId")}`,
);
