import { createGetHandler } from "~/lib/route-wrapper.server";
import { apiGet } from "~/lib/api-helpers.server";

export const GET = createGetHandler(async (_request, token, _params) => {
  const response = await apiGet<{ data: unknown }>(
    `/api/active/supervisors/all`,
    token,
  );
  return response.data;
});
