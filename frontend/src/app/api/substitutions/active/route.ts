import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";
import type { BackendSubstitution } from "~/lib/substitution-helpers";

/**
 * Handler for GET /api/substitutions/active
 * Returns active substitutions for a specific date
 */
export const GET = createGetHandler(async (request: NextRequest, token: string) => {
  // Build URL with query parameters
  const queryParams = new URLSearchParams();
  
  // Get date parameter from query
  const date = request.nextUrl.searchParams.get('date');
  if (date) {
    queryParams.append('date', date);
  }
  
  const endpoint = `/api/substitutions/active${queryParams.toString() ? '?' + queryParams.toString() : ''}`;
  
  // Fetch active substitutions from the API
  const response = await apiGet<BackendSubstitution[]>(endpoint, token);
  
  // Log for debugging
  console.log('Active substitutions API Response:', response);
  
  // Return the response directly as it's already an array
  return response ?? [];
});