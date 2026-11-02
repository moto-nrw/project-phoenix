import { authFetch } from "~/lib/api-helpers";
import type { CourseWaitlist } from "~/lib/course-waitlist-helpers";

interface Envelope<T> {
  readonly data?: T;
}

/**
 * Liest die Wartelisten der AGs (#4767). Mit `activityGroupId` nur die eine
 * AG. Fehler laufen als ApiError zum Aufrufer, der sie über den Fehlerweg zeigt.
 */
export async function fetchCourseWaitlists(
  activityGroupId?: string,
): Promise<CourseWaitlist[]> {
  const query = activityGroupId
    ? `?activity_group_id=${encodeURIComponent(activityGroupId)}`
    : "";
  const body = await authFetch<Envelope<CourseWaitlist[]>>(
    `/api/students/offering-change-requests/course-waitlists${query}`,
  );
  return body.data ?? [];
}
