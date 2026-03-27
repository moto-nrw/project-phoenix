import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";

export const GET = createGetHandler(async (_request, token) => {
  return await apiGet("/api/settings/schema", token);
});
