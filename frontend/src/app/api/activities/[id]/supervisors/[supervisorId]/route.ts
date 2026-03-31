// src/app/api/activities/[id]/supervisors/[supervisorId]/route.ts
import type { NextRequest } from "next/server";
import { apiDelete, apiGet, apiPut } from "~/lib/api-helpers";
import { createPutHandler, createDeleteHandler } from "~/lib/route-wrapper";
import {
  mapActivitySupervisorSummariesResponse,
  type BackendActivitySupervisor,
} from "~/lib/activity-helpers";

/**
 * PUT handler for updating a supervisor's role for an activity (primarily is_primary status)
 * @route PUT /api/activities/:id/supervisors/:supervisorId
 * Request body: { is_primary: boolean }
 */
export const PUT = createPutHandler(
  async (
    _request: NextRequest,
    body: { is_primary: boolean },
    token: string,
    params: Record<string, unknown>,
  ) => {
    // Extract parameters
    const activityId = String(params.id);
    const supervisorId = String(params.supervisorId);

    if (!activityId) {
      throw new Error("Activity ID is required");
    }

    if (!supervisorId) {
      throw new Error("Supervisor ID is required");
    }

    if (body.is_primary === undefined) {
      throw new Error("is_primary parameter is required");
    }

    await apiPut(
      `/api/activities/${activityId}/supervisors/${supervisorId}`,
      token,
      {
        is_primary: body.is_primary,
      },
    );

    const response = await apiGet<{ data: BackendActivitySupervisor[] }>(
      `/api/activities/${activityId}/supervisors`,
      token,
    );
    return mapActivitySupervisorSummariesResponse(response.data ?? []);
  },
);

/**
 * DELETE handler for removing a supervisor from an activity
 * @route DELETE /api/activities/:id/supervisors/:supervisorId
 */
export const DELETE = createDeleteHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    // Extract parameters
    const activityId = String(params.id);
    const supervisorId = String(params.supervisorId);

    if (!activityId) {
      throw new Error("Activity ID is required");
    }

    if (!supervisorId) {
      throw new Error("Supervisor ID is required");
    }

    await apiDelete(
      `/api/activities/${activityId}/supervisors/${supervisorId}`,
      token,
    );

    return { success: true };
  },
);
