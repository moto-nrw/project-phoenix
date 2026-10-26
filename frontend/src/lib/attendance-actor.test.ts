import { describe, expect, it } from "vitest";
import { attendanceActorLabel, parseAttendanceActor } from "./attendance-actor";

describe("parseAttendanceActor", () => {
  it("keeps the staff member's name", () => {
    expect(
      parseAttendanceActor({ kind: "staff", name: "Anna Muster" }),
    ).toEqual({ kind: "staff", name: "Anna Muster" });
  });

  it("drops a name on the other kinds", () => {
    expect(parseAttendanceActor({ kind: "device", name: "x" })).toEqual({
      kind: "device",
    });
    expect(parseAttendanceActor({ kind: "former_staff" })).toEqual({
      kind: "former_staff",
    });
    expect(parseAttendanceActor({ kind: "automatic" })).toEqual({
      kind: "automatic",
    });
  });

  it("shows nothing for a missing, unknown or nameless staff actor", () => {
    expect(parseAttendanceActor(undefined)).toBeNull();
    expect(parseAttendanceActor(null)).toBeNull();
    expect(parseAttendanceActor({ kind: "robot" })).toBeNull();
    expect(parseAttendanceActor({ kind: "staff" })).toBeNull();
  });
});

describe("attendanceActorLabel", () => {
  it("names the person, the device, a former staff member or moto itself", () => {
    expect(attendanceActorLabel({ kind: "staff", name: "Anna Muster" })).toBe(
      "Anna Muster",
    );
    expect(attendanceActorLabel({ kind: "device" })).toBe("am Gerät");
    expect(attendanceActorLabel({ kind: "former_staff" })).toBe(
      "Ehemalige Mitarbeitende",
    );
    expect(attendanceActorLabel({ kind: "automatic" })).toBe("automatisch");
    expect(attendanceActorLabel(null)).toBe("");
  });
});
