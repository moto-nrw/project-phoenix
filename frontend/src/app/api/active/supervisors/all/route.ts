import { createGetHandler } from "~/lib/route-wrapper";
import { apiGet } from "~/lib/api-helpers";

export const GET = createGetHandler(async (_request, token, _params) => {
  const response = await apiGet<{ data: unknown }>(
    `/api/active/supervisors/all`,
    token,
  );
  return response.data;
});
