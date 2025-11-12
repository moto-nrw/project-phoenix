// lib/active-api.ts
import { activeService } from "./active-service";

/**
 * Export the active service for use throughout the application
 */
export { activeService };

/**
 * Export all types from active-helpers
 */
export * from "./active-helpers";

/**
 * Utility function to check if an active group is currently active
 */
export function isActiveGroupCurrent(endTime?: Date): boolean {
  return !endTime || endTime > new Date();
}

/**
 * Utility function to check if a visit is currently active
 */
export function isVisitActive(checkOutTime?: Date): boolean {
  return !checkOutTime || checkOutTime > new Date();
}

/**
 * Utility function to check if a supervision is currently active
 */
export function isSupervisionActive(endTime?: Date): boolean {
  return !endTime || endTime > new Date();
}

/**
 * Utility function to check if a combined group is currently active
 */
export function isCombinedGroupActive(endTime?: Date): boolean {
  return !endTime || endTime > new Date();
}

/**
 * Utility function to format duration between two dates
 */
export function formatDuration(start: Date, end?: Date): string {
  const endTime = end ?? new Date();
  const duration = endTime.getTime() - start.getTime();

  const hours = Math.floor(duration / (1000 * 60 * 60));
  const minutes = Math.floor((duration % (1000 * 60 * 60)) / (1000 * 60));

  if (hours > 0) {
    return `${hours}h ${minutes}m`;
  }
  return `${minutes}m`;
}

/**
 * Utility function to get the current active visit for a student from a list of visits
 */
export function getCurrentVisit(
  visits: Array<{ checkOutTime?: Date }>,
): (typeof visits)[0] | undefined {
  return visits.find((visit) => isVisitActive(visit.checkOutTime));
}

/**
 * Utility function to get active supervisions from a list
 */
export function getActiveSupervisions(
  supervisions: Array<{ endTime?: Date }>,
): typeof supervisions {
  return supervisions.filter((supervision) =>
    isSupervisionActive(supervision.endTime),
  );
}

/**
 * Utility function to get active groups from a list
 */
export function getActiveGroups(
  groups: Array<{ endTime?: Date }>,
): typeof groups {
  return groups.filter((group) => isActiveGroupCurrent(group.endTime));
}
