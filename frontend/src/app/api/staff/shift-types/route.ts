import { proxyGet, proxyPost } from "~/lib/route-wrapper.server";

interface ShiftTypeBody {
  name: string;
  color: string;
  description: string;
  is_active: boolean;
}

/**
 * GET /api/staff/shift-types
 * The offered shift types (Schichtarten) of the current tenant;
 * `include_archived=true` adds removed ones planned shifts still carry,
 * `with_usage=true` the planned shifts and series per type (#6845).
 */
export const GET = proxyGet("/api/shift-types", {
  forwardQuery: ["include_archived", "with_usage"],
});

/**
 * POST /api/staff/shift-types
 * Create a shift type.
 */
export const POST = proxyPost<unknown, ShiftTypeBody>("/api/shift-types");
