// app/api/activities/[id]/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPut, apiDelete } from "~/lib/api-helpers";
import {
  createGetHandler,
  createPutHandler,
  createDeleteHandler,
} from "~/lib/route-wrapper";
import type {
  Activity,
  BackendActivity,
  UpdateActivityRequest,
} from "~/lib/activity-helpers";
import { mapActivityResponse } from "~/lib/activity-helpers";

/**
 * Handler for GET /api/activities/[id]
 * Returns a single activity by ID
 */
export const GET = createGetHandler(
  async (
    request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;

    try {
      const response = await apiGet<
        BackendActivity | { status: string; data: BackendActivity }
      >(`/api/activities/${id}`, token);

      // Handle both response formats (raw object or wrapped in status/data)
      if (response) {
        if (
          "status" in response &&
          response.status === "success" &&
          "data" in response
        ) {
          // Handle wrapped response { status: "success", data: BackendActivity }
          return mapActivityResponse(response.data);
        } else if ("id" in response) {
          // Handle direct response (BackendActivity)
          return mapActivityResponse(response);
        }
      }

      throw new Error("Unexpected response structure");
    } catch (error) {
      // If the error contains a 404 status, return the appropriate error
      if (error instanceof Error && error.message.includes("API error (404)")) {
        throw new Error(`API error (404): Activity with ID ${id} not found`);
      }

      // No more mock data fallback - throw the error to show proper errors
      throw error;
    }
  },
);

/**
 * Handler for PUT /api/activities/[id]
 * Updates an existing activity
 */
export const PUT = createPutHandler<Activity, UpdateActivityRequest>(
  async (
    _request: NextRequest,
    body: UpdateActivityRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;
    const activityId = Number.parseInt(id, 10);

    if (Number.isNaN(activityId)) {
      throw new TypeError("Invalid activity ID");
    }

    // The body already matches the UpdateActivityRequest structure expected by backend
    const response = await apiPut<
      BackendActivity | { status: string; data: BackendActivity },
      UpdateActivityRequest
    >(
      `/api/activities/${id}`,
      token,
      body, // Send the raw UpdateActivityRequest
    );

    // Handle both response formats (raw object or wrapped in status/data)
    if (response) {
      if (
        "status" in response &&
        response.status === "success" &&
        "data" in response
      ) {
        // Handle wrapped response { status: "success", data: BackendActivity }
        return mapActivityResponse(response.data);
      } else if ("id" in response) {
        // Handle direct response (BackendActivity)
        return mapActivityResponse(response);
      }
    }

    throw new Error("Unexpected response structure");
  },
);

/**
 * Handler for DELETE /api/activities/[id]
 * Removes an activity by ID
 */
export const DELETE = createDeleteHandler(
  async (
    request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;
    const activityId = Number.parseInt(id, 10);

    if (Number.isNaN(activityId)) {
      throw new TypeError("Invalid activity ID");
    }

    const response = await apiDelete<{
      status?: string | number;
      success?: boolean;
    }>(`/api/activities/${id}`, token);

    // Backend typically returns no content on successful delete
    if (
      !response ||
      (response.status && response.status === 204) ||
      (response.status && response.status === "204") ||
      response.success
    ) {
      return { success: true };
    }

    // Also handle the case where status might be "success"
    if (response.status && response.status === "success") {
      return { success: true };
    }

    throw new Error("Unexpected response structure");
  },
);
