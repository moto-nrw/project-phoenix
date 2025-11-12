export * from "./theme";
export { default as theme } from "./theme";
export * from "./auth-api";
export * from "./auth-service";
export * from "./auth-helpers";
export * from "./file-upload-wrapper";

// Export active module without conflicting types
export {
  activeService,
  isActiveGroupCurrent,
  isVisitActive,
  isSupervisionActive,
  isCombinedGroupActive,
  formatDuration,
} from "./active-api";

export {
  type ActiveGroup,
  type Visit,
  type CombinedGroup,
  type GroupMapping,
  type BackendActiveGroup,
  type BackendVisit,
  type BackendCombinedGroup,
  type BackendGroupMapping,
  mapActiveGroupResponse,
  mapVisitResponse,
  mapCombinedGroupResponse,
} from "./active-helpers";

// Export activity module without conflicting types
export {
  fetchActivities,
  getActivity,
  createActivity,
  updateActivity,
  deleteActivity,
  getCategories,
  getSupervisors,
} from "./activity-api";

export { activityService } from "./activity-service";

export type {
  Activity,
  ActivityCategory,
  CreateActivityRequest,
  UpdateActivityRequest,
  ActivityFilter,
} from "./activity-helpers";

export {
  mapActivityResponse,
  mapActivityCategoryResponse,
  prepareActivityForBackend,
  formatActivityTimes,
  formatParticipantStatus,
} from "./activity-helpers";
