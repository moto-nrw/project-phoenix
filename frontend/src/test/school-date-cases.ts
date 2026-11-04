import { expect, vi } from "vitest";
import { parseISODate, todayISO, toISODate } from "~/lib/date-helpers";
import { setTestClock } from "~/test/clock";

/** Each browser's local date differs from this fixed Berlin school date. */
export function schoolDateCases(day: string) {
  const previous = parseISODate(day);
  previous.setDate(previous.getDate() - 1);
  return [
    ["UTC", `${toISODate(previous)}T23:30:00Z`],
    ["Pacific/Honolulu", `${toISODate(previous)}T23:30:00Z`],
    ["Pacific/Kiritimati", `${day}T12:30:00Z`],
  ] as const;
}

export function setSchoolDateClock(zone: string, instant: string, day: string) {
  vi.stubEnv("TZ", zone);
  setTestClock(instant);
  expect(todayISO()).not.toBe(day);
}
