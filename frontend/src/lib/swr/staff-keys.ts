import { SWR_FAMILIES } from "./shell-keys";
import type { staffService } from "~/lib/staff-api";
import { resourceKey } from "./resource-key";
export type StaffFilters = NonNullable<
  Parameters<typeof staffService.getAllStaff>[0]
>;
export const staffKeys = {
  family: SWR_FAMILIES.staff,
  key: (params: StaffFilters = {}, options: { strict?: boolean } = {}) =>
    resourceKey("staff-directory-list-", {
      ...params,
      strict: options.strict ?? false,
    }),
  teachersKey: () =>
    resourceKey("staff-directory-teachers-", { page: 1, pageSize: 1000 }),
} as const;
