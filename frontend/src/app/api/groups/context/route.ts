import { createGetHandler } from "~/lib/route-wrapper.server";
import { apiGet } from "~/lib/api-helpers.server";

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
      groups: response.data ?? [],
    };
  } catch {
    return {
      groups: [],
    };
  }
});
