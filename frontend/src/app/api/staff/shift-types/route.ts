import { proxyGet, proxyPost } from "~/lib/route-proxy.server";

interface ShiftTypeBody {
  name: string;
  color: string;
  description: string;
  is_active: boolean;
}

/**
 * GET /api/staff/shift-types
 * All shift types (Schichtarten) for the current tenant.
 */
export const GET = proxyGet("/api/shift-types");

/**
 * POST /api/staff/shift-types
 * Create a shift type.
 */
export const POST = proxyPost<unknown, ShiftTypeBody>("/api/shift-types");
