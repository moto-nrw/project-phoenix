import { createGetHandler } from "~/lib/route-wrapper";
import { apiGet } from "~/lib/api-client";

/**
 * Check if the current user has any educational groups
 * Used by the supervision context to determine menu visibility
 */
export const GET = createGetHandler(async (_request, token) => {
  try {
    // Fetch user's groups from the usercontext endpoint
    const response = await apiGet("/api/me/groups", token);
    
    console.log("DEBUG: Groups response:", response.data);
    console.log("DEBUG: Groups count:", response.data?.length || 0);
    console.log("DEBUG: Actual groups array:", response.data.data);
    console.log("DEBUG: Actual groups count:", response.data.data?.length || 0);
    
    return {
      groups: response.data.data ?? [],
    };
  } catch (error) {
    // If the endpoint doesn't exist or user has no groups, return empty array
    console.error("Error fetching user groups context:", error);
    return {
      groups: [],
    };
  }
});