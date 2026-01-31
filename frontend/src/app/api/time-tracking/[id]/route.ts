import type { NextRequest } from "next/server";
import { apiPut } from "~/lib/api-helpers";
import { createPutHandler, isStringParam } from "~/lib/route-wrapper";

interface UpdateSessionRequest {
  status?: "present" | "home_office";
  checkInTime?: string;
  checkOutTime?: string;
  breakMinutes?: number;
  notes?: string;
}

/**
 * PUT /api/time-tracking/{id}
 * Update a work session
 */
export const PUT = createPutHandler<unknown, UpdateSessionRequest>(
  async (
    _request: NextRequest,
    body: UpdateSessionRequest,
    token: string,
    params,
  ) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid session ID");
    }

    const response = await apiPut<{ data: unknown }>(
      `/api/time-tracking/${params.id}`,
      token,
      body,
    );
    return response.data;
  },
);
