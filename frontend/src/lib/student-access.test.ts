import type { Session } from "next-auth";
import { describe, expect, it } from "vitest";

import { canGrantExternalViewAccess } from "./external-caregiver-access";
import { canReadStudents } from "./student-access";

function session(permissions: string[]): Session {
  return {
    user: { id: "1", token: "t", permissions },
    expires: "2099-01-01",
  } as Session;
}

// ADR 0083: the role „Gast“ opens the child views with students:read only.
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
