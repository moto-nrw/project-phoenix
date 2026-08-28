import { proxyGet, proxyPut } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// GET /api/guardians/[id]/payment - masked bank details of one guardian
export const GET = proxyGet(
  (p) => `/api/guardians/${requirePathSegmentParam(p, "id")}/payment`,
);

// PUT /api/guardians/[id]/payment - replace the bank details
export const PUT = proxyPut(
  (p) => `/api/guardians/${requirePathSegmentParam(p, "id")}/payment`,
);
