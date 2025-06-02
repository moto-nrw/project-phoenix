import { createGetHandler } from "~/lib/route-wrapper";
import { apiGet } from "~/lib/api-client";

export const GET = createGetHandler(async (request, token, params) => {
  const resolvedParams = await params;
  const groupId = resolvedParams?.id;
  
  if (!groupId || typeof groupId !== 'string') {
    throw new Error('Group ID is required');
  }
  
  // Call backend endpoint to get students in the group
  const response = await apiGet(`/api/groups/${groupId}/students`, token);
  
  // Return the data directly (already in the correct format from backend)
  return response;
});