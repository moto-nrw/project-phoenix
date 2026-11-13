import type { NextRequest } from "next/server";
import { apiDelete } from "~/lib/api-helpers.server";
import { createDeleteHandler, proxyPut } from "~/lib/route-wrapper.server";
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
 * Removes a shift type. One that planned shifts or series still carry is
 * archived instead; the answer names the outcome (#6845).
 */
export const DELETE = createDeleteHandler(
  async (_request: NextRequest, token: string, params) => {
    const id = requirePathSegmentParam(params, "shiftTypeId");
    const result = await apiDelete<{ data?: { outcome?: string } }>(
      `/api/shift-types/${id}`,
      token,
    );
    return result?.data ?? null;
  },
);
