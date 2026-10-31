import { staffService } from "~/lib/staff-api";
import type { Teacher } from "~/lib/teacher-api";
import type { CrudService } from "~/lib/database/types";
import { staffKeys, type StaffFilters } from "./staff-keys";

export const staff = {
  ...staffKeys,
  fetcher: (params: StaffFilters = {}, options: { strict?: boolean } = {}) =>
    staffService.getAllStaff(params, options),
  // The UI owns the existing CRUD configuration, including its mapper.
  // Keep that transport without importing a UI module into this library.
  async teachersFetcher(service: Pick<CrudService<Teacher>, "getList">) {
    const response = await service.getList({ page: 1, pageSize: 1000 });
    return Array.isArray(response.data) ? response.data : [];
  },
} as const;
