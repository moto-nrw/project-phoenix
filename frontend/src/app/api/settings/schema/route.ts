import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";

interface BackendSettingsResponse {
  status: string;
  data: unknown;
}

export const GET = createGetHandler(async (_request, token) => {
  const response = await apiGet<BackendSettingsResponse>(
    "/api/settings/schema",
    token,
  );
  // apiGet returns the full backend response { status, data, message }.
  // Extract .data so createGetHandler doesn't double-wrap it.
  return response.data;
});
