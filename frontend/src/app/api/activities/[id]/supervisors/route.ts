// src/app/api/activities/[id]/supervisors/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers.server";
import {
  createGetHandler,
  createPostHandler,
} from "~/lib/route-wrapper.server";
import {
  mapActivitySupervisorSummariesResponse,
  type BackendActivitySupervisor,
} from "~/lib/activity-helpers";

/**
 * GET handler for retrieving all supervisors assigned to an activity
 * @route GET /api/activities/:id/supervisors
 */
export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    // Extract the activity ID from params
    const activityId = String(params.id);

    if (!activityId) {
      throw new Error("Activity ID is required");
    }

    const response = await apiGet<{ data: BackendActivitySupervisor[] }>(
      `/api/activities/${activityId}/supervisors`,
      token,
    );
    return mapActivitySupervisorSummariesResponse(response.data ?? []);
  },
);

/**
 * POST handler for assigning a new supervisor to an activity
 * @route POST /api/activities/:id/supervisors
 * Request body: { staff_id: string, is_primary?: boolean }
 */
export const POST = createPostHandler(
  async (
    _request: NextRequest,
    body: { staff_id: string; is_primary?: boolean },
    token: string,
    params: Record<string, unknown>,
  ) => {
    // Extract the activity ID from params
    const activityId = String(params.id);

    if (!activityId) {
      throw new Error("Activity ID is required");
    }

    if (!body.staff_id) {
      throw new Error("Staff ID is required");
    }

    // Prepare the data for the backend
    const supervisorData = {
      staff_id: body.staff_id,
      is_primary: body.is_primary,
    };

    await apiPost(`/api/activities/${activityId}/supervisors`, token, {
      staff_id: Number.parseInt(supervisorData.staff_id, 10),
      is_primary: supervisorData.is_primary,
    });

    return { success: true };
  },
);
