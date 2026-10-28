import { resourceKey } from "./resource-key";
export type GroupFilters = {
  search?: string;
  page?: number;
  pageSize?: number;
};
export const groupKeys = {
  family: "groups-",
  key: (params: GroupFilters = {}) => resourceKey("groups-list-", params),
} as const;
