// app/api/users/rfid-cards/available/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";

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
export const GET = createGetHandler(async (_request: NextRequest, token: string) => {
  const endpoint = "/api/users/rfid-cards/available";
  
  try {
    // Fetch available RFID cards from backend API
    const response = await apiGet<ApiRFIDCardResponse>(endpoint, token);
    
    // Handle null or undefined response
    if (!response) {
      console.warn("API returned null response for available RFID cards");
      return [];
    }
    
    // Debug output to check the response data
    console.log("API available RFID cards response:", JSON.stringify(response, null, 2));
    
    // Check if the response is already an array (common pattern)
    if (Array.isArray(response)) {
      return response;
    }
    
    // Check for nested data structure (expected pattern based on backend)
    if (response.data && Array.isArray(response.data)) {
      return response.data;
    }
    
    // If the response doesn't have the expected structure, return an empty array
    console.warn("API response does not have the expected structure:", response);
    return [];
  } catch (error) {
    console.error("Error fetching available RFID cards:", error);
    // Return empty array instead of throwing error to avoid UI blocking
    return [];
  }
});