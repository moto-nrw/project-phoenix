/**
 * Tests for auth-client.ts
 *
 * These tests verify the BetterAuth client wrapper functions work correctly.
 * We mock better-auth at the module level to test our wrapper logic.
 */
import { describe, it, expect, vi, beforeEach, type Mock } from "vitest";

// Use vi.hoisted to define mocks that need to be available during module initialization
const { mockGetActiveMemberRole, mockGetFullOrganization, mockSetActive } =
  vi.hoisted(() => ({
    mockGetActiveMemberRole: vi.fn(),
    mockGetFullOrganization: vi.fn(),
    mockSetActive: vi.fn(),
  }));

// Mock better-auth modules BEFORE importing auth-client
vi.mock("better-auth/react", () => ({
  createAuthClient: vi.fn(() => ({
    signIn: { email: vi.fn() },
    signOut: vi.fn(),
    signUp: { email: vi.fn() },
    useSession: vi.fn(),
    getSession: vi.fn(),
    organization: {
      getActiveMemberRole: mockGetActiveMemberRole,
      getFullOrganization: mockGetFullOrganization,
      setActive: mockSetActive,
    },
  })),
}));

vi.mock("better-auth/client/plugins", () => ({
  organizationClient: vi.fn(() => ({})),
}));

// Unmock auth-client so we test the actual implementation
vi.unmock("~/lib/auth-client");

// Import after mocks are set up
import {
  getActiveRole,
  isAdmin,
  isSupervisor,
  getOrganizationInfo,
  switchOrganization,
  signupWithOrganization,
  SignupWithOrgException,
} from "./auth-client";

describe("auth-client", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("getActiveRole", () => {
    it("returns role when data is present", async () => {
      mockGetActiveMemberRole.mockResolvedValueOnce({
        data: { role: "supervisor" },
      });

      const role = await getActiveRole();

      expect(role).toBe("supervisor");
      expect(mockGetActiveMemberRole).toHaveBeenCalledWith({});
    });

    it("returns null when data is null", async () => {
      mockGetActiveMemberRole.mockResolvedValueOnce({ data: null });

      const role = await getActiveRole();

      expect(role).toBeNull();
    });

    it("returns null when data.role is undefined", async () => {
      mockGetActiveMemberRole.mockResolvedValueOnce({ data: {} });

      const role = await getActiveRole();

      expect(role).toBeNull();
    });

    it("returns all Phoenix role types correctly", async () => {
      const roles = [
        "admin",
        "member",
        "owner",
        "supervisor",
        "ogsAdmin",
        "bueroAdmin",
        "traegerAdmin",
      ] as const;

      for (const expectedRole of roles) {
        mockGetActiveMemberRole.mockResolvedValueOnce({
          data: { role: expectedRole },
        });

        const role = await getActiveRole();

        expect(role).toBe(expectedRole);
      }
    });
  });

  describe("isAdmin", () => {
    it('returns true for "admin" role', async () => {
      mockGetActiveMemberRole.mockResolvedValueOnce({
        data: { role: "admin" },
      });

      const result = await isAdmin();

      expect(result).toBe(true);
    });

    it('returns true for "owner" role', async () => {
      mockGetActiveMemberRole.mockResolvedValueOnce({
        data: { role: "owner" },
      });

      const result = await isAdmin();

      expect(result).toBe(true);
    });

    it('returns true for "ogsAdmin" role', async () => {
      mockGetActiveMemberRole.mockResolvedValueOnce({
        data: { role: "ogsAdmin" },
      });

      const result = await isAdmin();

      expect(result).toBe(true);
    });

    it('returns true for "bueroAdmin" role', async () => {
      mockGetActiveMemberRole.mockResolvedValueOnce({
        data: { role: "bueroAdmin" },
      });

      const result = await isAdmin();

      expect(result).toBe(true);
    });

    it('returns true for "traegerAdmin" role', async () => {
      mockGetActiveMemberRole.mockResolvedValueOnce({
        data: { role: "traegerAdmin" },
      });

      const result = await isAdmin();

      expect(result).toBe(true);
    });

    it('returns false for "supervisor" role', async () => {
      mockGetActiveMemberRole.mockResolvedValueOnce({
        data: { role: "supervisor" },
      });

      const result = await isAdmin();

      expect(result).toBe(false);
    });

    it('returns false for "member" role', async () => {
      mockGetActiveMemberRole.mockResolvedValueOnce({
        data: { role: "member" },
      });

      const result = await isAdmin();

      expect(result).toBe(false);
    });

    it("returns false when role is null", async () => {
      mockGetActiveMemberRole.mockResolvedValueOnce({ data: null });

      const result = await isAdmin();

      expect(result).toBe(false);
    });
  });

  describe("isSupervisor", () => {
    it('returns true for "supervisor" role', async () => {
      mockGetActiveMemberRole.mockResolvedValueOnce({
        data: { role: "supervisor" },
      });

      const result = await isSupervisor();

      expect(result).toBe(true);
    });

    it('returns false for "admin" role', async () => {
      mockGetActiveMemberRole.mockResolvedValueOnce({
        data: { role: "admin" },
      });

      const result = await isSupervisor();

      expect(result).toBe(false);
    });

    it('returns false for "member" role', async () => {
      mockGetActiveMemberRole.mockResolvedValueOnce({
        data: { role: "member" },
      });

      const result = await isSupervisor();

      expect(result).toBe(false);
    });

    it("returns false when role is null", async () => {
      mockGetActiveMemberRole.mockResolvedValueOnce({ data: null });

      const result = await isSupervisor();

      expect(result).toBe(false);
    });
  });

  describe("getOrganizationInfo", () => {
    it("returns mapped OrganizationInfo when data exists", async () => {
      const mockOrgData = {
        id: "org-123",
        name: "Test OGS",
        slug: "test-ogs",
        metadata: {
          traegerId: "traeger-1",
          bueroId: "buero-1",
        },
      };
      mockGetFullOrganization.mockResolvedValueOnce({ data: mockOrgData });

      const result = await getOrganizationInfo("org-123");

      expect(result).toEqual({
        id: "org-123",
        name: "Test OGS",
        slug: "test-ogs",
        metadata: {
          traegerId: "traeger-1",
          bueroId: "buero-1",
        },
      });
      expect(mockGetFullOrganization).toHaveBeenCalledWith({
        query: { organizationId: "org-123" },
      });
    });

    it("returns null when data is null", async () => {
      mockGetFullOrganization.mockResolvedValueOnce({ data: null });

      const result = await getOrganizationInfo("org-456");

      expect(result).toBeNull();
    });

    it("handles organization without metadata", async () => {
      const mockOrgData = {
        id: "org-789",
        name: "Simple OGS",
        slug: "simple-ogs",
      };
      mockGetFullOrganization.mockResolvedValueOnce({ data: mockOrgData });

      const result = await getOrganizationInfo("org-789");

      expect(result).toEqual({
        id: "org-789",
        name: "Simple OGS",
        slug: "simple-ogs",
        metadata: undefined,
      });
    });
  });

  describe("switchOrganization", () => {
    it("calls setActive with correct organizationId", async () => {
      mockSetActive.mockResolvedValueOnce({});

      await switchOrganization("new-org-id");

      expect(mockSetActive).toHaveBeenCalledWith({
        organizationId: "new-org-id",
      });
    });

    it("does not return any value", async () => {
      mockSetActive.mockResolvedValueOnce({ data: { success: true } });

      const result = await switchOrganization("org-id");

      expect(result).toBeUndefined();
    });
  });

  describe("signupWithOrganization", () => {
    const validRequest = {
      name: "Test User",
      email: "test@example.com",
      password: "SecurePass123!",
      orgName: "Test Organization",
      orgSlug: "test-org",
    };

    const successResponse = {
      success: true,
      user: {
        id: "user-123",
        email: "test@example.com",
        name: "Test User",
        emailVerified: false,
        createdAt: "2025-01-23T00:00:00Z",
      },
      organization: {
        id: "org-123",
        name: "Test Organization",
        slug: "test-org",
        status: "active",
      },
      session: {
        token: "session-token-abc",
        expiresAt: "2025-01-24T00:00:00Z",
      },
    };

    beforeEach(() => {
      global.fetch = vi.fn() as Mock;
    });

    it("returns response on successful signup", async () => {
      (global.fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(successResponse),
      });

      const result = await signupWithOrganization(validRequest);

      expect(result).toEqual(successResponse);
      expect(global.fetch).toHaveBeenCalledWith("/api/auth/signup-with-org", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(validRequest),
      });
    });

    it("throws SignupWithOrgException on error response", async () => {
      const errorResponse = {
        error: "Email already registered",
        code: "EMAIL_TAKEN",
        field: "email",
      };
      (global.fetch as Mock).mockResolvedValueOnce({
        ok: false,
        status: 409,
        json: () => Promise.resolve(errorResponse),
      });

      await expect(signupWithOrganization(validRequest)).rejects.toThrow(
        SignupWithOrgException,
      );
    });

    it("includes error details in thrown exception", async () => {
      const errorResponse = {
        error: "Email already registered",
        code: "EMAIL_TAKEN",
        field: "email",
      };
      (global.fetch as Mock).mockResolvedValueOnce({
        ok: false,
        status: 409,
        json: () => Promise.resolve(errorResponse),
      });

      try {
        await signupWithOrganization(validRequest);
        expect.fail("Should have thrown");
      } catch (error) {
        const e = error as SignupWithOrgException;
        expect(e.message).toBe("Email already registered");
        expect(e.code).toBe("EMAIL_TAKEN");
        expect(e.field).toBe("email");
        expect(e.status).toBe(409);
      }
    });

    it("throws SignupWithOrgException on slug conflict", async () => {
      const errorResponse = {
        error: "Organization slug already taken",
        code: "SLUG_TAKEN",
        field: "orgSlug",
      };
      (global.fetch as Mock).mockResolvedValueOnce({
        ok: false,
        status: 409,
        json: () => Promise.resolve(errorResponse),
      });

      await expect(signupWithOrganization(validRequest)).rejects.toThrow(
        SignupWithOrgException,
      );
    });

    it("handles error response without code and field", async () => {
      const errorResponse = {
        error: "Internal server error",
      };
      (global.fetch as Mock).mockResolvedValueOnce({
        ok: false,
        status: 500,
        json: () => Promise.resolve(errorResponse),
      });

      try {
        await signupWithOrganization(validRequest);
        expect.fail("Should have thrown");
      } catch (error) {
        const e = error as SignupWithOrgException;
        expect(e.message).toBe("Internal server error");
        expect(e.code).toBeUndefined();
        expect(e.field).toBeUndefined();
        expect(e.status).toBe(500);
      }
    });

    it("handles validation error (400)", async () => {
      const errorResponse = {
        error: "Password too weak",
        code: "WEAK_PASSWORD",
        field: "password",
      };
      (global.fetch as Mock).mockResolvedValueOnce({
        ok: false,
        status: 400,
        json: () => Promise.resolve(errorResponse),
      });

      try {
        await signupWithOrganization(validRequest);
        expect.fail("Should have thrown");
      } catch (error) {
        const e = error as SignupWithOrgException;
        expect(e.status).toBe(400);
        expect(e.code).toBe("WEAK_PASSWORD");
      }
    });
  });

  describe("SignupWithOrgException", () => {
    it("sets all properties correctly", () => {
      const error = new SignupWithOrgException(
        "Test error",
        "TEST_CODE",
        "testField",
        422,
      );

      expect(error.message).toBe("Test error");
      expect(error.code).toBe("TEST_CODE");
      expect(error.field).toBe("testField");
      expect(error.status).toBe(422);
      expect(error.name).toBe("SignupWithOrgException");
    });

    it("uses default status of 500 when not provided", () => {
      const error = new SignupWithOrgException("Error message");

      expect(error.status).toBe(500);
    });

    it("allows undefined code and field", () => {
      const error = new SignupWithOrgException(
        "Error only",
        undefined,
        undefined,
        400,
      );

      expect(error.code).toBeUndefined();
      expect(error.field).toBeUndefined();
      expect(error.status).toBe(400);
    });

    it("is instanceof Error", () => {
      const error = new SignupWithOrgException("Test");

      expect(error).toBeInstanceOf(Error);
      expect(error).toBeInstanceOf(SignupWithOrgException);
    });
  });
});
