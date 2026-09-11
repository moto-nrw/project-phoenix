import { createGetHandler } from "~/lib/route-wrapper.server";
import { apiGet } from "~/lib/api-helpers.server";

interface GroupsApiResponse {
  data: Array<{
    id: string;
    name: string;
    room_id?: string;
    room_name?: string;
    via_substitution: boolean;
    is_personal: boolean;
  }>;
}

/** Load the educational groups visible in the tenant portal navigation. */
export const GET = createGetHandler(async (_request, token) => {
  const response = await apiGet<GroupsApiResponse>(
    "/api/students/ogs-group-navigation",
    token,
  );

  return { groups: response.data };
});
