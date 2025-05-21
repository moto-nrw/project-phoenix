// app/api/activities/supervisors/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";
import type { BackendSupervisor } from "~/lib/activity-helpers";
import { mapSupervisorResponse } from "~/lib/activity-helpers";

// Mock supervisors for testing
const MOCK_SUPERVISORS: BackendSupervisor[] = [
  { 
    id: 1, 
    person: { 
      first_name: "Max", 
      last_name: "Müller" 
    },
    is_teacher: true,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
  { 
    id: 2, 
    person: { 
      first_name: "Anna", 
      last_name: "Schmidt" 
    },
    is_teacher: true,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
  { 
    id: 3, 
    person: { 
      first_name: "Lisa", 
      last_name: "Weber" 
    },
    is_teacher: true,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
  { 
    id: 4, 
    person: { 
      first_name: "Tom", 
      last_name: "Fischer" 
    },
    is_teacher: true,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
  { 
    id: 5, 
    person: { 
      first_name: "Sarah", 
      last_name: "Meyer" 
    },
    is_teacher: true,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
];

/**
 * Handler for GET /api/activities/supervisors
 * Returns a list of available supervisors (teachers/staff)
 */
export const GET = createGetHandler(async (request: NextRequest, token: string) => {
  try {
    // Try getting supervisors from the activities API endpoint first
    try {
      const response = await apiGet<{ status: string; data: BackendSupervisor[] }>('/api/activities/supervisors/available', token);
      
      // Handle response structure
      if (response && response.status === "success" && Array.isArray(response.data)) {
        return response.data.map(mapSupervisorResponse);
      }
    } catch (activityApiError) {
      console.error('Error fetching from activities supervisors endpoint:', activityApiError);
      // Fall through to try the staff endpoint
    }
    
    // Try fetching from staff endpoint with teachers_only as a fallback
    const response = await apiGet<{ status: string; data: BackendSupervisor[] }>('/api/staff?teachers_only=true', token);
    
    // Handle response structure
    if (response && response.status === "success" && Array.isArray(response.data)) {
      return response.data.map(mapSupervisorResponse);
    }
    
    // If we get here, we have a response but it's not in the expected format
    console.error('Unexpected response structure:', response);
    throw new Error('Unexpected response structure from supervisors API');
  } catch (error) {
    console.error('Error fetching supervisors:', error);
    
    // For now, we'll return mock data to ensure frontend doesn't break
    // In the future, this should be removed when API is stable
    console.warn('Falling back to mock supervisors data');
    return MOCK_SUPERVISORS.map(mapSupervisorResponse);
  }
});