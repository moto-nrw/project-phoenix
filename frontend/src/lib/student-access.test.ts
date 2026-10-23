import type { Session } from "next-auth";
import { describe, expect, it } from "vitest";

import { canGrantExternalViewAccess } from "./external-caregiver-access";
import {
  canCheckInChildren,
  canExportStudents,
  canReadStudents,
} from "./student-access";

function session(permissions: string[]): Session {
  return {
    user: { id: "1", token: "t", permissions },
    expires: "2099-01-01",
  } as Session;
}

// ADR 0091: the role „Gast“ opens the child views with students:read only.
describe("canReadStudents", () => {
  it.each([
    [["users:read"], true],
    [["students:read"], true],
    [["admin:*"], true],
    [["schedules:read"], false],
    [[], false],
  ])("permissions %j → %s", (permissions, expected) => {
    expect(canReadStudents(session(permissions))).toBe(expected);
  });

  it("is false without session", () => {
    expect(canReadStudents(null)).toBe(false);
  });
});

// „Nur ansehen“ grants the role „Gast“; the backend wants users:manage.
describe("canGrantExternalViewAccess", () => {
  it.each([
    [["users:manage"], true],
    [["admin:*"], true],
    [["users:create"], false],
  ])("permissions %j → %s", (permissions, expected) => {
    expect(canGrantExternalViewAccess(session(permissions))).toBe(expected);
  });
});

// The check-in mode posts to school-checkin, gated on users:checkin.
describe("canCheckInChildren", () => {
  it.each([
    [["users:checkin"], true],
    [["admin:*"], true],
    [["students:read", "groups:read"], false],
  ])("permissions %j → %s", (permissions, expected) => {
    expect(canCheckInChildren(session(permissions))).toBe(expected);
  });

  it("keeps the mode for a session without permission claims", () => {
    expect(
      canCheckInChildren({
        user: { id: "1" },
        expires: "2099-01-01",
      } as Session),
    ).toBe(true);
    expect(canCheckInChildren(null)).toBe(false);
  });
});

describe("canExportStudents", () => {
  it.each([
    [["users:read"], true],
    [["students:read"], false],
  ])("permissions %j → %s", (permissions, expected) => {
    expect(canExportStudents(session(permissions))).toBe(expected);
  });
});
