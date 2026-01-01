// app/api/activities/[id]/schedules/[scheduleId]/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPut, apiDelete } from "~/lib/api-helpers";
import {
  createGetHandler,
  createPutHandler,
  createDeleteHandler,
} from "~/lib/route-wrapper";
import type { BackendActivitySchedule } from "~/lib/activity-helpers";
import { mapActivityScheduleResponse } from "~/lib/activity-helpers";

/**
 * Handler for GET /api/activities/[id]/schedules/[scheduleId]
 * Returns a specific schedule by ID
 */
export const GET = createGetHandler(
  async (
    request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;
    const scheduleId = params.scheduleId as string;
    const endpoint = `/api/activities/${id}/schedules/${scheduleId}`;

    try {
      const response = await apiGet<
        | { data: BackendActivitySchedule; status: string }
        | BackendActivitySchedule
      >(endpoint, token);

      // Handle response structure
      if (
        response &&
        "status" in response &&
        response.status === "success" &&
        "data" in response
      ) {
        // Handle wrapped response { status: "success", data: BackendActivitySchedule }
        return mapActivityScheduleResponse(response.data);
      } else if (response && "id" in response) {
        // Handle direct response (BackendActivitySchedule)
        return mapActivityScheduleResponse(response);
      }

      // If we get here, we have a response but it's not in the expected format
      throw new Error(
        `Unexpected response structure from schedule API for activity ${id}, schedule ${scheduleId}`,
      );
    } catch (error) {
      // Properly propagate the error for handling in the service layer
      throw new Error(
        JSON.stringify({
          status: 500,
          message: `Failed to fetch schedule ${scheduleId} for activity ${id}: ${error instanceof Error ? error.message : "Unknown error"}`,
          code: "ACTIVITY_SCHEDULE_ERROR",
        }),
      );
    }
  },
);

/**
 * Handler for PUT /api/activities/[id]/schedules/[scheduleId]
 * Updates a specific schedule
 */
export const PUT = createPutHandler(
  async (
    request: NextRequest,
    body: Partial<BackendActivitySchedule>,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;
    const scheduleId = params.scheduleId as string;
    const endpoint = `/api/activities/${id}/schedules/${scheduleId}`;

    try {
      // Prepare data for backend
      const backendData = {
        weekday: body.weekday
          ? Number.parseInt(body.weekday.toString(), 10)
          : undefined,
        timeframe_id: body.timeframe_id,
      };

      // Call the API
      const response = await apiPut<
        | { data: BackendActivitySchedule; status: string }
        | BackendActivitySchedule
      >(endpoint, token, backendData);

      // Handle response structure
      if (
        response &&
        "status" in response &&
        response.status === "success" &&
        "data" in response
      ) {
        // Handle wrapped response { status: "success", data: BackendActivitySchedule }
        return mapActivityScheduleResponse(response.data);
      } else if (response && "id" in response) {
        // Handle direct response (BackendActivitySchedule)
        return mapActivityScheduleResponse(response);
      }

      // If we get here, we have a response but it's not in the expected format
      throw new Error(
        `Unexpected response structure from schedule update API for activity ${id}, schedule ${scheduleId}`,
      );
    } catch (error) {
      // Properly propagate the error for handling in the service layer
      throw new Error(
        JSON.stringify({
          status: 500,
          message: `Failed to update schedule ${scheduleId} for activity ${id}: ${error instanceof Error ? error.message : "Unknown error"}`,
          code: "ACTIVITY_SCHEDULE_UPDATE_ERROR",
        }),
      );
    }
  },
);

/**
 * Handler for DELETE /api/activities/[id]/schedules/[scheduleId]
 * Deletes a specific schedule
 */
export const DELETE = createDeleteHandler(
  async (
    request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;
    const scheduleId = params.scheduleId as string;
    const endpoint = `/api/activities/${id}/schedules/${scheduleId}`;

    try {
      // Call the API
      await apiDelete(endpoint, token);

      // Return success response
      return {
        success: true,
        message: `Schedule ${scheduleId} deleted successfully`,
      };
    } catch (error) {
      // Properly propagate the error for handling in the service layer
      throw new Error(
        JSON.stringify({
          status: 500,
          message: `Failed to delete schedule ${scheduleId} for activity ${id}: ${error instanceof Error ? error.message : "Unknown error"}`,
          code: "ACTIVITY_SCHEDULE_DELETE_ERROR",
        }),
      );
    }
  },
);
