import { describe, it, expect } from "vitest";
import type { Session } from "next-auth";
import {
  getSmartRedirectPath,
  HOME_PATH,
  isSchoolPortalHandoffPath,
  useSmartRedirectPath,
} from "./redirect-utils";

// Seit #2180 ist /home die Startseite jeder Rolle im Mitarbeiter-Portal. Die
// frühere Verteilung auf /tagesplan, /students/search, /ogs-groups,
// /active-supervisions und /dashboard ist ersetzt: was eine Person sieht,
// entscheidet die Zusammensetzung der Startseite, nicht der Einstiegspfad.
describe("redirect-utils", () => {
  const createSession = (
    roles: string[],
    permissions: string[] = ["schedules:read"],
  ): Session => ({
    user: {
      id: "1",
      email: "test@example.com",
      roles,
      permissions,
      token: "token",
    },
    expires: "2024-12-31",
  });

  describe("getSmartRedirectPath", () => {
    it("sends caregivers to the start page", () => {
      expect(getSmartRedirectPath(createSession(["user"]))).toBe(HOME_PATH);
    });

    it("sends admins to the start page", () => {
      expect(getSmartRedirectPath(createSession(["admin"]))).toBe(HOME_PATH);
    });

    it("sends a tenant-defined role to the start page", () => {
      // Eine Schule kann sich eigene Rollen anlegen; sie darf keinen eigenen
      // Einstiegspfad brauchen.
      expect(getSmartRedirectPath(createSession(["springer"], []))).toBe(
        HOME_PATH,
      );
    });

    it("hands an existing school-only session to moto schule", () => {
      const result = getSmartRedirectPath(createSession(["lehrkraft"]));
      expect(result).toBe("/school/login");
      expect(isSchoolPortalHandoffPath(result)).toBe(true);
    });

    it("keeps dual-role lehrkraft accounts in the staff portal", () => {
      expect(getSmartRedirectPath(createSession(["lehrkraft", "user"]))).toBe(
        HOME_PATH,
      );
    });

    it("handles a null session", () => {
      expect(getSmartRedirectPath(null)).toBe(HOME_PATH);
    });
  });

  describe("useSmartRedirectPath", () => {
    it("is ready immediately: the target waits on nothing", () => {
      expect(useSmartRedirectPath(createSession(["user"]))).toEqual({
        redirectPath: HOME_PATH,
        isReady: true,
      });
    });

    it("keeps the school-portal handoff", () => {
      expect(useSmartRedirectPath(createSession(["lehrkraft"]))).toEqual({
        redirectPath: "/school/login",
        isReady: true,
      });
    });
  });
});
