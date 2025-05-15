// app/api/rooms/[id]/history/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";

// Backend interface for room history entries
export interface BackendRoomHistoryEntry {
  id: number;
  room_id: number;
  date: string; // ISO string date
  group_name: string;
  activity_name?: string;
  supervisor_name?: string;
  student_count: number;
  duration: number; // in minutes
}

/**
 * Type guard to check if parameter exists and is a string
 */
function isStringParam(param: unknown): param is string {
  return typeof param === 'string';
}

/**
 * Handler for GET /api/rooms/[id]/history
 * Returns history of a specific room's usage
 */
export const GET = createGetHandler(async (request: NextRequest, token: string, params) => {
  if (!isStringParam(params.id)) {
    throw new Error('Invalid id parameter');
  }

  // Extract date range query parameters if provided
  const start_date = request.nextUrl.searchParams.get('start_date');
  const end_date = request.nextUrl.searchParams.get('end_date');
  
  // Build query parameters for the API call
  let endpoint = `/api/rooms/${params.id}/history`;
  const queryParams = new URLSearchParams();
  
  if (start_date) {
    queryParams.append('start_date', start_date);
  }
  
  if (end_date) {
    queryParams.append('end_date', end_date);
  }
  
  if (queryParams.toString()) {
    endpoint += `?${queryParams.toString()}`;
  }
  
  // Try to fetch from API
  return await apiGet<BackendRoomHistoryEntry[]>(endpoint, token);
});