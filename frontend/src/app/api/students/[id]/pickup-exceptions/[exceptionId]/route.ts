import { proxyPut, proxyDelete } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// PUT /api/students/[id]/pickup-exceptions/[exceptionId] - Update a pickup exception
export const PUT = proxyPut(
  (p) =>
    `/api/students/${requirePathSegmentParam(p)}/pickup-exceptions/${requirePathSegmentParam(p, "exceptionId")}`,
);

// DELETE /api/students/[id]/pickup-exceptions/[exceptionId] - Delete a pickup exception
export const DELETE = proxyDelete(
  (p) =>
    `/api/students/${requirePathSegmentParam(p)}/pickup-exceptions/${requirePathSegmentParam(p, "exceptionId")}`,
);
