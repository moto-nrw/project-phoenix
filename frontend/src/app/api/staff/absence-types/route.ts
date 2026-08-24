import { proxyGet, proxyPost } from "~/lib/route-proxy.server";

interface AbsenceTypeBody {
  name: string;
}

/**
 * GET /api/staff/absence-types
 * The school's own Abwesenheitsarten (#2403), active and retired.
 */
export const GET = proxyGet("/api/absence-types");

/**
 * POST /api/staff/absence-types
 * Add a school-defined Abwesenheitsart.
 */
export const POST = proxyPost<unknown, AbsenceTypeBody>("/api/absence-types");
