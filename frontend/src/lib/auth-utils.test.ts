/**
 * Tests for auth-utils.ts
 *
 * Verifies auth utility functions for BetterAuth sessions.
 * Tests role checking, authentication status, display names, and re-auth detection.
 */
import { describe, it, expect } from "vitest";
import {
  hasRole,
  isAdmin,
  isTeacher,
  isAuthenticated,
  getUserDisplayName,
  getUserRolesDisplay,
  requiresReauth,
  type BetterAuthSession,
} from "./auth-utils";

describe("auth-utils", () => {
  // Helper to create test sessions
  const createSession = (
    overrides: Partial<BetterAuthSession["user"]> = {},
    sessionOverrides: Partial<BetterAuthSession> = {},
  ): BetterAuthSession => ({
    user: {
      id: "user-123",
      email: "test@example.com",
      name: "Test User",
      ...overrides,
    },
    ...sessionOverrides,
  });

  describe("hasRole", () => {
    it("returns true when user has the specified role", () => {
      const session = createSession({ roles: ["admin", "teacher"] });
      expect(hasRole(session, "admin")).toBe(true);
    });

    it("returns true for any matching role in the array", () => {
      const session = createSession({ roles: ["viewer", "editor", "admin"] });
      expect(hasRole(session, "editor")).toBe(true);
    });

    it("returns false when user does not have the specified role", () => {
      const session = createSession({ roles: ["viewer"] });
      expect(hasRole(session, "admin")).toBe(false);
    });

    it("returns false when roles array is empty", () => {
      const session = createSession({ roles: [] });
      expect(hasRole(session, "admin")).toBe(false);
    });

    it("returns false when roles is undefined", () => {
      const session = createSession({ roles: undefined });
      expect(hasRole(session, "admin")).toBe(false);
    });

    it("returns false when session is null", () => {
      expect(hasRole(null, "admin")).toBe(false);
    });

    it("returns false when session.user is undefined", () => {
      const session = { user: undefined } as unknown as BetterAuthSession;
      expect(hasRole(session, "admin")).toBe(false);
    });

    it("is case-sensitive for role matching", () => {
      const session = createSession({ roles: ["Admin"] });
      expect(hasRole(session, "admin")).toBe(false);
      expect(hasRole(session, "Admin")).toBe(true);
    });
  });

  describe("isAdmin", () => {
    it("returns true when user.isAdmin is true", () => {
      const session = createSession({ isAdmin: true });
      expect(isAdmin(session)).toBe(true);
    });

    it("returns false when user.isAdmin is false", () => {
      const session = createSession({ isAdmin: false });
      expect(isAdmin(session)).toBe(false);
    });

    it("returns false when user.isAdmin is undefined", () => {
      const session = createSession({ isAdmin: undefined });
      expect(isAdmin(session)).toBe(false);
    });

    it("returns false when session is null", () => {
      expect(isAdmin(null)).toBe(false);
    });

    it("returns false when session.user is undefined", () => {
      const session = { user: undefined } as unknown as BetterAuthSession;
      expect(isAdmin(session)).toBe(false);
    });
  });

  describe("isTeacher", () => {
    it("returns true when user.isTeacher is true", () => {
      const session = createSession({ isTeacher: true });
      expect(isTeacher(session)).toBe(true);
    });

    it("returns false when user.isTeacher is false", () => {
      const session = createSession({ isTeacher: false });
      expect(isTeacher(session)).toBe(false);
    });

    it("returns false when user.isTeacher is undefined", () => {
      const session = createSession({ isTeacher: undefined });
      expect(isTeacher(session)).toBe(false);
    });

    it("returns false when session is null", () => {
      expect(isTeacher(null)).toBe(false);
    });

    it("returns false when session.user is undefined", () => {
      const session = { user: undefined } as unknown as BetterAuthSession;
      expect(isTeacher(session)).toBe(false);
    });
  });

  describe("isAuthenticated", () => {
    it("returns true when session has user data", () => {
      const session = createSession();
      expect(isAuthenticated(session)).toBe(true);
    });

    it("returns true even with minimal user data", () => {
      const session = {
        user: { id: "123", email: "test@test.com", name: null },
      };
      expect(isAuthenticated(session)).toBe(true);
    });

    it("returns false when session is null", () => {
      expect(isAuthenticated(null)).toBe(false);
    });

    it("returns false when session.user is undefined", () => {
      const session = { user: undefined } as unknown as BetterAuthSession;
      expect(isAuthenticated(session)).toBe(false);
    });

    it("returns false when session.user is null", () => {
      const session = { user: null } as unknown as BetterAuthSession;
      expect(isAuthenticated(session)).toBe(false);
    });
  });

  describe("getUserDisplayName", () => {
    it("returns firstName when available", () => {
      const session = createSession({
        firstName: "John",
        name: "John Doe",
        email: "john@example.com",
      });
      expect(getUserDisplayName(session)).toBe("John");
    });

    it("returns name when firstName is not available", () => {
      const session = createSession({
        firstName: undefined,
        name: "John Doe",
        email: "john@example.com",
      });
      expect(getUserDisplayName(session)).toBe("John Doe");
    });

    it("returns email when firstName and name are not available", () => {
      const session = createSession({
        firstName: undefined,
        name: null,
        email: "john@example.com",
      });
      expect(getUserDisplayName(session)).toBe("john@example.com");
    });

    it("returns 'User' when all fields are missing", () => {
      const session = {
        user: {
          id: "123",
          email: undefined,
          name: null,
          firstName: undefined,
        },
      } as unknown as BetterAuthSession;
      expect(getUserDisplayName(session)).toBe("User");
    });

    it("returns 'User' when session is null", () => {
      expect(getUserDisplayName(null)).toBe("User");
    });

    it("returns 'User' when session.user is undefined", () => {
      const session = { user: undefined } as unknown as BetterAuthSession;
      expect(getUserDisplayName(session)).toBe("User");
    });

    it("prefers firstName over name even if name is defined", () => {
      const session = createSession({
        firstName: "Johnny",
        name: "John Smith",
      });
      expect(getUserDisplayName(session)).toBe("Johnny");
    });

    it("handles empty string firstName by returning name", () => {
      const session = createSession({
        firstName: "",
        name: "John Doe",
      });
      // Empty string is falsy, so it falls through to name
      expect(getUserDisplayName(session)).toBe("John Doe");
    });
  });

  describe("getUserRolesDisplay", () => {
    it("returns comma-separated roles", () => {
      const session = createSession({ roles: ["admin", "teacher", "viewer"] });
      expect(getUserRolesDisplay(session)).toBe("admin, teacher, viewer");
    });

    it("returns single role without comma", () => {
      const session = createSession({ roles: ["admin"] });
      expect(getUserRolesDisplay(session)).toBe("admin");
    });

    it("returns 'No roles' when roles array is empty", () => {
      const session = createSession({ roles: [] });
      expect(getUserRolesDisplay(session)).toBe("No roles");
    });

    it("returns 'No roles' when roles is undefined", () => {
      const session = createSession({ roles: undefined });
      expect(getUserRolesDisplay(session)).toBe("No roles");
    });

    it("returns 'No roles' when session is null", () => {
      expect(getUserRolesDisplay(null)).toBe("No roles");
    });

    it("returns 'No roles' when session.user is undefined", () => {
      const session = { user: undefined } as unknown as BetterAuthSession;
      expect(getUserRolesDisplay(session)).toBe("No roles");
    });

    it("preserves role casing", () => {
      const session = createSession({ roles: ["Admin", "SuperUser"] });
      expect(getUserRolesDisplay(session)).toBe("Admin, SuperUser");
    });
  });

  describe("requiresReauth", () => {
    it("returns true when error is RefreshTokenExpired", () => {
      const session = createSession({}, { error: "RefreshTokenExpired" });
      expect(requiresReauth(session)).toBe(true);
    });

    it("returns false when error is different", () => {
      const session = createSession({}, { error: "InvalidToken" });
      expect(requiresReauth(session)).toBe(false);
    });

    it("returns false when error is undefined", () => {
      const session = createSession({}, { error: undefined });
      expect(requiresReauth(session)).toBe(false);
    });

    it("returns false when session is null", () => {
      expect(requiresReauth(null)).toBe(false);
    });

    it("returns false for valid session without error", () => {
      const session = createSession();
      expect(requiresReauth(session)).toBe(false);
    });

    it("is case-sensitive for error matching", () => {
      const session = createSession({}, { error: "refreshtokenexpired" });
      expect(requiresReauth(session)).toBe(false);
    });
  });

  describe("type exports", () => {
    it("BetterAuthSession has correct structure", () => {
      // This test validates the interface shape at compile time
      const session: BetterAuthSession = {
        user: {
          id: "123",
          email: "test@example.com",
          name: "Test",
          firstName: "Test",
          roles: ["admin"],
          isAdmin: true,
          isTeacher: false,
        },
        error: undefined,
      };

      expect(session.user.id).toBe("123");
      expect(session.user.email).toBe("test@example.com");
      expect(session.user.name).toBe("Test");
      expect(session.user.firstName).toBe("Test");
      expect(session.user.roles).toEqual(["admin"]);
      expect(session.user.isAdmin).toBe(true);
      expect(session.user.isTeacher).toBe(false);
    });

    it("BetterAuthSession allows null name", () => {
      const session: BetterAuthSession = {
        user: {
          id: "123",
          email: "test@example.com",
          name: null,
        },
      };

      expect(session.user.name).toBeNull();
    });
  });
});
