import { describe, it, expect } from "vitest";
import type { Session } from "next-auth";
import {
  hasRole,
  isAdmin,
  isTeacher,
  isAuthenticated,
  getUserDisplayName,
  getUserRolesDisplay,
  requiresReauth,
} from "./auth-utils";

describe("auth-utils", () => {
  describe("hasRole", () => {
    it("should return true when user has the specified role", () => {
      const session: Session = {
        user: {
          id: "1",
          email: "test@example.com",
          roles: ["admin", "teacher"],
          isAdmin: false,
          isTeacher: false,
          token: "token",
        },
        expires: "2024-12-31",
      };

      expect(hasRole(session, "admin")).toBe(true);
      expect(hasRole(session, "teacher")).toBe(true);
    });

    it("should return false when user does not have the specified role", () => {
      const session: Session = {
        user: {
          id: "1",
          email: "test@example.com",
          roles: ["teacher"],
          isAdmin: false,
          isTeacher: false,
          token: "token",
        },
        expires: "2024-12-31",
      };

      expect(hasRole(session, "admin")).toBe(false);
      expect(hasRole(session, "moderator")).toBe(false);
    });

    it("should return false when session is null", () => {
      expect(hasRole(null, "admin")).toBe(false);
    });

    it("should return false when user is undefined", () => {
      const session = { expires: "2024-12-31" } as Session;
      expect(hasRole(session, "admin")).toBe(false);
    });

    it("should return false when roles array is undefined", () => {
      const session: Session = {
        user: {
          id: "1",
          email: "test@example.com",
          isAdmin: false,
          isTeacher: false,
          token: "token",
        },
        expires: "2024-12-31",
      };

      expect(hasRole(session, "admin")).toBe(false);
    });

    it("should return false when roles array is empty", () => {
      const session: Session = {
        user: {
          id: "1",
          email: "test@example.com",
          roles: [],
          isAdmin: false,
          isTeacher: false,
          token: "token",
        },
        expires: "2024-12-31",
      };

      expect(hasRole(session, "admin")).toBe(false);
    });
  });

  describe("isAdmin", () => {
    it("should return true when user is admin", () => {
      const session: Session = {
        user: {
          id: "1",
          email: "admin@example.com",
          isAdmin: true,
          isTeacher: false,
          token: "token",
        },
        expires: "2024-12-31",
      };

      expect(isAdmin(session)).toBe(true);
    });

    it("should return false when user is not admin", () => {
      const session: Session = {
        user: {
          id: "1",
          email: "teacher@example.com",
          isAdmin: false,
          isTeacher: true,
          token: "token",
        },
        expires: "2024-12-31",
      };

      expect(isAdmin(session)).toBe(false);
    });

    it("should return false when session is null", () => {
      expect(isAdmin(null)).toBe(false);
    });

    it("should return false when user is undefined", () => {
      const session = { expires: "2024-12-31" } as Session;
      expect(isAdmin(session)).toBe(false);
    });

    it("should return false when isAdmin is undefined", () => {
      const session: Session = {
        user: {
          id: "1",
          email: "test@example.com",
          isTeacher: false,
          token: "token",
        },
        expires: "2024-12-31",
      };

      expect(isAdmin(session)).toBe(false);
    });
  });

  describe("isTeacher", () => {
    it("should return true when user is teacher", () => {
      const session: Session = {
        user: {
          id: "1",
          email: "teacher@example.com",
          isAdmin: false,
          isTeacher: true,
          token: "token",
        },
        expires: "2024-12-31",
      };

      expect(isTeacher(session)).toBe(true);
    });

    it("should return false when user is not teacher", () => {
      const session: Session = {
        user: {
          id: "1",
          email: "admin@example.com",
          isAdmin: true,
          isTeacher: false,
          token: "token",
        },
        expires: "2024-12-31",
      };

      expect(isTeacher(session)).toBe(false);
    });

    it("should return false when session is null", () => {
      expect(isTeacher(null)).toBe(false);
    });

    it("should return false when user is undefined", () => {
      const session = { expires: "2024-12-31" } as Session;
      expect(isTeacher(session)).toBe(false);
    });

    it("should return false when isTeacher is undefined", () => {
      const session: Session = {
        user: {
          id: "1",
          email: "test@example.com",
          isAdmin: false,
          token: "token",
        },
        expires: "2024-12-31",
      };

      expect(isTeacher(session)).toBe(false);
    });
  });

  describe("isAuthenticated", () => {
    it("should return true when user has a token", () => {
      const session: Session = {
        user: {
          id: "1",
          email: "test@example.com",
          isAdmin: false,
          isTeacher: false,
          token: "valid-token-123",
        },
        expires: "2024-12-31",
      };

      expect(isAuthenticated(session)).toBe(true);
    });

    it("should return false when session is null", () => {
      expect(isAuthenticated(null)).toBe(false);
    });

    it("should return false when user is undefined", () => {
      const session = { expires: "2024-12-31" } as Session;
      expect(isAuthenticated(session)).toBe(false);
    });

    it("should return false when token is undefined", () => {
      const session: Session = {
        user: {
          id: "1",
          email: "test@example.com",
          isAdmin: false,
          isTeacher: false,
        },
        expires: "2024-12-31",
      };

      expect(isAuthenticated(session)).toBe(false);
    });

    it("should return false when token is empty string", () => {
      const session: Session = {
        user: {
          id: "1",
          email: "test@example.com",
          isAdmin: false,
          isTeacher: false,
          token: "",
        },
        expires: "2024-12-31",
      };

      expect(isAuthenticated(session)).toBe(false);
    });
  });

  describe("getUserDisplayName", () => {
    it("should return firstName when available", () => {
      const session: Session = {
        user: {
          id: "1",
          email: "test@example.com",
          firstName: "John",
          name: "John Doe",
          isAdmin: false,
          isTeacher: false,
          token: "token",
        },
        expires: "2024-12-31",
      };

      expect(getUserDisplayName(session)).toBe("John");
    });

    it("should return name when firstName is not available", () => {
      const session: Session = {
        user: {
          id: "1",
          email: "test@example.com",
          name: "John Doe",
          isAdmin: false,
          isTeacher: false,
          token: "token",
        },
        expires: "2024-12-31",
      };

      expect(getUserDisplayName(session)).toBe("John Doe");
    });

    it("should return email when firstName and name are not available", () => {
      const session: Session = {
        user: {
          id: "1",
          email: "test@example.com",
          isAdmin: false,
          isTeacher: false,
          token: "token",
        },
        expires: "2024-12-31",
      };

      expect(getUserDisplayName(session)).toBe("test@example.com");
    });

    it('should return "User" when session is null', () => {
      expect(getUserDisplayName(null)).toBe("User");
    });

    it('should return "User" when user is undefined', () => {
      const session = { expires: "2024-12-31" } as Session;
      expect(getUserDisplayName(session)).toBe("User");
    });

    it('should return "User" when all name fields are null', () => {
      const session: Session = {
        user: {
          id: "1",
          email: undefined,
          name: undefined,
          firstName: undefined,
          isAdmin: false,
          isTeacher: false,
          token: "token",
        },
        expires: "2024-12-31",
      };

      expect(getUserDisplayName(session)).toBe("User");
    });

    it("should prefer firstName over name and email", () => {
      const session: Session = {
        user: {
          id: "1",
          email: "test@example.com",
          firstName: "John",
          name: "John Doe",
          isAdmin: false,
          isTeacher: false,
          token: "token",
        },
        expires: "2024-12-31",
      };

      expect(getUserDisplayName(session)).toBe("John");
    });
  });

  describe("getUserRolesDisplay", () => {
    it("should return comma-separated roles when roles exist", () => {
      const session: Session = {
        user: {
          id: "1",
          email: "test@example.com",
          roles: ["admin", "teacher", "moderator"],
          isAdmin: true,
          isTeacher: true,
          token: "token",
        },
        expires: "2024-12-31",
      };

      expect(getUserRolesDisplay(session)).toBe("admin, teacher, moderator");
    });

    it("should return single role when only one role exists", () => {
      const session: Session = {
        user: {
          id: "1",
          email: "test@example.com",
          roles: ["teacher"],
          isAdmin: false,
          isTeacher: true,
          token: "token",
        },
        expires: "2024-12-31",
      };

      expect(getUserRolesDisplay(session)).toBe("teacher");
    });

    it('should return "No roles" when session is null', () => {
      expect(getUserRolesDisplay(null)).toBe("No roles");
    });

    it('should return "No roles" when user is undefined', () => {
      const session = { expires: "2024-12-31" } as Session;
      expect(getUserRolesDisplay(session)).toBe("No roles");
    });

    it('should return "No roles" when roles is undefined', () => {
      const session: Session = {
        user: {
          id: "1",
          email: "test@example.com",
          isAdmin: false,
          isTeacher: false,
          token: "token",
        },
        expires: "2024-12-31",
      };

      expect(getUserRolesDisplay(session)).toBe("No roles");
    });

    it('should return "No roles" when roles array is empty', () => {
      const session: Session = {
        user: {
          id: "1",
          email: "test@example.com",
          roles: [],
          isAdmin: false,
          isTeacher: false,
          token: "token",
        },
        expires: "2024-12-31",
      };

      expect(getUserRolesDisplay(session)).toBe("No roles");
    });

    it("should handle roles with special characters", () => {
      const session: Session = {
        user: {
          id: "1",
          email: "test@example.com",
          roles: ["super-admin", "teacher_level_2"],
          isAdmin: true,
          isTeacher: true,
          token: "token",
        },
        expires: "2024-12-31",
      };

      expect(getUserRolesDisplay(session)).toBe("super-admin, teacher_level_2");
    });
  });

  describe("requiresReauth", () => {
    it("should return true when error is RefreshTokenExpired", () => {
      const session: Session = {
        user: {
          id: "1",
          email: "test@example.com",
          isAdmin: false,
          isTeacher: false,
          token: "token",
        },
        expires: "2024-12-31",
        error: "RefreshTokenExpired",
      };

      expect(requiresReauth(session)).toBe(true);
    });

    it("should return false when error is RefreshTokenError", () => {
      const session: Session = {
        user: {
          id: "1",
          email: "test@example.com",
          isAdmin: false,
          isTeacher: false,
          token: "token",
        },
        expires: "2024-12-31",
        error: "RefreshTokenError",
      };

      expect(requiresReauth(session)).toBe(false);
    });

    it("should return false when session is null", () => {
      expect(requiresReauth(null)).toBe(false);
    });

    it("should return false when error is undefined", () => {
      const session: Session = {
        user: {
          id: "1",
          email: "test@example.com",
          isAdmin: false,
          isTeacher: false,
          token: "token",
        },
        expires: "2024-12-31",
      };

      expect(requiresReauth(session)).toBe(false);
    });

    it("should only match exact RefreshTokenExpired string", () => {
      // Test with RefreshTokenError (the other valid error type)
      const session: Session = {
        user: {
          id: "1",
          email: "test@example.com",
          isAdmin: false,
          isTeacher: false,
          token: "token",
        },
        expires: "2024-12-31",
        error: "RefreshTokenError",
      };

      expect(requiresReauth(session)).toBe(false);
    });
  });
});
