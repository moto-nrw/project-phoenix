import { describe, it, expect, vi, beforeEach } from "vitest";

// Mock next/headers before importing the module under test
vi.mock("next/headers", () => ({
  headers: vi.fn(),
}));

// Import after mocking
import {
  getTenantContext,
  isTenantRequest,
  getTenantSlug,
  getTenantId,
  requireTenantContext,
} from "./tenant-context";
import { headers } from "next/headers";

// Type the mocked function
const mockedHeaders = vi.mocked(headers);

// Helper to create a mock ReadonlyHeaders-like object
function createMockHeaders(
  headerMap: Record<string, string | null>,
): Awaited<ReturnType<typeof headers>> {
  return {
    get: (name: string) => headerMap[name] ?? null,
  } as Awaited<ReturnType<typeof headers>>;
}

describe("tenant-context", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // ==========================================================================
  // getTenantContext
  // ==========================================================================
  describe("getTenantContext", () => {
    it("returns full tenant context when all headers are present", async () => {
      mockedHeaders.mockResolvedValueOnce(
        createMockHeaders({
          "x-tenant-slug": "ogs-musterstadt",
          "x-tenant-id": "org-123",
          "x-tenant-name": "OGS Musterstadt",
        }),
      );

      const context = await getTenantContext();

      expect(context).toEqual({
        slug: "ogs-musterstadt",
        id: "org-123",
        name: "OGS Musterstadt",
        isMultiTenant: true,
      });
    });

    it("returns partial context when only slug is present", async () => {
      mockedHeaders.mockResolvedValueOnce(
        createMockHeaders({
          "x-tenant-slug": "ogs-musterstadt",
          "x-tenant-id": null,
          "x-tenant-name": null,
        }),
      );

      const context = await getTenantContext();

      expect(context).toEqual({
        slug: "ogs-musterstadt",
        id: null,
        name: null,
        isMultiTenant: true,
      });
    });

    it("returns null values and isMultiTenant false when no headers are present", async () => {
      mockedHeaders.mockResolvedValueOnce(
        createMockHeaders({
          "x-tenant-slug": null,
          "x-tenant-id": null,
          "x-tenant-name": null,
        }),
      );

      const context = await getTenantContext();

      expect(context).toEqual({
        slug: null,
        id: null,
        name: null,
        isMultiTenant: false,
      });
    });

    it("handles empty string headers as falsy but present", async () => {
      mockedHeaders.mockResolvedValueOnce(
        createMockHeaders({
          "x-tenant-slug": "",
          "x-tenant-id": "",
          "x-tenant-name": "",
        }),
      );

      const context = await getTenantContext();

      // Empty string is still a value, not null
      expect(context.slug).toBe("");
      expect(context.id).toBe("");
      expect(context.name).toBe("");
      // Empty string is truthy for null check
      expect(context.isMultiTenant).toBe(true);
    });

    it("handles special characters in tenant slug", async () => {
      mockedHeaders.mockResolvedValueOnce(
        createMockHeaders({
          "x-tenant-slug": "ogs-köln-süd",
          "x-tenant-id": "org-456",
          "x-tenant-name": "OGS Köln-Süd",
        }),
      );

      const context = await getTenantContext();

      expect(context.slug).toBe("ogs-köln-süd");
      expect(context.name).toBe("OGS Köln-Süd");
    });
  });

  // ==========================================================================
  // isTenantRequest
  // ==========================================================================
  describe("isTenantRequest", () => {
    it("returns true when x-tenant-slug header is present", async () => {
      mockedHeaders.mockResolvedValueOnce(
        createMockHeaders({
          "x-tenant-slug": "ogs-test",
        }),
      );

      const result = await isTenantRequest();

      expect(result).toBe(true);
    });

    it("returns false when x-tenant-slug header is not present", async () => {
      mockedHeaders.mockResolvedValueOnce(
        createMockHeaders({
          "x-tenant-slug": null,
        }),
      );

      const result = await isTenantRequest();

      expect(result).toBe(false);
    });

    it("returns true when x-tenant-slug is empty string", async () => {
      mockedHeaders.mockResolvedValueOnce(
        createMockHeaders({
          "x-tenant-slug": "",
        }),
      );

      const result = await isTenantRequest();

      // Empty string !== null, so this is technically a tenant request
      expect(result).toBe(true);
    });
  });

  // ==========================================================================
  // getTenantSlug
  // ==========================================================================
  describe("getTenantSlug", () => {
    it("returns the tenant slug when present", async () => {
      mockedHeaders.mockResolvedValueOnce(
        createMockHeaders({
          "x-tenant-slug": "ogs-example",
        }),
      );

      const slug = await getTenantSlug();

      expect(slug).toBe("ogs-example");
    });

    it("returns null when tenant slug is not present", async () => {
      mockedHeaders.mockResolvedValueOnce(
        createMockHeaders({
          "x-tenant-slug": null,
        }),
      );

      const slug = await getTenantSlug();

      expect(slug).toBeNull();
    });

    it("returns empty string when header value is empty", async () => {
      mockedHeaders.mockResolvedValueOnce(
        createMockHeaders({
          "x-tenant-slug": "",
        }),
      );

      const slug = await getTenantSlug();

      expect(slug).toBe("");
    });
  });

  // ==========================================================================
  // getTenantId
  // ==========================================================================
  describe("getTenantId", () => {
    it("returns the tenant ID when present", async () => {
      mockedHeaders.mockResolvedValueOnce(
        createMockHeaders({
          "x-tenant-id": "org-789",
        }),
      );

      const id = await getTenantId();

      expect(id).toBe("org-789");
    });

    it("returns null when tenant ID is not present", async () => {
      mockedHeaders.mockResolvedValueOnce(
        createMockHeaders({
          "x-tenant-id": null,
        }),
      );

      const id = await getTenantId();

      expect(id).toBeNull();
    });

    it("returns empty string when header value is empty", async () => {
      mockedHeaders.mockResolvedValueOnce(
        createMockHeaders({
          "x-tenant-id": "",
        }),
      );

      const id = await getTenantId();

      expect(id).toBe("");
    });

    it("handles UUID format tenant IDs", async () => {
      mockedHeaders.mockResolvedValueOnce(
        createMockHeaders({
          "x-tenant-id": "550e8400-e29b-41d4-a716-446655440000",
        }),
      );

      const id = await getTenantId();

      expect(id).toBe("550e8400-e29b-41d4-a716-446655440000");
    });
  });

  // ==========================================================================
  // requireTenantContext
  // ==========================================================================
  describe("requireTenantContext", () => {
    it("returns required tenant context when all headers are present", async () => {
      mockedHeaders.mockResolvedValueOnce(
        createMockHeaders({
          "x-tenant-slug": "ogs-required",
          "x-tenant-id": "org-required",
          "x-tenant-name": "Required OGS",
        }),
      );

      const context = await requireTenantContext();

      expect(context).toEqual({
        slug: "ogs-required",
        id: "org-required",
        name: "Required OGS",
        isMultiTenant: true,
      });
    });

    it("returns required tenant context with empty string defaults for missing id/name", async () => {
      mockedHeaders.mockResolvedValueOnce(
        createMockHeaders({
          "x-tenant-slug": "ogs-partial",
          "x-tenant-id": null,
          "x-tenant-name": null,
        }),
      );

      const context = await requireTenantContext();

      expect(context).toEqual({
        slug: "ogs-partial",
        id: "",
        name: "",
        isMultiTenant: true,
      });
    });

    it("throws error when slug is not present", async () => {
      mockedHeaders.mockResolvedValueOnce(
        createMockHeaders({
          "x-tenant-slug": null,
          "x-tenant-id": "org-123",
          "x-tenant-name": "Test OGS",
        }),
      );

      await expect(requireTenantContext()).rejects.toThrow(
        "This page requires a tenant context (subdomain)",
      );
    });

    it("throws error when not in multi-tenant context", async () => {
      mockedHeaders.mockResolvedValueOnce(
        createMockHeaders({
          "x-tenant-slug": null,
          "x-tenant-id": null,
          "x-tenant-name": null,
        }),
      );

      await expect(requireTenantContext()).rejects.toThrow(
        "This page requires a tenant context (subdomain)",
      );
    });

    it("throws specific error message for missing tenant context", async () => {
      mockedHeaders.mockResolvedValueOnce(
        createMockHeaders({
          "x-tenant-slug": null,
        }),
      );

      try {
        await requireTenantContext();
        expect.fail("Expected requireTenantContext to throw");
      } catch (error) {
        expect(error).toBeInstanceOf(Error);
        expect((error as Error).message).toBe(
          "This page requires a tenant context (subdomain)",
        );
      }
    });

    it("handles empty slug as valid (edge case)", async () => {
      // Empty string is truthy in the !== null check but falsy in the !context.slug check
      mockedHeaders.mockResolvedValueOnce(
        createMockHeaders({
          "x-tenant-slug": "",
          "x-tenant-id": "org-123",
          "x-tenant-name": "Test",
        }),
      );

      // Empty slug should trigger error since !context.slug is true for empty string
      await expect(requireTenantContext()).rejects.toThrow(
        "This page requires a tenant context (subdomain)",
      );
    });
  });

  // ==========================================================================
  // Integration-like tests
  // ==========================================================================
  describe("integration scenarios", () => {
    it("simulates main domain request (no tenant headers)", async () => {
      mockedHeaders.mockResolvedValue(
        createMockHeaders({
          "x-tenant-slug": null,
          "x-tenant-id": null,
          "x-tenant-name": null,
        }),
      );

      const context = await getTenantContext();
      const isTenant = await isTenantRequest();

      expect(context.isMultiTenant).toBe(false);
      expect(isTenant).toBe(false);
      expect(context.slug).toBeNull();
    });

    it("simulates subdomain request (with tenant headers)", async () => {
      const tenantHeaders = {
        "x-tenant-slug": "ogs-integration",
        "x-tenant-id": "org-integration-id",
        "x-tenant-name": "Integration OGS",
      };

      // Mock for multiple calls
      mockedHeaders.mockResolvedValue(createMockHeaders(tenantHeaders));

      const context = await getTenantContext();
      const isTenant = await isTenantRequest();
      const slug = await getTenantSlug();
      const id = await getTenantId();

      expect(context.isMultiTenant).toBe(true);
      expect(isTenant).toBe(true);
      expect(slug).toBe("ogs-integration");
      expect(id).toBe("org-integration-id");
    });

    it("handles concurrent calls with different tenant contexts", async () => {
      // First call - tenant A
      mockedHeaders.mockResolvedValueOnce(
        createMockHeaders({
          "x-tenant-slug": "tenant-a",
          "x-tenant-id": "id-a",
          "x-tenant-name": "Tenant A",
        }),
      );

      // Second call - tenant B
      mockedHeaders.mockResolvedValueOnce(
        createMockHeaders({
          "x-tenant-slug": "tenant-b",
          "x-tenant-id": "id-b",
          "x-tenant-name": "Tenant B",
        }),
      );

      const contextA = await getTenantContext();
      const contextB = await getTenantContext();

      expect(contextA.slug).toBe("tenant-a");
      expect(contextB.slug).toBe("tenant-b");
    });
  });
});
