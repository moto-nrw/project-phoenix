import { createGetHandler } from "~/lib/route-wrapper";
import { apiGet } from "~/lib/api-helpers";

export const GET = createGetHandler(async (request, token, params) => {
  const groupId = params.id as string;
  const response = await apiGet<{ data: unknown }>(
    `/api/groups/${groupId}/substitutions`,
    token,
  );
  return response.data;
});
