import { staffService } from "~/lib/staff-api";
import { staffKeys, type StaffFilters } from "./staff-keys";
import { sessionFetch } from "~/lib/session-cache";
import { apiErrorFromResponse } from "~/lib/api-error";
import { mapTeacherResponse } from "~/lib/teacher-helpers";
export const staff = {
  ...staffKeys,
  fetcher: (params: StaffFilters = {}, options: { strict?: boolean } = {}) =>
    staffService.getAllStaff(params, options),
  async teachersFetcher() {
    const response = await sessionFetch("/api/staff?page=1&page_size=1000");
    if (!response.ok)
      throw await apiErrorFromResponse(response, "staff list failed");
    const body = (await response.json()) as { data: unknown[] };
    return body.data.map(mapTeacherResponse);
  },
} as const;
