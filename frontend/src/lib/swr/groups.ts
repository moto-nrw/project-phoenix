import { groupService } from "~/lib/api";
import { groupKeys, type GroupFilters } from "./groups-keys";
export const groups = {
  ...groupKeys,
  // A refusal must not poison a shared cache with a successful empty list.
  fetcher: (params: GroupFilters = {}) =>
    groupService.getGroups(params, { strict: true }),
} as const;
