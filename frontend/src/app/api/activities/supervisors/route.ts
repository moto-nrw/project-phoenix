// app/api/activities/supervisors/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";
import type { BackendSupervisor } from "~/lib/activity-helpers";
import { mapSupervisorResponse } from "~/lib/activity-helpers";


/**
 * Handler for GET /api/activities/supervisors
 * Returns a list of available supervisors (teachers/staff)
 */
export const GET = createGetHandler(async (request: NextRequest, token: string) => {
  try {
    console.log("Fetching supervisors - start");
    
    // Try fetching from the backend activities API endpoint first
    try {
      const response = await apiGet<any>('/api/activities/supervisors/available', token);
      console.log("Activities supervisors API response:", response);
      
      // Handle response structure with more flexible error checking
      if (response) {
        // If response has a data property that is an array
        if (response.data && Array.isArray(response.data)) {
          const mapped = response.data.map(mapSupervisorResponse);
          console.log("Mapped supervisors:", mapped);
          return mapped;
        } 
        // If response itself is an array
        else if (Array.isArray(response)) {
          const mapped = response.map(mapSupervisorResponse);
          console.log("Mapped supervisors (direct array):", mapped);
          return mapped;
        }
      }
    } catch (activityApiError) {
      console.error('Error fetching from activities supervisors endpoint:', activityApiError);
      // Fall through to try the staff endpoint
    }
    
    // Try fetching from staff endpoint as a fallback
    try {
      console.log("Requesting staff from backend: /api/staff?teachers_only=true");
      const response = await apiGet<any>('/api/staff?teachers_only=true', token);
      console.log("API staff response:", response);
      
      // Handle response structure with more flexible checking
      if (response) {
        // If response has a data property that is an array
        if (response.data && Array.isArray(response.data)) {
          const mapped = response.data.map(supervisor => ({
            id: String(supervisor.id),
            name: supervisor.person ? 
              `${supervisor.person.first_name} ${supervisor.person.last_name}` : 
              `Teacher ${supervisor.id}`
          }));
          console.log("Mapped staff supervisors:", mapped);
          return mapped;
        } 
        // If response itself is an array
        else if (Array.isArray(response)) {
          const mapped = response.map(supervisor => ({
            id: String(supervisor.id),
            name: supervisor.person ? 
              `${supervisor.person.first_name} ${supervisor.person.last_name}` : 
              `Teacher ${supervisor.id}`
          }));
          console.log("Mapped staff supervisors (direct array):", mapped);
          return mapped;
        }
      }
    } catch (staffApiError) {
      console.error('Error fetching from staff endpoint:', staffApiError);
    }
    
    // If all API calls failed, return empty array
    console.log("All API calls failed, returning empty array");
    return [];
  } catch (error) {
    console.error('Error in supervisors API route:', error);
    
    // Return empty array instead of mock data
    return [];
  }
});