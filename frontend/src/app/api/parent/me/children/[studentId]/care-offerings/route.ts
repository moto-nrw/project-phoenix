import { proxyGet } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Proxy GET /api/parent/me/children/{studentId}/care-offerings → backend.
 * Returns the care offerings the child is booked into for the current care
 * period, its activity-group memberships (current plus starting later), any
 * open change request, and whether the guardian may request a change.
 */
export const GET = proxyGet<unknown>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/care-offerings`,
);
