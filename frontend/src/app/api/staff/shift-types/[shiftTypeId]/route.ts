import { proxyDelete, proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface ShiftTypeBody {
  name: string;
  color: string;
  description: string;
  is_active: boolean;
}

/**
 * PUT /api/staff/shift-types/{shiftTypeId}
 * Update a shift type.
 */
export const PUT = proxyPut<unknown, ShiftTypeBody>(
  (p) => `/api/shift-types/${requirePathSegmentParam(p, "shiftTypeId")}`,
);

/**
 * DELETE /api/staff/shift-types/{shiftTypeId}
 * Delete a shift type (shifts referencing it keep the shift, lose the type).
 */
export const DELETE = proxyDelete(
  (p) => `/api/shift-types/${requirePathSegmentParam(p, "shiftTypeId")}`,
);
