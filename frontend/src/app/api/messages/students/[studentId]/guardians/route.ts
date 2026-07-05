import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Proxy GET /api/messages/students/{studentId}/guardians → backend. Returns
 * the child's guardians who have a parents-portal account and can therefore
 * receive a new message thread.
 */
export const GET = proxyGet(
  (p) =>
    `/api/messages/students/${requirePathSegmentParam(p, "studentId")}/guardians`,
);
