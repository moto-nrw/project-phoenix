import { createGetHandler } from "~/lib/route-wrapper";
// import { apiGet } from "~/lib/api-client";

// interface ActiveSupervision {
//   is_supervising: boolean;
//   room_id?: number;
//   room_name?: string;
// }

/**
 * Check if the current user is supervising an active session
 * Used by the supervision context to show/hide room menu item
 */
export const GET = createGetHandler(async (_request, _token) => {
  try {
    // TODO: Implement backend endpoint for checking current user's supervision status
    // For now, return no supervision
    // Potential implementation: 
    // - Check /api/active/supervisors/staff/{currentUserId}/active
    // - Or create a new endpoint /api/usercontext/supervision
    
    return {
      isSupervising: false,
      roomId: undefined,
      roomName: undefined,
    };
  } catch (error) {
    // If the endpoint doesn't exist or user is not supervising, return false
    console.error("Error fetching supervision status:", error);
    return {
      isSupervising: false,
    };
  }
});