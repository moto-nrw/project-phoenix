// app/api/active/supervisors/staff/[staffId]/active/route.ts
import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Handler for GET /api/active/supervisors/staff/[staffId]/active
 * Returns active supervisions for a specific staff member.
 * The backend wraps the list in { status, data, message } — proxyGet unwraps it
 * so clients receive the plain array (coreFetch reads response.data once).
 */
export const GET = proxyGet(
  (p) =>
    `/api/active/supervisors/staff/${requirePathSegmentParam(p, "staffId")}/active`,
);
