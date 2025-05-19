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
    // Fetch from staff endpoint with teachers_only
    const response = await apiGet<{ status: string; data: BackendSupervisor[] }>('/api/staff?teachers_only=true', token);
    
    // Handle response structure
    if (response && response.status === "success" && Array.isArray(response.data)) {
      if (response.data.length === 0) {
        console.log('No supervisors in database, returning mock data');
        return MOCK_SUPERVISORS.map(mapSupervisorResponse);
      }
      return response.data.map(mapSupervisorResponse);
    }
    
    // If no data or unexpected structure, return mock data
    console.log('Unexpected response structure, returning mock data:', response);
    return MOCK_SUPERVISORS.map(mapSupervisorResponse);
  } catch (error) {
    console.log('Error fetching supervisors, returning mock data:', error);
    return MOCK_SUPERVISORS.map(mapSupervisorResponse);
  }
});