import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

// Abwesenheiten, die noch nicht im Plan stehen (#7021). Backend gates on
// vacation:approve or time_tracking:manage.
export const GET = createGetHandler(async (_request, token) => {
  const response = await apiGet<{ data: unknown }>(
    "/api/staff/absences/plan-gaps",
    token,
  );
  return response.data;
});
