import { describe, expect, it } from "vitest";

import { spontaneousActivityWindow } from "./spontaneous-window";

describe("spontaneousActivityWindow", () => {
  it("uses the Berlin calendar day and wall clock", () => {
    expect(
      spontaneousActivityWindow(new Date("2026-01-15T23:30:00.000Z")),
    ).toEqual({
      date: "2026-01-16",
      startTime: "00:30",
      endTime: "01:30",
    });
  });
});
