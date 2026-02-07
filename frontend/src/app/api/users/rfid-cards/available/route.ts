// app/api/users/rfid-cards/available/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "RfidCardsAvailableRoute" });

/**
 * Type definition for RFID card response from backend
 */
interface BackendRFIDCardResponse {
  TagID: string;
  IsActive: boolean;
  CreatedAt: string;
  UpdatedAt: string;
}

/**
 * Type definition for API response format
 */
interface ApiRFIDCardResponse {
  status: string;
  data: BackendRFIDCardResponse[];
}

/**
 * Handler for GET /api/users/rfid-cards/available
 * Returns a list of available (unassigned) RFID cards
 */
export const GET = createGetHandler(
  async (_request: NextRequest, token: string) => {
    const endpoint = "/api/users/rfid-cards/available";

    try {
      // Fetch available RFID cards from backend API
      const response = await apiGet<ApiRFIDCardResponse>(endpoint, token);

      // Handle null or undefined response
      if (!response) {
        logger.warn("API returned null response for available RFID cards");
        return [];
      }

      logger.debug("available RFID cards response received");

      // Check if the response is already an array (common pattern)
      if (Array.isArray(response)) {
        return response;
      }

      // Check for nested data structure (expected pattern based on backend)
      if (response.data && Array.isArray(response.data)) {
        return response.data;
      }

      // If the response doesn't have the expected structure, return an empty array
      logger.warn("RFID cards API response has unexpected structure");
      return [];
    } catch (error) {
      logger.error("available RFID cards fetch failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      // Return empty array instead of throwing error to avoid UI blocking
      return [];
    }
  },
);
