// app/api/students/[id]/in-group-room/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";

/**
 * Handler for GET /api/students/[id]/in-group-room
 * Checks if a student is in their educational group's room
 */
export const GET = createGetHandler(async (_request: NextRequest, token: string, params: Record<string, unknown>) => {
  const id = params.id as string;
  
  if (!id) {
    throw new Error('Student ID is required');
  }
  
  try {
    // Fetch room status from backend API
    const response = await apiGet<unknown>(`/api/students/${id}/in-group-room`, token);
    
    // Type guard to check response structure
    if (!response || typeof response !== 'object' || !('data' in response)) {
      throw new Error('Invalid response format');
    }
    
    const typedResponse = response as { data: unknown };
    
    // Return the room status data
    return typedResponse.data;
  } catch (error) {
    console.error("Error fetching student room status:", error);
    throw error;
  }
});