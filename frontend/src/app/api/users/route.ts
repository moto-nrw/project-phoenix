// app/api/users/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers";
import { createGetHandler, createPostHandler } from "~/lib/route-wrapper";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "UsersRoute" });

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
 * Type definition for person creation request
 */
interface PersonCreateRequest {
  first_name: string;
  last_name: string;
  tag_id?: string | null;
  account_id?: number | null;
}

/**
 * Type definition for API response format
 */
interface ApiPersonResponse {
  status: string;
  data: BackendPersonResponse[];
}

/**
 * Handler for GET /api/users
 * Returns a list of persons, optionally filtered by query parameters
 */
export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    // Build URL with any query parameters
    const queryParams = new URLSearchParams();
    request.nextUrl.searchParams.forEach((value, key) => {
      queryParams.append(key, value);
    });

    const endpoint = `/api/users${queryParams.toString() ? "?" + queryParams.toString() : ""}`;

    try {
      // Fetch persons from backend API
      const response = await apiGet<ApiPersonResponse>(endpoint, token);

      // Handle null or undefined response
      if (!response) {
        logger.warn("API returned null response for persons");
        return [];
      }

      logger.debug("persons API response received");

      // Check if the response is already an array (common pattern)
      if (Array.isArray(response)) {
        return response;
      }

      // Check for nested data structure
      if (response.data && Array.isArray(response.data)) {
        return response.data;
      }

      // If the response doesn't have the expected structure, return an empty array
      logger.warn("persons API response has unexpected structure");
      return [];
    } catch (error) {
      logger.error("persons fetch failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      // Return empty array instead of throwing error
      return [];
    }
  },
);

/**
 * Handler for POST /api/users
 * Creates a new person
 */
export const POST = createPostHandler<
  BackendPersonResponse,
  PersonCreateRequest
>(async (_request: NextRequest, body: PersonCreateRequest, token: string) => {
  // Validate required fields
  if (!body.first_name || body.first_name.trim() === "") {
    throw new Error("Missing required field: first_name cannot be blank");
  }
  if (!body.last_name || body.last_name.trim() === "") {
    throw new Error("Missing required field: last_name cannot be blank");
  }

  try {
    // Create the person via the API
    const response = await apiPost<BackendPersonResponse>(
      "/api/users",
      token,
      body,
    );

    return response;
  } catch (error) {
    // Check for permission errors (403 Forbidden)
    if (error instanceof Error && error.message.includes("403")) {
      logger.error("permission denied when creating person", {
        error: error instanceof Error ? error.message : String(error),
      });
      throw new Error(
        "Permission denied: You need the 'users:create' permission to create persons.",
      );
    }

    // Check for validation errors
    if (error instanceof Error && error.message.includes("400")) {
      const errorMessage = error.message;
      logger.error("validation error when creating person", {
        error: errorMessage,
      });

      // Extract specific error message if possible
      if (errorMessage.includes("first name is required")) {
        throw new Error("First name is required");
      }
      if (errorMessage.includes("last name is required")) {
        throw new Error("Last name is required");
      }
    }

    // Re-throw other errors
    throw error;
  }
});
