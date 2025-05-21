// app/api/activities/timespans/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";

// Define the backend timeframe interface based on what we see in backend/api/activities/api.go:TimespanResponse
interface BackendTimespan {
  id: number;
  name: string;
  start_time: string;
  end_time: string;
  description?: string;
}

// Frontend timeframe interface
interface Timeframe {
  id: string;
  name: string;
  start_time: string;
  end_time: string;
  description?: string;
}

// Mapping function for timeframes
function mapTimespanResponse(timespan: BackendTimespan): Timeframe {
  return {
    id: String(timespan.id),
    name: timespan.name,
    start_time: timespan.start_time,
    end_time: timespan.end_time,
    description: timespan.description
  };
}

/**
 * Handler for GET /api/activities/timespans
 * Returns a list of available time spans for activities
 */
export const GET = createGetHandler(async (request: NextRequest, token: string) => {
  try {
    const response = await apiGet<{ status: string; data: BackendTimespan[] }>('/api/activities/timespans', token);
    
    // Handle response structure
    if (response && response.status === "success" && Array.isArray(response.data)) {
      return response.data.map(mapTimespanResponse);
    }
    
    // If no data or unexpected structure, handle safely
    if (response && Array.isArray(response.data)) {
      return response.data.map(mapTimespanResponse);
    }
    
    // In case of other unexpected response format
    console.error('Unexpected timespan response format:', response);
    return [];
  } catch (error) {
    console.error('Error fetching timespans:', error);
    // Return empty array for now rather than mock data to ensure users know if data isn't available
    return [];
  }
});