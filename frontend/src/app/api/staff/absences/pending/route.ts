import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

// Tenant-wide open absence requests (status requested + question) for the
// /staff inbox and the sidebar pending counter (#1419). Backend gates on
// vacation:approve.
export const GET = createGetHandler(async (_request, token) => {
  const response = await apiGet<{ data: unknown }>(
    `/api/staff/absences/pending`,
    token,
  );
  return response.data;
});
