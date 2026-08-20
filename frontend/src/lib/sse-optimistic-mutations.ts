import type { SSEEvent } from "~/lib/sse-types";

type AttendanceEventType = "student_checkin" | "student_checkout";

const OWN_EVENT_TTL_MS = 5_000;
const pending = new Map<string, number>();

function key(eventType: AttendanceEventType, studentId: string): string {
  return `${eventType}:${studentId}`;
}

export function markOwnAttendanceMutation(
  eventType: AttendanceEventType,
  studentId: string,
): void {
  pending.set(key(eventType, studentId), Date.now() + OWN_EVENT_TTL_MS);
}

export function clearOwnAttendanceMutation(
  eventType: AttendanceEventType,
  studentId: string,
): void {
  pending.delete(key(eventType, studentId));
}

export function isOwnAttendanceEvent(
  eventType: SSEEvent["type"],
  studentId: string | undefined,
): boolean {
  if (!studentId) return false;
  if (eventType !== "student_checkin" && eventType !== "student_checkout") {
    return false;
  }
  const mutationKey = key(eventType, studentId);
  const expiresAt = pending.get(mutationKey);
  if (expiresAt === undefined) return false;
  if (expiresAt <= Date.now()) {
    pending.delete(mutationKey);
    return false;
  }
  pending.delete(mutationKey);
  return true;
}
