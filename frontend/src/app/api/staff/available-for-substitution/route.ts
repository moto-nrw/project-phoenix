import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";
import type { BackendStaffWithSubstitutionStatus } from "~/lib/substitution-helpers";

/**
 * Handler for GET /api/staff/available-for-substitution
 * Returns staff members available for substitution with their current status
 */
export const GET = createGetHandler(async (request: NextRequest, token: string) => {
  // Build URL with query parameters
  const queryParams = new URLSearchParams();
  
  // Get query parameters
  const date = request.nextUrl.searchParams.get('date');
  const search = request.nextUrl.searchParams.get('search');
  
  if (date) {
    queryParams.append('date', date);
  }
  if (search) {
    queryParams.append('search', search);
  }
  
  const endpoint = `/api/staff/available-for-substitution${queryParams.toString() ? '?' + queryParams.toString() : ''}`;
  
  // Fetch available staff from the API
  const response = await apiGet<BackendStaffWithSubstitutionStatus[]>(endpoint, token);
  
  // Log for debugging
  console.log('Available staff API Response:', response);
  
  // Extract data array from response
  return response?.data || [];
});