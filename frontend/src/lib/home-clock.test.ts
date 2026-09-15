import { describe, expect, it } from "vitest";

import {
  blockPhase,
  formatMinutesAhead,
  minutesBetween,
  wallClockMinutes,
} from "./home-clock";

describe("home-clock (#2180)", () => {
  it("rechnet Wanduhrzeiten in Minuten um", () => {
    expect(wallClockMinutes("00:00")).toBe(0);
    expect(wallClockMinutes("10:15")).toBe(615);
    expect(wallClockMinutes("kaputt")).toBe(0);
    expect(minutesBetween("10:00", "11:30")).toBe(90);
    expect(minutesBetween("11:30", "10:00")).toBe(-90);
  });

  // Das Ende ist exklusiv: um 11:00 ist „10:00–11:00" vorbei.
  it("ordnet einen Block relativ zur Uhr ein", () => {
    expect(blockPhase("10:00", "11:00", "09:59")).toBe("upcoming");
    expect(blockPhase("10:00", "11:00", "10:00")).toBe("running");
    expect(blockPhase("10:00", "11:00", "10:59")).toBe("running");
    expect(blockPhase("10:00", "11:00", "11:00")).toBe("past");
  });

  // „in 0 Min" liest sich wie ein Fehler.
  it("formuliert die Zeit bis zum Beginn", () => {
    expect(formatMinutesAhead(0)).toBe("gleich");
    expect(formatMinutesAhead(5)).toBe("in 5 Min");
    expect(formatMinutesAhead(60)).toBe("in 1 Std");
    expect(formatMinutesAhead(80)).toBe("in 1 Std 20 Min");
    expect(formatMinutesAhead(180)).toBe("in 3 Std");
  });
});
