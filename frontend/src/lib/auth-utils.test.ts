import { describe, it, expect } from "vitest";
import type { Session } from "next-auth";
import {
  hasRole,
  hasAnyRole,
  hasPermission,
  isAdmin,
  hasEffectiveAdminScope,
  isCaregiver,
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
          token: "token",
        },
        expires: "2024-12-31",
      };

      expect(hasRole(session, "admin")).toBe(false);
    });
  });

  describe("hasAnyRole", () => {
    it("should return true when at least one role matches", () => {
      const session: Session = {
        user: {
          id: "1",
          email: "test@example.com",
          roles: ["admin", "user"],
          token: "token",
        },
        expires: "2024-12-31",
      };

      expect(hasAnyRole(session, ["user", "teacher"])).toBe(true);
      expect(hasAnyRole(session, ["teacher", "moderator"])).toBe(false);
    });
  });

  describe("hasPermission", () => {
    it("should return true when the user holds the permission", () => {
      const session: Session = {
        user: {
          id: "1",
          email: "test@example.com",
          roles: ["user"],
          permissions: ["groups:read", "groups:update"],
          token: "token",
        },
        expires: "2024-12-31",
      };

      expect(hasPermission(session, "groups:read")).toBe(true);
    });

    it("should return false when the user lacks the permission", () => {
      const session: Session = {
        user: {
          id: "1",
          email: "test@example.com",
          roles: ["guest"],
          permissions: ["users:read", "rooms:read"],
          token: "token",
        },
        expires: "2024-12-31",
      };

      expect(hasPermission(session, "groups:read")).toBe(false);
    });

    it("should return false when permissions are absent or session is null", () => {
      const session = {
        user: { id: "1", email: "test@example.com", token: "token" },
        expires: "2024-12-31",
      } as Session;

      expect(hasPermission(session, "groups:read")).toBe(false);
      expect(hasPermission(null, "groups:read")).toBe(false);
    });

    it("should allow an empty required permission even without a session", () => {
      expect(hasPermission(null, "")).toBe(true);
    });

    // The backend authorizer (backend/auth/authorize/permission.go) grants a
    // permission via several wildcard forms. hasPermission must honour the
    // same ones, or a legitimately-permitted user is wrongly treated as
    // lacking access on the client.
    const wildcardSession = (perms: string[]): Session => ({
      user: {
        id: "1",
        email: "test@example.com",
        permissions: perms,
        token: "token",
      },
      expires: "2024-12-31",
    });

    it("should grant via the admin wildcards admin:* and *:*", () => {
      expect(hasPermission(wildcardSession(["admin:*"]), "groups:read")).toBe(
        true,
      );
      expect(hasPermission(wildcardSession(["*:*"]), "groups:read")).toBe(true);
    });

    // Resource-level wildcards (groups:*, group*) are NOT granted today: the
    // permission catalog is a fixed set of literal resource:action strings plus
    // admin:* / *:*, and AssignPermissionToRole takes a permission ID (FK), so
    // no role can hold a string like groups:*. These cases therefore can't fire
    // in production right now. We keep covering them on purpose: hasPermission
    // mirrors the backend authorizer (backend/auth/authorize/permission.go), and
    // the day a resource wildcard is ever seeded the gate must already honour it
    // — otherwise a legitimately-permitted user is wrongly locked out (the exact
    // failure mode of issue #846, just reintroduced).
    it("should grant via a resource action-wildcard (groups:*)", () => {
      expect(hasPermission(wildcardSession(["groups:*"]), "groups:read")).toBe(
        true,
      );
    });

    it("should grant via a prefix wildcard on a component (group*)", () => {
      expect(
        hasPermission(wildcardSession(["group*:read"]), "groups:read"),
      ).toBe(true);
    });

    it("should not grant when a different resource wildcard is held", () => {
      expect(hasPermission(wildcardSession(["users:*"]), "groups:read")).toBe(
        false,
      );
    });

    it("should reject malformed required permissions and entries", () => {
      // Required string without a single resource:action split never matches.
      expect(hasPermission(wildcardSession(["groups:read"]), "groups")).toBe(
        false,
      );
      // A malformed entry in the list is ignored, not matched.
      expect(hasPermission(wildcardSession(["groups"]), "groups:read")).toBe(
        false,
      );
    });
  });

  describe("isAdmin", () => {
    it("should return true when user is admin", () => {
      const session: Session = {
        user: {
          id: "1",
          email: "admin@example.com",
          roles: ["admin"],
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
          roles: ["user"],
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

    it("should return false when admin role is missing", () => {
      const session: Session = {
        user: {
          id: "1",
          email: "test@example.com",
          roles: ["user"],
          token: "token",
        },
        expires: "2024-12-31",
      };

      expect(isAdmin(session)).toBe(false);
    });
  });

  describe("hasEffectiveAdminScope", () => {
    it.each(["admin:*", "*:*"])(
      "recognizes the %s wildcard without an admin role",
      (permission) => {
        expect(
          hasEffectiveAdminScope({
            user: {
              id: "1",
              email: "admin@example.com",
              permissions: [permission],
              token: "token",
            },
            expires: "2024-12-31",
          }),
        ).toBe(true);
      },
    );

    it("does not treat a resource-specific wildcard as admin scope", () => {
      expect(
        hasEffectiveAdminScope({
          user: {
            id: "1",
            email: "user@example.com",
            permissions: ["groups:*"],
            token: "token",
          },
          expires: "2024-12-31",
        }),
      ).toBe(false);
    });
  });

  describe("isCaregiver", () => {
    it('should return true when user has the "user" role', () => {
      const session: Session = {
        user: {
          id: "1",
          email: "caregiver@example.com",
          roles: ["user"],
          token: "token",
        },
        expires: "2024-12-31",
      };

      expect(isCaregiver(session)).toBe(true);
    });

    it('should return true when user has the "teacher" role', () => {
      const session: Session = {
        user: {
          id: "1",
          email: "teacher@example.com",
          roles: ["teacher"],
          token: "token",
        },
        expires: "2024-12-31",
      };

      expect(isCaregiver(session)).toBe(true);
    });

    it("should return false when caregiver role is missing", () => {
      const session: Session = {
        user: {
          id: "1",
          email: "admin@example.com",
          roles: ["admin"],
          token: "token",
        },
        expires: "2024-12-31",
      };

      expect(isCaregiver(session)).toBe(false);
    });
  });

  describe("isAuthenticated", () => {
    it("should return true when user has a token", () => {
      const session: Session = {
        user: {
          id: "1",
          email: "test@example.com",
          isAdmin: false,
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
          token: "token",
        },
        expires: "2024-12-31",
        error: "RefreshTokenError",
      };

      expect(requiresReauth(session)).toBe(false);
    });
  });
});
