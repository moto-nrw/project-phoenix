// src/app/api/activities/[id]/supervisors/route.ts
import type { NextRequest } from "next/server";
import { createGetHandler, createPostHandler } from "~/lib/route-wrapper";
import { getActivitySupervisors, assignSupervisor } from "~/lib/activity-api";

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

    // Fetch supervisors for the activity
    const supervisors = await getActivitySupervisors(activityId);
    return supervisors;
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

    // Assign the supervisor to the activity
    const success = await assignSupervisor(activityId, supervisorData);

    if (!success) {
      throw new Error("Failed to assign supervisor");
    }

    // If successful, return the updated supervisors list
    const updatedSupervisors = await getActivitySupervisors(activityId);
    return updatedSupervisors;
  },
);
