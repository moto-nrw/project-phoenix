// app/api/activities/[id]/schedules/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { ApiResponseError } from "~/lib/api-response-error.server";
import { createGetHandler } from "~/lib/route-wrapper.server";
import type { BackendActivitySchedule } from "~/lib/activity-helpers";
import { mapActivityScheduleResponse } from "~/lib/activity-helpers";

/**
 * Handler for GET /api/activities/[id]/schedules
 * Returns a list of schedules for a specific activity
 */
export const GET = createGetHandler(
  async (
    request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;
    const endpoint = `/api/activities/${encodeURIComponent(id)}/schedules`;

    try {
      const response = await apiGet<
        | { data: BackendActivitySchedule[]; status: string }
        | BackendActivitySchedule[]
      >(endpoint, token);

      // Handle response structure
      if (
        response &&
        "status" in response &&
        response.status === "success" &&
        "data" in response &&
        Array.isArray(response.data)
      ) {
        // Handle wrapped response { status: "success", data: BackendActivitySchedule[] }
        return response.data.map(mapActivityScheduleResponse);
      } else if (Array.isArray(response)) {
        // Handle direct response (BackendActivitySchedule[])
        return response.map(mapActivityScheduleResponse);
      }

      // If we get here, we have a response but it's not in the expected format
      throw new Error(
        `Unexpected response structure from schedules API for activity ${id}`,
      );
    } catch (error) {
      if (error instanceof ApiResponseError) throw error;
      // Properly propagate the error for handling in the service layer
      throw new Error(
        `Failed to fetch schedules for activity ${id}: ${error instanceof Error ? error.message : "Unknown error"}`,
        { cause: error },
      );
    }
  },
);
