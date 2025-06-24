import { createGetHandler } from "~/lib/route-wrapper";
import { apiGet } from "~/lib/api-client";

interface GroupsApiResponse {
  status: string;
  data: Array<{
    id: number;
    name: string;
    created_at: string;
    updated_at: string;
  }>;
  message: string;
}

/**
 * Check if the current user has any educational groups
 * Used by the supervision context to determine menu visibility
 */
export const GET = createGetHandler(async (_request, token) => {
  try {
    const response = await apiGet<GroupsApiResponse>("/api/me/groups", token);
    
    return {
      groups: response.data.data ?? [],
    };
  } catch {
    return {
      groups: [],
    };
  }
});