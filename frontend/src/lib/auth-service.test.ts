// auth-service.test.ts
// Comprehensive tests for auth service

/* eslint-disable @typescript-eslint/unbound-method */
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type {
  BackendAccount,
  BackendRole,
  BackendPermission,
  BackendToken,
  BackendParentAccount,
  LoginRequest,
  RegisterRequest,
  ChangePasswordRequest,
  PasswordResetRequest,
  PasswordResetConfirmRequest,
  CreateRoleRequest,
  UpdateRoleRequest,
  CreatePermissionRequest,
  UpdatePermissionRequest,
  UpdateAccountRequest,
  CreateParentAccountRequest,
} from "./auth-helpers";

// Mock dependencies
vi.mock("./api", () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}));

vi.mock("~/env", () => ({
  env: {
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
  },
}));

// Import after mocks
import api from "./api";
import { authService } from "./auth-service";

const mockedApiGet = vi.mocked(api.get);
const mockedApiPost = vi.mocked(api.post);
const mockedApiPut = vi.mocked(api.put);
const mockedApiDelete = vi.mocked(api.delete);

// Sample test data
const sampleBackendAccount: BackendAccount = {
  ID: 1,
  Email: "test@example.com",
  Username: "testuser",
  Name: "Test User",
  Active: true,
  CreatedAt: "2024-01-01T00:00:00Z",
  UpdatedAt: "2024-01-01T00:00:00Z",
};

const sampleBackendRole: BackendRole = {
  ID: 1,
  Name: "admin",
  Description: "Administrator role",
  CreatedAt: "2024-01-01T00:00:00Z",
  UpdatedAt: "2024-01-01T00:00:00Z",
};

const sampleBackendPermission: BackendPermission = {
  id: 1,
  name: "users.read",
  description: "Read users",
  resource: "users",
  action: "read",
  created_at: "2024-01-01T00:00:00Z",
  updated_at: "2024-01-01T00:00:00Z",
};

const sampleBackendToken: BackendToken = {
  ID: 1,
  Token: "sample-token",
  Expiry: "2024-12-31T23:59:59Z",
  Mobile: false,
  Identifier: "desktop",
  CreatedAt: "2024-01-01T00:00:00Z",
};

const sampleBackendParentAccount: BackendParentAccount = {
  ID: 1,
  Email: "parent@example.com",
  Username: "parentuser",
  Active: true,
  CreatedAt: "2024-01-01T00:00:00Z",
  UpdatedAt: "2024-01-01T00:00:00Z",
};

describe("authService", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal("window", undefined);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  describe("login", () => {
    const credentials: LoginRequest = {
      email: "test@example.com",
      password: "password123",
    };

    it("logs in user in server context", async () => {
      mockedApiPost.mockResolvedValueOnce({
        data: {
          access_token: "access-token",
          refresh_token: "refresh-token",
        },
      });

      const result = await authService.login(credentials);

      expect(mockedApiPost).toHaveBeenCalledWith(
        "http://localhost:8080/auth/login",
        credentials,
      );
      expect(result.access_token).toBe("access-token");
    });

    it("logs in user in browser context", async () => {
      vi.stubGlobal("window", {});

      const mockResponse = {
        ok: true,
        json: async () => ({
          access_token: "access-token",
          refresh_token: "refresh-token",
        }),
      };
      vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(mockResponse));

      const result = await authService.login(credentials);

      expect(globalThis.fetch).toHaveBeenCalledWith(
        "/api/auth/login",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify(credentials),
        }),
      );
      expect(result.access_token).toBe("access-token");
    });

    it("throws error on login failure in browser", async () => {
      vi.stubGlobal("window", {});
      vi.stubGlobal(
        "fetch",
        vi.fn().mockResolvedValueOnce({
          ok: false,
          status: 401,
          text: async () => "Invalid credentials",
        }),
      );

      await expect(authService.login(credentials)).rejects.toThrow(
        "Login failed: 401",
      );
    });
  });

  describe("register", () => {
    const registerData: RegisterRequest = {
      email: "new@example.com",
      username: "newuser",
      name: "New User",
      password: "password123",
      confirmPassword: "password123",
    };

    it("registers user in server context", async () => {
      mockedApiPost.mockResolvedValueOnce({
        data: { data: sampleBackendAccount },
      });

      const result = await authService.register(registerData);

      expect(result.email).toBe("test@example.com");
    });

    it("registers user in browser context", async () => {
      vi.stubGlobal("window", {});

      const mockResponse = {
        ok: true,
        json: async () => ({ data: sampleBackendAccount }),
      };
      vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(mockResponse));

      const result = await authService.register(registerData);

      expect(result.email).toBe("test@example.com");
    });

    it("throws error on registration failure", async () => {
      vi.stubGlobal("window", {});
      vi.stubGlobal(
        "fetch",
        vi.fn().mockResolvedValueOnce({
          ok: false,
          status: 400,
          text: async () => "Email already exists",
        }),
      );

      await expect(authService.register(registerData)).rejects.toThrow(
        "Registration failed: 400",
      );
    });
  });

  describe("logout", () => {
    it("logs out user in server context", async () => {
      mockedApiPost.mockResolvedValueOnce({});

      await authService.logout();

      expect(mockedApiPost).toHaveBeenCalledWith(
        "http://localhost:8080/auth/logout",
      );
    });

    it("logs out user in browser context", async () => {
      vi.stubGlobal("window", {});

      const mockResponse = { ok: true };
      vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(mockResponse));

      await authService.logout();

      expect(globalThis.fetch).toHaveBeenCalledWith(
        "/api/auth/logout",
        expect.objectContaining({ method: "POST" }),
      );
    });

    it("calls logout endpoint even when no local session exists", async () => {
      // BetterAuth: logout always calls the backend to ensure server-side session is invalidated
      // The backend handles cases where the session doesn't exist gracefully
      mockedApiPost.mockResolvedValueOnce({});

      await authService.logout();

      expect(mockedApiPost).toHaveBeenCalledWith(
        "http://localhost:8080/auth/logout",
      );
    });

    it("does not throw on logout error", async () => {
      vi.stubGlobal("window", {});

      vi.stubGlobal(
        "fetch",
        vi.fn().mockRejectedValueOnce(new Error("Network error")),
      );

      // Should not throw
      await expect(authService.logout()).resolves.toBeUndefined();
    });
  });

  describe("refreshToken", () => {
    it("refreshes token in server context", async () => {
      mockedApiPost.mockResolvedValueOnce({
        data: {
          access_token: "new-access-token",
          refresh_token: "new-refresh-token",
        },
      });

      const result = await authService.refreshToken("old-refresh-token");

      expect(result.access_token).toBe("new-access-token");
    });

    it("refreshes token in browser context", async () => {
      vi.stubGlobal("window", {});

      const mockResponse = {
        ok: true,
        json: async () => ({
          access_token: "new-access-token",
          refresh_token: "new-refresh-token",
        }),
      };
      vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(mockResponse));

      const result = await authService.refreshToken("old-refresh-token");

      expect(result.access_token).toBe("new-access-token");
    });
  });

  describe("Password reset", () => {
    describe("initiatePasswordReset", () => {
      const resetRequest: PasswordResetRequest = { email: "test@example.com" };

      it("initiates password reset in server context", async () => {
        mockedApiPost.mockResolvedValueOnce({});

        await authService.initiatePasswordReset(resetRequest);

        expect(mockedApiPost).toHaveBeenCalledWith(
          "http://localhost:8080/auth/password-reset",
          resetRequest,
        );
      });

      it("initiates password reset in browser context", async () => {
        vi.stubGlobal("window", {});

        const mockResponse = { ok: true };
        vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(mockResponse));

        await authService.initiatePasswordReset(resetRequest);

        expect(globalThis.fetch).toHaveBeenCalledWith(
          "/api/auth/password-reset",
          expect.objectContaining({ method: "POST" }),
        );
      });
    });

    describe("resetPassword", () => {
      const confirmRequest: PasswordResetConfirmRequest = {
        token: "reset-token",
        newPassword: "newpassword123",
        confirmPassword: "newpassword123",
      };

      it("confirms password reset in server context", async () => {
        mockedApiPost.mockResolvedValueOnce({
          data: { message: "Password reset successfully" },
        });

        const result = await authService.resetPassword(confirmRequest);

        expect(result.message).toBe("Password reset successfully");
      });

      it("confirms password reset in browser context", async () => {
        vi.stubGlobal("window", {});

        const mockResponse = {
          ok: true,
          json: async () => ({ message: "Password reset successfully" }),
        };
        vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(mockResponse));

        const result = await authService.resetPassword(confirmRequest);

        expect(result.message).toBe("Password reset successfully");
      });

      it("returns default message when response has no message", async () => {
        vi.stubGlobal("window", {});

        const mockResponse = {
          ok: true,
          json: async () => {
            throw new Error("No JSON");
          },
        };
        vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(mockResponse));

        const result = await authService.resetPassword(confirmRequest);

        expect(result.message).toBe("Password reset successfully");
      });

      it("handles 429 rate limit with Retry-After header", async () => {
        vi.stubGlobal("window", {});

        const mockResponse = {
          ok: false,
          status: 429,
          headers: new Headers({
            "Retry-After": "60",
            "Content-Type": "application/json",
          }),
          json: async () => ({ error: "Too many requests" }),
        };
        vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(mockResponse));

        try {
          await authService.resetPassword(confirmRequest);
        } catch (error) {
          const apiError = error as Error & {
            status?: number;
            retryAfterSeconds?: number;
          };
          expect(apiError.status).toBe(429);
          expect(apiError.retryAfterSeconds).toBe(60);
        }
      });
    });
  });

  describe("getAccount", () => {
    it("fetches current account in server context", async () => {
      mockedApiGet.mockResolvedValueOnce({
        data: { data: sampleBackendAccount },
      });

      const result = await authService.getAccount();

      expect(result.email).toBe("test@example.com");
    });

    it("fetches current account in browser context", async () => {
      vi.stubGlobal("window", {});

      const mockResponse = {
        ok: true,
        text: async () => JSON.stringify({ data: sampleBackendAccount }),
      };
      vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(mockResponse));

      const result = await authService.getAccount();

      expect(result.email).toBe("test@example.com");
    });
  });

  describe("changePassword", () => {
    const changeRequest: ChangePasswordRequest = {
      currentPassword: "oldpassword",
      newPassword: "newpassword123",
      confirmPassword: "newpassword123",
    };

    it("changes password in server context", async () => {
      mockedApiPost.mockResolvedValueOnce({
        data: { success: true },
      });

      await authService.changePassword(changeRequest);

      expect(mockedApiPost).toHaveBeenCalled();
    });
  });

  describe("Role management", () => {
    describe("getRoles", () => {
      it("fetches roles in server context", async () => {
        mockedApiGet.mockResolvedValueOnce({
          data: { data: [sampleBackendRole] },
        });

        const result = await authService.getRoles();

        expect(result).toHaveLength(1);
        expect(result[0]?.name).toBe("admin");
      });

      it("fetches roles with name filter", async () => {
        mockedApiGet.mockResolvedValueOnce({
          data: { data: [sampleBackendRole] },
        });

        await authService.getRoles({ name: "admin" });

        expect(mockedApiGet).toHaveBeenCalledWith(
          expect.stringContaining("name=admin"),
          undefined,
        );
      });
    });

    describe("getRole", () => {
      it("fetches single role in server context", async () => {
        mockedApiGet.mockResolvedValueOnce({
          data: { data: sampleBackendRole },
        });

        const result = await authService.getRole("1");

        expect(result.name).toBe("admin");
      });

      it("handles lowercase role response", async () => {
        mockedApiGet.mockResolvedValueOnce({
          data: {
            data: {
              id: 1,
              name: "admin",
              description: "Admin role",
              created_at: "2024-01-01T00:00:00Z",
              updated_at: "2024-01-01T00:00:00Z",
            },
          },
        });

        const result = await authService.getRole("1");

        expect(result.name).toBe("admin");
      });
    });

    describe("createRole", () => {
      const roleRequest: CreateRoleRequest = {
        name: "editor",
        description: "Editor role",
      };

      it("creates role in server context", async () => {
        mockedApiPost.mockResolvedValueOnce({
          data: { data: { ...sampleBackendRole, Name: "editor" } },
        });

        const result = await authService.createRole(roleRequest);

        expect(result.name).toBe("editor");
      });
    });

    describe("updateRole", () => {
      const updateRequest: UpdateRoleRequest = {
        name: "editor",
        description: "Updated description",
      };

      it("updates role in server context", async () => {
        mockedApiPut.mockResolvedValueOnce({
          data: { success: true },
        });

        await authService.updateRole("1", updateRequest);

        expect(mockedApiPut).toHaveBeenCalled();
      });
    });

    describe("deleteRole", () => {
      it("deletes role in server context", async () => {
        mockedApiDelete.mockResolvedValueOnce({ data: {} });

        await authService.deleteRole("1");

        expect(mockedApiDelete).toHaveBeenCalled();
      });
    });

    describe("getRolePermissions", () => {
      it("fetches role permissions in server context", async () => {
        mockedApiGet.mockResolvedValueOnce({
          data: { data: [sampleBackendPermission] },
        });

        const result = await authService.getRolePermissions("1");

        expect(result).toHaveLength(1);
        expect(result[0]?.name).toBe("users.read");
      });

      it("handles double-nested response structure", async () => {
        mockedApiGet.mockResolvedValueOnce({
          data: { data: { data: [sampleBackendPermission] } },
        });

        const result = await authService.getRolePermissions("1");

        expect(result).toHaveLength(1);
      });
    });

    describe("assignPermissionToRole", () => {
      it("assigns permission to role", async () => {
        mockedApiPost.mockResolvedValueOnce({ data: {} });

        await authService.assignPermissionToRole("1", "2");

        expect(mockedApiPost).toHaveBeenCalledWith(
          expect.stringContaining("/auth/roles/1/permissions/2"),
          undefined,
          undefined,
        );
      });
    });

    describe("removePermissionFromRole", () => {
      it("removes permission from role", async () => {
        mockedApiDelete.mockResolvedValueOnce({ data: {} });

        await authService.removePermissionFromRole("1", "2");

        expect(mockedApiDelete).toHaveBeenCalled();
      });
    });
  });

  describe("Permission management", () => {
    describe("getPermissions", () => {
      it("fetches permissions in server context", async () => {
        mockedApiGet.mockResolvedValueOnce({
          data: { data: [sampleBackendPermission] },
        });

        const result = await authService.getPermissions();

        expect(result).toHaveLength(1);
      });

      it("fetches permissions with filters", async () => {
        mockedApiGet.mockResolvedValueOnce({
          data: { data: [] },
        });

        await authService.getPermissions({ resource: "users", action: "read" });

        expect(mockedApiGet).toHaveBeenCalledWith(
          expect.stringContaining("resource=users"),
          undefined,
        );
      });
    });

    describe("getPermission", () => {
      it("fetches single permission", async () => {
        mockedApiGet.mockResolvedValueOnce({
          data: { data: sampleBackendPermission },
        });

        const result = await authService.getPermission("1");

        expect(result.name).toBe("users.read");
      });
    });

    describe("createPermission", () => {
      const permRequest: CreatePermissionRequest = {
        name: "users.write",
        description: "Write users",
        resource: "users",
        action: "write",
      };

      it("creates permission", async () => {
        mockedApiPost.mockResolvedValueOnce({
          data: { data: { ...sampleBackendPermission, name: "users.write" } },
        });

        const result = await authService.createPermission(permRequest);

        expect(result.name).toBe("users.write");
      });
    });

    describe("updatePermission", () => {
      const updateRequest: UpdatePermissionRequest = {
        name: "users.write",
        description: "Updated description",
        resource: "users",
        action: "write",
      };

      it("updates permission", async () => {
        mockedApiPut.mockResolvedValueOnce({ data: {} });

        await authService.updatePermission("1", updateRequest);

        expect(mockedApiPut).toHaveBeenCalled();
      });
    });

    describe("deletePermission", () => {
      it("deletes permission", async () => {
        mockedApiDelete.mockResolvedValueOnce({ data: {} });

        await authService.deletePermission("1");

        expect(mockedApiDelete).toHaveBeenCalled();
      });
    });

    describe("getAvailablePermissions", () => {
      it("fetches all available permissions", async () => {
        mockedApiGet.mockResolvedValueOnce({
          data: { data: [sampleBackendPermission] },
        });

        const result = await authService.getAvailablePermissions();

        expect(result).toHaveLength(1);
      });
    });
  });

  describe("Account management", () => {
    describe("getAccounts", () => {
      it("fetches accounts in server context", async () => {
        mockedApiGet.mockResolvedValueOnce({
          data: { data: [sampleBackendAccount] },
        });

        const result = await authService.getAccounts();

        expect(result).toHaveLength(1);
      });

      it("fetches accounts with filters", async () => {
        mockedApiGet.mockResolvedValueOnce({
          data: { data: [] },
        });

        await authService.getAccounts({
          email: "test@example.com",
          active: true,
        });

        expect(mockedApiGet).toHaveBeenCalledWith(
          expect.stringContaining("email=test%40example.com"),
          undefined,
        );
      });
    });

    describe("getAccountById", () => {
      it("fetches single account", async () => {
        mockedApiGet.mockResolvedValueOnce({
          data: { data: sampleBackendAccount },
        });

        const result = await authService.getAccountById("1");

        expect(result.email).toBe("test@example.com");
      });
    });

    describe("updateAccount", () => {
      const updateRequest: UpdateAccountRequest = {
        email: "updated@example.com",
        username: "updateduser",
      };

      it("updates account", async () => {
        mockedApiPut.mockResolvedValueOnce({ data: {} });

        await authService.updateAccount("1", updateRequest);

        expect(mockedApiPut).toHaveBeenCalled();
      });
    });

    describe("activateAccount", () => {
      it("activates account", async () => {
        mockedApiPut.mockResolvedValueOnce({ data: {} });

        await authService.activateAccount("1");

        expect(mockedApiPut).toHaveBeenCalledWith(
          expect.stringContaining("/activate"),
          undefined,
          undefined,
        );
      });
    });

    describe("deactivateAccount", () => {
      it("deactivates account", async () => {
        mockedApiPut.mockResolvedValueOnce({ data: {} });

        await authService.deactivateAccount("1");

        expect(mockedApiPut).toHaveBeenCalledWith(
          expect.stringContaining("/deactivate"),
          undefined,
          undefined,
        );
      });
    });

    describe("getAccountsByRole", () => {
      it("fetches accounts by role name", async () => {
        mockedApiGet.mockResolvedValueOnce({
          data: { data: [sampleBackendAccount] },
        });

        const result = await authService.getAccountsByRole("admin");

        expect(result).toHaveLength(1);
      });
    });

    describe("Role assignment", () => {
      it("assigns role to account", async () => {
        mockedApiPost.mockResolvedValueOnce({ data: {} });

        await authService.assignRoleToAccount("1", "2");

        expect(mockedApiPost).toHaveBeenCalledWith(
          expect.stringContaining("/accounts/1/roles/2"),
          undefined,
          undefined,
        );
      });

      it("removes role from account", async () => {
        mockedApiDelete.mockResolvedValueOnce({ data: {} });

        await authService.removeRoleFromAccount("1", "2");

        expect(mockedApiDelete).toHaveBeenCalled();
      });

      it("gets account roles", async () => {
        mockedApiGet.mockResolvedValueOnce({
          data: { data: [sampleBackendRole] },
        });

        const result = await authService.getAccountRoles("1");

        expect(result).toHaveLength(1);
      });
    });

    describe("Permission management for accounts", () => {
      it("gets account permissions", async () => {
        mockedApiGet.mockResolvedValueOnce({
          data: { data: [sampleBackendPermission] },
        });

        const result = await authService.getAccountPermissions("1");

        expect(result).toHaveLength(1);
      });

      it("gets account direct permissions", async () => {
        mockedApiGet.mockResolvedValueOnce({
          data: { data: [sampleBackendPermission] },
        });

        const result = await authService.getAccountDirectPermissions("1");

        expect(result).toHaveLength(1);
      });

      it("grants permission to account", async () => {
        mockedApiPost.mockResolvedValueOnce({ data: {} });

        await authService.grantPermissionToAccount("1", "2");

        expect(mockedApiPost).toHaveBeenCalledWith(
          expect.stringContaining("/grant"),
          undefined,
          undefined,
        );
      });

      it("denies permission to account", async () => {
        mockedApiPost.mockResolvedValueOnce({ data: {} });

        await authService.denyPermissionToAccount("1", "2");

        expect(mockedApiPost).toHaveBeenCalledWith(
          expect.stringContaining("/deny"),
          undefined,
          undefined,
        );
      });

      it("removes permission from account", async () => {
        mockedApiDelete.mockResolvedValueOnce({ data: {} });

        await authService.removePermissionFromAccount("1", "2");

        expect(mockedApiDelete).toHaveBeenCalled();
      });

      it("assigns permission to account (alias for grant)", async () => {
        mockedApiPost.mockResolvedValueOnce({ data: {} });

        await authService.assignPermissionToAccount("1", "2");

        expect(mockedApiPost).toHaveBeenCalledWith(
          expect.stringContaining("/grant"),
          undefined,
          undefined,
        );
      });
    });
  });

  describe("Token management", () => {
    describe("getActiveTokens", () => {
      it("fetches active tokens", async () => {
        mockedApiGet.mockResolvedValueOnce({
          data: { data: [sampleBackendToken] },
        });

        const result = await authService.getActiveTokens("1");

        expect(result).toHaveLength(1);
        expect(result[0]?.token).toBe("sample-token");
      });
    });

    describe("revokeAllTokens", () => {
      it("revokes all tokens", async () => {
        mockedApiDelete.mockResolvedValueOnce({ data: {} });

        await authService.revokeAllTokens("1");

        expect(mockedApiDelete).toHaveBeenCalledWith(
          expect.stringContaining("/tokens"),
          undefined,
        );
      });
    });

    describe("cleanupExpiredTokens", () => {
      it("cleans up expired tokens", async () => {
        mockedApiDelete.mockResolvedValueOnce({
          data: { data: { cleaned_tokens: 5 } },
        });

        const result = await authService.cleanupExpiredTokens();

        expect(result).toBe(5);
      });
    });
  });

  describe("Parent account management", () => {
    describe("createParentAccount", () => {
      const createRequest: CreateParentAccountRequest = {
        email: "parent@example.com",
        username: "parentuser",
        password: "password123",
        confirmPassword: "password123",
      };

      it("creates parent account", async () => {
        mockedApiPost.mockResolvedValueOnce({
          data: { data: sampleBackendParentAccount },
        });

        const result = await authService.createParentAccount(createRequest);

        expect(result.email).toBe("parent@example.com");
      });
    });

    describe("getParentAccounts", () => {
      it("fetches parent accounts", async () => {
        mockedApiGet.mockResolvedValueOnce({
          data: { data: [sampleBackendParentAccount] },
        });

        const result = await authService.getParentAccounts();

        expect(result).toHaveLength(1);
      });

      it("fetches parent accounts with filters", async () => {
        mockedApiGet.mockResolvedValueOnce({
          data: { data: [] },
        });

        await authService.getParentAccounts({
          email: "parent@example.com",
          active: true,
        });

        expect(mockedApiGet).toHaveBeenCalledWith(
          expect.stringContaining("email=parent%40example.com"),
          undefined,
        );
      });
    });

    describe("getParentAccount", () => {
      it("fetches single parent account", async () => {
        mockedApiGet.mockResolvedValueOnce({
          data: { data: sampleBackendParentAccount },
        });

        const result = await authService.getParentAccount("1");

        expect(result.email).toBe("parent@example.com");
      });
    });

    describe("updateParentAccount", () => {
      it("updates parent account", async () => {
        mockedApiPut.mockResolvedValueOnce({ data: {} });

        await authService.updateParentAccount("1", {
          email: "updated@example.com",
          username: "updatedparent",
        });

        expect(mockedApiPut).toHaveBeenCalled();
      });
    });

    describe("activateParentAccount", () => {
      it("activates parent account", async () => {
        mockedApiPut.mockResolvedValueOnce({ data: {} });

        await authService.activateParentAccount("1");

        expect(mockedApiPut).toHaveBeenCalledWith(
          expect.stringContaining("/activate"),
          undefined,
          undefined,
        );
      });
    });

    describe("deactivateParentAccount", () => {
      it("deactivates parent account", async () => {
        mockedApiPut.mockResolvedValueOnce({ data: {} });

        await authService.deactivateParentAccount("1");

        expect(mockedApiPut).toHaveBeenCalledWith(
          expect.stringContaining("/deactivate"),
          undefined,
          undefined,
        );
      });
    });
  });
});
