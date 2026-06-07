import { createGetHandler } from "~/lib/route-wrapper.server";
import { apiGet } from "~/lib/api-helpers.server";

export const GET = createGetHandler(async (request, token, _params) => {
  const response = await apiGet<{ data: unknown }>(`/api/me`, token);
  return response.data;
});
