/**
 * Tests for Admin API client.
 *
 * Covers:
 * - getErrorMessage helper (internal)
 * - createOrganization
 * - fetchOrganizations (with/without status filter)
 * - approveOrganization
 * - rejectOrganization (with/without reason)
 * - suspendOrganization
 * - reactivateOrganization
 *
 * Each function tests:
 * - Success path
 * - Error path with JSON response
 * - Error path where JSON parsing fails
 */
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  createOrganization,
  fetchOrganizations,
  approveOrganization,
  rejectOrganization,
  suspendOrganization,
  reactivateOrganization,
  type Organization,
  type OrgActionResponse,
  type OrganizationsResponse,
} from "./admin-api";

// Mock organization fixture
const mockOrganization: Organization = {
  id: "org-123",
  name: "Test School",
  slug: "test-school",
  status: "active",
  createdAt: "2024-01-01T00:00:00Z",
  ownerEmail: "owner@test.com",
  ownerName: "Test Owner",
};

const mockOrgActionResponse: OrgActionResponse = {
  success: true,
  organization: mockOrganization,
};

const mockOrganizationsResponse: OrganizationsResponse = {
  organizations: [mockOrganization],
};

describe("admin-api", () => {
  // Store original fetch
  const originalFetch = global.fetch;

  beforeEach(() => {
    vi.resetAllMocks();
    // Mock window.location.origin for URL construction
    Object.defineProperty(window, "location", {
      value: { origin: "http://localhost:3000" },
      writable: true,
    });
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  // Helper to create a mock Response
  function createMockResponse(
    body: unknown,
    options: { ok: boolean; status?: number } = { ok: true },
  ): Response {
    return {
      ok: options.ok,
      status: options.status ?? (options.ok ? 200 : 400),
      json: () => Promise.resolve(body),
    } as Response;
  }

  // Helper to create a mock Response that fails JSON parsing
  function createMockResponseWithJsonError(
    options: { ok: boolean; status?: number } = { ok: false },
  ): Response {
    return {
      ok: options.ok,
      status: options.status ?? 500,
      json: () => Promise.reject(new Error("Invalid JSON")),
    } as Response;
  }

  describe("createOrganization", () => {
    it("should create an organization successfully", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(createMockResponse(mockOrgActionResponse));

      const result = await createOrganization({ name: "Test School" });

      expect(result).toEqual(mockOrganization);
      expect(global.fetch).toHaveBeenCalledWith("/api/admin/organizations", {
        method: "POST",
        credentials: "include",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ name: "Test School" }),
      });
    });

    it("should create an organization with optional slug", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(createMockResponse(mockOrgActionResponse));

      await createOrganization({ name: "Test School", slug: "custom-slug" });

      expect(global.fetch).toHaveBeenCalledWith("/api/admin/organizations", {
        method: "POST",
        credentials: "include",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ name: "Test School", slug: "custom-slug" }),
      });
    });

    it("should throw error with message from API response", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponse(
            { error: "Organization name already exists" },
            { ok: false, status: 409 },
          ),
        );

      await expect(createOrganization({ name: "Duplicate" })).rejects.toThrow(
        "Organization name already exists",
      );
    });

    it("should throw 'Unknown error' when API returns non-string error", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponse(
            { error: { code: 500 } },
            { ok: false, status: 500 },
          ),
        );

      await expect(createOrganization({ name: "Test" })).rejects.toThrow(
        "Unknown error",
      );
    });

    it("should throw 'Unknown error' when JSON parsing fails", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponseWithJsonError({ ok: false, status: 500 }),
        );

      await expect(createOrganization({ name: "Test" })).rejects.toThrow(
        "Unknown error",
      );
    });

    it("should throw 'Unknown error' when response has no error field", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponse(
            { message: "Something went wrong" },
            { ok: false, status: 400 },
          ),
        );

      await expect(createOrganization({ name: "Test" })).rejects.toThrow(
        "Unknown error",
      );
    });
  });

  describe("fetchOrganizations", () => {
    it("should fetch organizations without status filter", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(createMockResponse(mockOrganizationsResponse));

      const result = await fetchOrganizations();

      expect(result).toEqual([mockOrganization]);
      expect(global.fetch).toHaveBeenCalledWith(
        "http://localhost:3000/api/admin/organizations",
        {
          method: "GET",
          credentials: "include",
        },
      );
    });

    it("should fetch organizations with status filter", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(createMockResponse(mockOrganizationsResponse));

      await fetchOrganizations("pending");

      expect(global.fetch).toHaveBeenCalledWith(
        "http://localhost:3000/api/admin/organizations?status=pending",
        {
          method: "GET",
          credentials: "include",
        },
      );
    });

    it("should fetch organizations with 'active' status filter", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(createMockResponse(mockOrganizationsResponse));

      await fetchOrganizations("active");

      expect(global.fetch).toHaveBeenCalledWith(
        "http://localhost:3000/api/admin/organizations?status=active",
        {
          method: "GET",
          credentials: "include",
        },
      );
    });

    it("should fetch organizations with 'rejected' status filter", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(createMockResponse(mockOrganizationsResponse));

      await fetchOrganizations("rejected");

      expect(global.fetch).toHaveBeenCalledWith(
        "http://localhost:3000/api/admin/organizations?status=rejected",
        {
          method: "GET",
          credentials: "include",
        },
      );
    });

    it("should fetch organizations with 'suspended' status filter", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(createMockResponse(mockOrganizationsResponse));

      await fetchOrganizations("suspended");

      expect(global.fetch).toHaveBeenCalledWith(
        "http://localhost:3000/api/admin/organizations?status=suspended",
        {
          method: "GET",
          credentials: "include",
        },
      );
    });

    it("should throw error with message from API response", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponse(
            { error: "Unauthorized" },
            { ok: false, status: 401 },
          ),
        );

      await expect(fetchOrganizations()).rejects.toThrow("Unauthorized");
    });

    it("should throw 'Unknown error' when JSON parsing fails", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponseWithJsonError({ ok: false, status: 500 }),
        );

      await expect(fetchOrganizations()).rejects.toThrow("Unknown error");
    });

    it("should return empty array when no organizations", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(createMockResponse({ organizations: [] }));

      const result = await fetchOrganizations();

      expect(result).toEqual([]);
    });
  });

  describe("approveOrganization", () => {
    it("should approve an organization successfully", async () => {
      const approvedOrg = { ...mockOrganization, status: "active" as const };
      global.fetch = vi.fn().mockResolvedValue(
        createMockResponse({
          success: true,
          organization: approvedOrg,
        }),
      );

      const result = await approveOrganization("org-123");

      expect(result).toEqual(approvedOrg);
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/admin/organizations/org-123/approve",
        {
          method: "POST",
          credentials: "include",
          headers: {
            "Content-Type": "application/json",
          },
        },
      );
    });

    it("should throw error with message from API response", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponse(
            { error: "Organization not found" },
            { ok: false, status: 404 },
          ),
        );

      await expect(approveOrganization("invalid-id")).rejects.toThrow(
        "Organization not found",
      );
    });

    it("should throw 'Unknown error' when JSON parsing fails", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponseWithJsonError({ ok: false, status: 500 }),
        );

      await expect(approveOrganization("org-123")).rejects.toThrow(
        "Unknown error",
      );
    });

    it("should throw error when organization is already approved", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponse(
            { error: "Organization is already active" },
            { ok: false, status: 400 },
          ),
        );

      await expect(approveOrganization("org-123")).rejects.toThrow(
        "Organization is already active",
      );
    });
  });

  describe("rejectOrganization", () => {
    it("should reject an organization without reason", async () => {
      const rejectedOrg = { ...mockOrganization, status: "rejected" as const };
      global.fetch = vi.fn().mockResolvedValue(
        createMockResponse({
          success: true,
          organization: rejectedOrg,
        }),
      );

      const result = await rejectOrganization("org-123");

      expect(result).toEqual(rejectedOrg);
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/admin/organizations/org-123/reject",
        {
          method: "POST",
          credentials: "include",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({ reason: undefined }),
        },
      );
    });

    it("should reject an organization with reason", async () => {
      const rejectedOrg = { ...mockOrganization, status: "rejected" as const };
      global.fetch = vi.fn().mockResolvedValue(
        createMockResponse({
          success: true,
          organization: rejectedOrg,
        }),
      );

      const result = await rejectOrganization(
        "org-123",
        "Does not meet requirements",
      );

      expect(result).toEqual(rejectedOrg);
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/admin/organizations/org-123/reject",
        {
          method: "POST",
          credentials: "include",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({ reason: "Does not meet requirements" }),
        },
      );
    });

    it("should throw error with message from API response", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponse(
            { error: "Organization not found" },
            { ok: false, status: 404 },
          ),
        );

      await expect(rejectOrganization("invalid-id")).rejects.toThrow(
        "Organization not found",
      );
    });

    it("should throw 'Unknown error' when JSON parsing fails", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponseWithJsonError({ ok: false, status: 500 }),
        );

      await expect(rejectOrganization("org-123")).rejects.toThrow(
        "Unknown error",
      );
    });

    it("should throw error when organization is not pending", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponse(
            { error: "Cannot reject an active organization" },
            { ok: false, status: 400 },
          ),
        );

      await expect(rejectOrganization("org-123")).rejects.toThrow(
        "Cannot reject an active organization",
      );
    });
  });

  describe("suspendOrganization", () => {
    it("should suspend an organization successfully", async () => {
      const suspendedOrg = {
        ...mockOrganization,
        status: "suspended" as const,
      };
      global.fetch = vi.fn().mockResolvedValue(
        createMockResponse({
          success: true,
          organization: suspendedOrg,
        }),
      );

      const result = await suspendOrganization("org-123");

      expect(result).toEqual(suspendedOrg);
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/admin/organizations/org-123/suspend",
        {
          method: "POST",
          credentials: "include",
          headers: {
            "Content-Type": "application/json",
          },
        },
      );
    });

    it("should throw error with message from API response", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponse(
            { error: "Organization not found" },
            { ok: false, status: 404 },
          ),
        );

      await expect(suspendOrganization("invalid-id")).rejects.toThrow(
        "Organization not found",
      );
    });

    it("should throw 'Unknown error' when JSON parsing fails", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponseWithJsonError({ ok: false, status: 500 }),
        );

      await expect(suspendOrganization("org-123")).rejects.toThrow(
        "Unknown error",
      );
    });

    it("should throw error when organization is not active", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponse(
            { error: "Cannot suspend a pending organization" },
            { ok: false, status: 400 },
          ),
        );

      await expect(suspendOrganization("org-123")).rejects.toThrow(
        "Cannot suspend a pending organization",
      );
    });

    it("should throw error when organization is already suspended", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponse(
            { error: "Organization is already suspended" },
            { ok: false, status: 400 },
          ),
        );

      await expect(suspendOrganization("org-123")).rejects.toThrow(
        "Organization is already suspended",
      );
    });
  });

  describe("reactivateOrganization", () => {
    it("should reactivate an organization successfully", async () => {
      const reactivatedOrg = { ...mockOrganization, status: "active" as const };
      global.fetch = vi.fn().mockResolvedValue(
        createMockResponse({
          success: true,
          organization: reactivatedOrg,
        }),
      );

      const result = await reactivateOrganization("org-123");

      expect(result).toEqual(reactivatedOrg);
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/admin/organizations/org-123/reactivate",
        {
          method: "POST",
          credentials: "include",
          headers: {
            "Content-Type": "application/json",
          },
        },
      );
    });

    it("should throw error with message from API response", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponse(
            { error: "Organization not found" },
            { ok: false, status: 404 },
          ),
        );

      await expect(reactivateOrganization("invalid-id")).rejects.toThrow(
        "Organization not found",
      );
    });

    it("should throw 'Unknown error' when JSON parsing fails", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponseWithJsonError({ ok: false, status: 500 }),
        );

      await expect(reactivateOrganization("org-123")).rejects.toThrow(
        "Unknown error",
      );
    });

    it("should throw error when organization is not suspended", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponse(
            { error: "Cannot reactivate a non-suspended organization" },
            { ok: false, status: 400 },
          ),
        );

      await expect(reactivateOrganization("org-123")).rejects.toThrow(
        "Cannot reactivate a non-suspended organization",
      );
    });

    it("should throw error when organization is pending", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponse(
            { error: "Cannot reactivate a pending organization" },
            { ok: false, status: 400 },
          ),
        );

      await expect(reactivateOrganization("org-123")).rejects.toThrow(
        "Cannot reactivate a pending organization",
      );
    });
  });

  describe("getErrorMessage edge cases", () => {
    // Test the internal getErrorMessage function indirectly through error paths
    it("should handle null error response", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponse(null, { ok: false, status: 400 }),
        );

      await expect(createOrganization({ name: "Test" })).rejects.toThrow(
        "Unknown error",
      );
    });

    it("should handle undefined error response", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponse(undefined, { ok: false, status: 400 }),
        );

      await expect(createOrganization({ name: "Test" })).rejects.toThrow(
        "Unknown error",
      );
    });

    it("should handle empty object error response", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(createMockResponse({}, { ok: false, status: 400 }));

      await expect(createOrganization({ name: "Test" })).rejects.toThrow(
        "Unknown error",
      );
    });

    it("should handle array error response", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponse(["error1", "error2"], { ok: false, status: 400 }),
        );

      await expect(createOrganization({ name: "Test" })).rejects.toThrow(
        "Unknown error",
      );
    });

    it("should handle error with number value", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponse({ error: 500 }, { ok: false, status: 500 }),
        );

      await expect(createOrganization({ name: "Test" })).rejects.toThrow(
        "Unknown error",
      );
    });

    it("should handle error with boolean value", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponse({ error: true }, { ok: false, status: 400 }),
        );

      await expect(createOrganization({ name: "Test" })).rejects.toThrow(
        "Unknown error",
      );
    });

    it("should handle primitive string error response", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(
          createMockResponse("string error", { ok: false, status: 400 }),
        );

      await expect(createOrganization({ name: "Test" })).rejects.toThrow(
        "Unknown error",
      );
    });
  });

  describe("organization with null owner fields", () => {
    it("should handle organization with null owner fields", async () => {
      const orgWithNullOwner: Organization = {
        ...mockOrganization,
        ownerEmail: null,
        ownerName: null,
      };
      global.fetch = vi.fn().mockResolvedValue(
        createMockResponse({
          success: true,
          organization: orgWithNullOwner,
        }),
      );

      const result = await createOrganization({ name: "No Owner Org" });

      expect(result.ownerEmail).toBeNull();
      expect(result.ownerName).toBeNull();
    });
  });

  describe("fetch configuration", () => {
    it("should always include credentials", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(createMockResponse(mockOrgActionResponse));

      await createOrganization({ name: "Test" });

      expect(global.fetch).toHaveBeenCalledWith(
        expect.any(String),
        expect.objectContaining({ credentials: "include" }),
      );
    });

    it("should use POST method for create", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(createMockResponse(mockOrgActionResponse));

      await createOrganization({ name: "Test" });

      expect(global.fetch).toHaveBeenCalledWith(
        expect.any(String),
        expect.objectContaining({ method: "POST" }),
      );
    });

    it("should use GET method for fetch", async () => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(createMockResponse(mockOrganizationsResponse));

      await fetchOrganizations();

      expect(global.fetch).toHaveBeenCalledWith(
        expect.any(String),
        expect.objectContaining({ method: "GET" }),
      );
    });
  });
});
