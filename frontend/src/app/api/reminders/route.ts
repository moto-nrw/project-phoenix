import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

interface BackendRemindersResponse {
  status: string;
  data: unknown;
}

export const GET = createGetHandler(async (_request, token) => {
  const response = await apiGet<BackendRemindersResponse>(
    "/api/reminders",
    token,
  );
  // apiGet returns the full backend envelope { status, data, message }.
  // Return .data so createGetHandler doesn't double-wrap it.
  return response.data;
});
