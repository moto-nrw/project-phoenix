import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * GET /api/staff/[id]/time-tracking/sessions/[sessionId]/edits
 * Admin endpoint: list audit edits for a single work session of a staff
 * member. Mirrors /api/time-tracking/[id]/edits (the MA-self route),
 * but verifies the session belongs to the staff named in the URL
 * instead of the JWT subject.
 */
export const GET = proxyGet(
  (p) =>
    `/api/staff/${requirePathSegmentParam(p)}/time-tracking/sessions/${requirePathSegmentParam(p, "sessionId")}/edits`,
);
