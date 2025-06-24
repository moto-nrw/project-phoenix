import { createGetHandler } from "~/lib/route-wrapper";
import { apiGet } from "~/lib/api-client";

interface SupervisionResponse {
  is_supervising: boolean;
  room_id?: number;
  room_name?: string;
  group_id?: number;
  group_name?: string;
}

/**
 * Check if the current user is supervising an active session
 * Used by the supervision context to show/hide room menu item
 */
export const GET = createGetHandler(async (_request, token) => {
  try {
    // Check user's supervision status via the usercontext endpoint
    const response = await apiGet<SupervisionResponse>("/api/me/supervision", token);
    
    return {
      isSupervising: response.data.is_supervising ?? false,
      roomId: response.data.room_id?.toString(),
      roomName: response.data.room_name,
      groupId: response.data.group_id?.toString(),
      groupName: response.data.group_name,
    };
  } catch (error) {
    // If the endpoint fails or user is not supervising, return false
    console.error("Error fetching supervision status:", error);
    return {
      isSupervising: false,
    };
  }
});