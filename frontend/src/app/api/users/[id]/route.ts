// app/api/users/[id]/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPut, apiDelete } from "~/lib/api-helpers";
import {
  createGetHandler,
  createPutHandler,
  createDeleteHandler,
} from "~/lib/route-wrapper";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "UserDetailRoute" });

/**
 * Type definition for person response from backend
 */
interface BackendPersonResponse {
  id: number;
  first_name: string;
  last_name: string;
  tag_id?: string;
  account_id?: number;
  created_at: string;
  updated_at: string;
}

/**
 * Type definition for person update request
 */
interface PersonUpdateRequest {
  first_name?: string;
  last_name?: string;
  tag_id?: string | null;
  account_id?: number | null;
}

/**
 * Type definition for API response format
 */
interface ApiPersonResponse {
  status: string;
  data: BackendPersonResponse;
}

/**
 * Handler for GET /api/users/[id]
 * Returns a single person by ID
 */
export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;

    if (!id) {
      throw new Error("Person ID is required");
    }

    try {
      // Fetch person from backend API
      const response = await apiGet<ApiPersonResponse>(
        `/api/users/${id}`,
        token,
      );

      // Handle null or undefined response
      if (!response?.data) {
        logger.warn("API returned null response for person", { person_id: id });
        throw new Error("Person not found");
      }

      return response.data;
    } catch (error) {
      logger.error("person fetch failed", {
        person_id: id,
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },
);

/**
 * Handler for PUT /api/users/[id]
 * Updates an existing person
 */
export const PUT = createPutHandler<BackendPersonResponse, PersonUpdateRequest>(
  async (
    _request: NextRequest,
    body: PersonUpdateRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;

    if (!id) {
      throw new Error("Person ID is required");
    }

    // Validate fields if provided
    if (body.first_name?.trim() === "") {
      throw new Error("First name cannot be blank");
    }
    if (body.last_name?.trim() === "") {
      throw new Error("Last name cannot be blank");
    }

    try {
      // Update the person via the API
      const response = await apiPut<BackendPersonResponse>(
        `/api/users/${id}`,
        token,
        body,
      );

      return response;
    } catch (error) {
      // Check for permission errors (403 Forbidden)
      if (error instanceof Error && error.message.includes("403")) {
        logger.error("permission denied when updating person", {
          person_id: id,
          error: error instanceof Error ? error.message : String(error),
        });
        throw new Error(
          "Permission denied: You need the 'users:update' permission to update persons.",
        );
      }

      // Check for validation errors
      if (error instanceof Error && error.message.includes("400")) {
        const errorMessage = error.message;
        logger.error("validation error when updating person", {
          person_id: id,
          error: errorMessage,
        });

        // Extract specific error message if possible
        if (errorMessage.includes("person not found")) {
          throw new Error("Person not found");
        }
      }

      // Re-throw other errors
      throw error;
    }
  },
);

/**
 * Handler for DELETE /api/users/[id]
 * Deletes a person
 */
export const DELETE = createDeleteHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;

    if (!id) {
      throw new Error("Person ID is required");
    }

    try {
      // Delete the person via the API
      await apiDelete(`/api/users/${id}`, token);

      // Return null to indicate success with no content
      return null;
    } catch (error) {
      // Check for permission errors (403 Forbidden)
      if (error instanceof Error && error.message.includes("403")) {
        logger.error("permission denied when deleting person", {
          person_id: id,
          error: error instanceof Error ? error.message : String(error),
        });
        throw new Error(
          "Permission denied: You need the 'users:delete' permission to delete persons.",
        );
      }

      // Check for not found errors
      if (error instanceof Error && error.message.includes("404")) {
        throw new Error("Person not found");
      }

      // Re-throw other errors
      throw error;
    }
  },
);
