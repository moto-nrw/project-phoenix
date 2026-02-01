import type { NextRequest } from "next/server";
import { apiPut } from "~/lib/api-helpers";
import { createPutHandler, isStringParam } from "~/lib/route-wrapper";

interface UpdateSessionRequest {
  status?: "present" | "home_office";
  checkInTime?: string;
  checkOutTime?: string;
  breakMinutes?: number;
  notes?: string;
  breaks?: Array<{ id: string; durationMinutes: number }>;
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

    // Convert camelCase to snake_case for backend
    const backendBody: Record<string, unknown> = {
      check_in_time: body.checkInTime,
      check_out_time: body.checkOutTime,
      break_minutes: body.breakMinutes,
      status: body.status,
      notes: body.notes,
    };

    // Convert break IDs from string to int for backend
    if (body.breaks && body.breaks.length > 0) {
      backendBody.breaks = body.breaks.map((b) => ({
        id: parseInt(b.id, 10),
        duration_minutes: b.durationMinutes,
      }));
    }

    const response = await apiPut<{ data: unknown }>(
      `/api/time-tracking/${params.id}`,
      token,
      backendBody,
    );
    return response.data;
  },
);
