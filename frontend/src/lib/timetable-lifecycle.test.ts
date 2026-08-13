import { describe, expect, it } from "vitest";

import {
  canCompleteInstance,
  canStartPlannedInstance,
  isPlannedStartExpired,
} from "./timetable-lifecycle";

const now = new Date("2026-05-10T13:50:00+02:00");

describe("timetable lifecycle clock", () => {
  it("unlocks start after startAvailableAt and locks it after plan end", () => {
    const instance = {
      canStart: false,
      startAvailableAt: "2026-05-10T13:45:00+02:00",
      startExpiresAt: "2026-05-10T15:00:00+02:00",
    };
    expect(canStartPlannedInstance(instance, now)).toBe(true);
    expect(
      canStartPlannedInstance(instance, new Date("2026-05-10T15:00:00+02:00")),
    ).toBe(false);
  });

  it("treats a block as expired once the planned end is reached", () => {
    expect(
      isPlannedStartExpired(
        "2026-05-10T15:00:00+02:00",
        new Date("2026-05-10T15:00:00+02:00"),
      ),
    ).toBe(true);
    expect(
      isPlannedStartExpired(
        "2026-05-10T15:00:00+02:00",
        new Date("2026-05-10T14:59:00+02:00"),
      ),
    ).toBe(false);
  });

  it("unlocks complete when completeAvailableAt is reached", () => {
    expect(canCompleteInstance(false, "2026-05-10T14:30:00+02:00", now)).toBe(
      false,
    );
    expect(canCompleteInstance(false, "2026-05-10T13:45:00+02:00", now)).toBe(
      true,
    );
    expect(canCompleteInstance(true, "", now)).toBe(true);
  });
});
