import { createGetHandler } from "~/lib/route-wrapper";
import { apiGet } from "~/lib/api-helpers";

export const GET = createGetHandler(async (request, token, _params) => {
  const response = await apiGet<{ data: unknown }>(
    `/api/me/groups/active`,
    token,
  );
  return response.data;
});
