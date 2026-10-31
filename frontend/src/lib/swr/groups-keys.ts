import { SWR_FAMILIES } from "./shell-keys";
import { resourceKey } from "./resource-key";
export type GroupFilters = {
  search?: string;
  page?: number;
  pageSize?: number;
};
export const groupKeys = {
  family: SWR_FAMILIES.groups,
  key: (params: GroupFilters = {}) => resourceKey("groups-list-", params),
} as const;
