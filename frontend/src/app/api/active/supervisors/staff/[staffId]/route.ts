// app/api/active/supervisors/staff/[staffId]/route.ts
import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Handler for GET /api/active/supervisors/staff/[staffId]
 * Returns supervisions for a specific staff member
 */
export const GET = proxyGet(
  (p) =>
    `/api/active/supervisors/staff/${requirePathSegmentParam(p, "staffId")}`,
  { raw: true },
);
