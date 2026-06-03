import { createGetHandler } from "~/lib/route-wrapper.server";
import { apiGet } from "~/lib/api-helpers.server";

export const GET = createGetHandler(async (request, token, params) => {
  const groupId = params.id as string;
  const response = await apiGet<{ data: unknown }>(
    `/api/groups/${groupId}/substitutions`,
    token,
  );
  return response.data;
});
