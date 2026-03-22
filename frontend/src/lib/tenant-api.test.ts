import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { TenantSwitchError } from "./tenant-api";

// ============================================================================
// Mocks
// ============================================================================

const { mockSessionFetch } = vi.hoisted(() => ({
  mockSessionFetch: vi.fn(),
}));

vi.mock("./session-cache", () => ({
  sessionFetch: mockSessionFetch,
}));

import {
  resolveTenant,
  listAvailableTenants,
  switchTenant,
} from "./tenant-api";

// ============================================================================
// Tests
// ============================================================================

describe("tenant-api", () => {
  const originalFetch = global.fetch;

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  // --------------------------------------------------------------------------
  // resolveTenant (uses plain fetch, no auth)
  // --------------------------------------------------------------------------

  describe("resolveTenant", () => {
    it("resolves a tenant slug to TenantInfo", async () => {
      const backendData = {
        status: "success",
        data: {
          tenant_id: 1,
          slug: "demo-school",
          name: "Demo School",
          subdomain: "demo",
          organization_id: 10,
          organization_name: "Org A",
          settings: { primaryColor: "#ff0000" },
        },
      };

      vi.mocked(global.fetch).mockResolvedValueOnce(
        new Response(JSON.stringify(backendData), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      );

      const result = await resolveTenant("demo-school");

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/tenant/resolve?slug=demo-school",
      );
      expect(result).toEqual({
        tenantId: 1,
        slug: "demo-school",
        name: "Demo School",
        subdomain: "demo",
        organizationId: 10,
        organizationName: "Org A",
        settings: { primaryColor: "#ff0000" },
      });
    });

    it("returns null when tenant is not found", async () => {
      vi.mocked(global.fetch).mockResolvedValueOnce(
        new Response("Not Found", { status: 404 }),
      );

      const result = await resolveTenant("nonexistent");

      expect(result).toBeNull();
    });

    it("returns null on network error", async () => {
      vi.mocked(global.fetch).mockRejectedValueOnce(new Error("Network error"));

      const result = await resolveTenant("demo-school");

      expect(result).toBeNull();
    });

    it("defaults settings to empty object when not provided", async () => {
      const backendData = {
        status: "success",
        data: {
          tenant_id: 1,
          slug: "demo",
          name: "Demo",
          subdomain: "demo",
          organization_id: 10,
          organization_name: "Org",
          settings: null,
        },
      };

      vi.mocked(global.fetch).mockResolvedValueOnce(
        new Response(JSON.stringify(backendData), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      );

      const result = await resolveTenant("demo");

      expect(result?.settings).toEqual({});
    });
  });

  // --------------------------------------------------------------------------
  // listAllTenants (uses plain fetch, no auth — public tenant selector)
  // --------------------------------------------------------------------------

  describe("listAllTenants", () => {
    it("returns mapped tenants with status ok", async () => {
      const data = {
        data: [
          {
            slug: "school-a",
            name: "School A",
            subdomain: "school-a",
            organization_name: "Org Alpha",
          },
          {
            slug: "school-b",
            name: "School B",
            subdomain: "school-b",
            organization_name: "Org Beta",
          },
        ],
      };

      vi.mocked(global.fetch).mockResolvedValueOnce(
        new Response(JSON.stringify(data), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      );

      const { listAllTenants } = await import("./tenant-api");
      const result = await listAllTenants(1);

      expect(global.fetch).toHaveBeenCalledWith("/api/tenant/list");
      expect(result.status).toBe("ok");
      expect(result.tenants).toHaveLength(2);
      expect(result.tenants[0]).toEqual({
        tenantId: 0,
        slug: "school-a",
        name: "School A",
        subdomain: "school-a",
        organizationId: 0,
        organizationName: "Org Alpha",
        settings: {},
      });
    });

    it("returns error status after exhausting retries on server error", async () => {
      vi.mocked(global.fetch).mockResolvedValue(
        new Response("Server Error", { status: 500 }),
      );

      const { listAllTenants } = await import("./tenant-api");
      const result = await listAllTenants(1);

      expect(result).toEqual({ tenants: [], status: "error" });
    });

    it("returns empty tenants with status ok when data field is missing", async () => {
      vi.mocked(global.fetch).mockResolvedValueOnce(
        new Response(JSON.stringify({}), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      );

      const { listAllTenants } = await import("./tenant-api");
      const result = await listAllTenants(1);

      expect(result).toEqual({ tenants: [], status: "ok" });
    });

    it("returns error status after exhausting retries on network error", async () => {
      vi.mocked(global.fetch).mockRejectedValue(new Error("Network error"));

      const { listAllTenants } = await import("./tenant-api");
      const result = await listAllTenants(1);

      expect(result).toEqual({ tenants: [], status: "error" });
    });

    it("retries on transient failure then succeeds", async () => {
      const data = {
        data: [
          {
            slug: "school-a",
            name: "School A",
            subdomain: "school-a",
            organization_name: "Org Alpha",
          },
        ],
      };

      vi.mocked(global.fetch)
        .mockRejectedValueOnce(new Error("Network error"))
        .mockResolvedValueOnce(
          new Response(JSON.stringify(data), {
            status: 200,
            headers: { "Content-Type": "application/json" },
          }),
        );

      const { listAllTenants } = await import("./tenant-api");
      const result = await listAllTenants(2);

      expect(global.fetch).toHaveBeenCalledTimes(2);
      expect(result.status).toBe("ok");
      expect(result.tenants).toHaveLength(1);
    });
  });

  // --------------------------------------------------------------------------
  // listAvailableTenants (uses sessionFetch, requires auth)
  // --------------------------------------------------------------------------

  describe("listAvailableTenants", () => {
    it("returns mapped tenant list", async () => {
      const data = {
        data: [
          {
            tenant_id: 1,
            slug: "school-a",
            name: "School A",
            subdomain: "school-a",
            organization_id: 10,
            organization_name: "Org Alpha",
          },
          {
            tenant_id: 2,
            slug: "school-b",
            name: "School B",
            subdomain: "school-b",
            organization_id: 20,
            organization_name: "Org Beta",
          },
        ],
      };

      mockSessionFetch.mockResolvedValueOnce(
        new Response(JSON.stringify(data), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      );

      const result = await listAvailableTenants();

      expect(mockSessionFetch).toHaveBeenCalledWith(
        "/api/auth/account-tenants",
      );
      expect(result).toHaveLength(2);
      expect(result[0]).toEqual({
        tenantId: 1,
        slug: "school-a",
        name: "School A",
        subdomain: "school-a",
        organizationId: 10,
        organizationName: "Org Alpha",
        settings: {},
      });
    });

    it("returns empty array on error response", async () => {
      mockSessionFetch.mockResolvedValueOnce(
        new Response("Unauthorized", { status: 401 }),
      );

      const result = await listAvailableTenants();

      expect(result).toEqual([]);
    });

    it("returns empty array when data field is missing", async () => {
      mockSessionFetch.mockResolvedValueOnce(
        new Response(JSON.stringify({}), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      );

      const result = await listAvailableTenants();

      expect(result).toEqual([]);
    });
  });

  // --------------------------------------------------------------------------
  // switchTenant (uses sessionFetch, requires auth)
  // --------------------------------------------------------------------------

  describe("switchTenant", () => {
    it("returns token pair on success", async () => {
      const tokens = {
        access_token: "new-jwt",
        refresh_token: "new-refresh",
      };

      mockSessionFetch.mockResolvedValueOnce(
        new Response(JSON.stringify(tokens), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      );

      const result = await switchTenant("school-b");

      expect(mockSessionFetch).toHaveBeenCalledWith("/api/auth/switch-tenant", {
        method: "POST",
        body: JSON.stringify({ tenant_slug: "school-b" }),
      });
      expect(result).toEqual(tokens);
    });

    it("throws classified access-denied error on tenant authorization failure", async () => {
      mockSessionFetch.mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            status: "error",
            error: "account does not have access to this tenant",
          }),
          {
            status: 401,
            headers: { "Content-Type": "application/json" },
          },
        ),
      );

      await expect(switchTenant("nonexistent")).rejects.toMatchObject({
        name: "TenantSwitchError",
        message: "account does not have access to this tenant",
        status: 401,
        code: "access_denied",
      } satisfies Partial<TenantSwitchError>);
    });

    it("throws generic error when response body is empty", async () => {
      mockSessionFetch.mockResolvedValueOnce(new Response("", { status: 500 }));

      await expect(switchTenant("bad")).rejects.toMatchObject({
        name: "TenantSwitchError",
        message: "Failed to switch tenant",
        status: 500,
        code: "unknown",
      } satisfies Partial<TenantSwitchError>);
    });
  });
});
