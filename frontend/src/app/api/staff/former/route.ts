import { apiGet } from "~/lib/api-helpers.server";
import type { BackendFormerStaff } from "~/lib/staff-api";
import { createGetHandler } from "~/lib/route-wrapper.server";

// Former employments for the Ehemalige filter of the staff list (#3403). The
// backend serves them on its staff list behind the same permission.
export const GET = createGetHandler(async (_request, token) => {
  const response = await apiGet<{ data: BackendFormerStaff[] | null }>(
    "/api/staff?employment=former",
    token,
  );
  return (response.data ?? []).map((entry) => ({
    id: entry.id.toString(),
    firstName: entry.first_name,
    lastName: entry.last_name,
    startedOn: entry.started_on,
    endedOn: entry.ended_on,
  }));
});
