import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { TenantSwitchError } from "./tenant-api";

// ============================================================================
// Mocks
// ============================================================================

const { mockSessionFetch, mockClearSessionCache } = vi.hoisted(() => ({
  mockSessionFetch: vi.fn(),
  mockClearSessionCache: vi.fn(),
}));

vi.mock("./session-cache", () => ({
  sessionFetch: mockSessionFetch,
  clearSessionCache: mockClearSessionCache,
}));

import {
  resolveTenant,
  listAvailableTenants,
  switchTenant,
  performTenantSwitch,
  loginImageSrc,
  normalizePresenceMode,
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
  // loginImageSrc (pure helper, no fetch)
  // --------------------------------------------------------------------------

  describe("normalizePresenceMode", () => {
    it("returns 'binary' for exact match", () => {
      expect(normalizePresenceMode("binary")).toBe("binary");
    });

    it("returns 'detailed' for exact match", () => {
      expect(normalizePresenceMode("detailed")).toBe("detailed");
    });

    it("returns 'detailed' for unknown strings (safe default)", () => {
      expect(normalizePresenceMode("futuristic")).toBe("detailed");
    });

    it("returns 'detailed' for undefined/null", () => {
      expect(normalizePresenceMode(undefined)).toBe("detailed");
      expect(normalizePresenceMode(null)).toBe("detailed");
    });

    it("returns 'detailed' for non-string types (defensive)", () => {
      expect(normalizePresenceMode(42)).toBe("detailed");
      expect(normalizePresenceMode({})).toBe("detailed");
    });
  });

  describe("loginImageSrc", () => {
    it("extracts filename from stored path", () => {
      expect(loginImageSrc("/uploads/login-images/123_abcdef.png")).toBe(
        "/api/public/login-image/123_abcdef.png",
      );
    });

    it("handles filename without directory prefix", () => {
      expect(loginImageSrc("myfile.jpg")).toBe(
        "/api/public/login-image/myfile.jpg",
      );
    });

    it("returns base path for empty string", () => {
      expect(loginImageSrc("")).toBe("/api/public/login-image/");
    });

    it("returns base path for trailing slash", () => {
      expect(loginImageSrc("/")).toBe("/api/public/login-image/");
    });
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
          hidden: false,
          settings: { primaryColor: "#ff0000" },
          grade_level_max: 13,
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
        hidden: false,
        settings: { primaryColor: "#ff0000" },
        // Missing presence_mode on a backend response defaults to "detailed"
        // — matches backend's own safe fallback.
        presenceMode: "detailed",
        studentPhotosEnabled: false,
        nfcEnabled: false,
        messagingEnabled: false,
        // Der Team-Chat (#2598) fehlt in dieser Antwort, also bleibt er aus —
        // eine ältere Backend-Antwort darf die Fläche nie aufschalten.
        staffMessagingEnabled: false,
        displayEnabled: false,
        // Older backend responses omit this additive field. Keep the editor
        // available until the server explicitly publishes false.
        careOfferingsEnabled: true,
        attendanceWebEnabled: false,
        // Opt-in feature flag (#1456): missing on older backends means off.
        attendanceLogEnabled: false,
        groupMode: "fixed_groups",
        // Absent operational_overview_scope collapses to the restrictive
        // "own": an older backend must never widen the UI (#2380).
        operationalOverviewScope: "own",
        showTimetableCounts: true,
        waitlistEnabled: true,
        gradeLevelMax: 13,
      });
    });

    it("normalizes explicit enabled and disabled tenant settings", async () => {
      const tenantData = {
        tenant_id: 2,
        slug: "settings-school",
        name: "Settings School",
        subdomain: "settings",
        organization_id: 11,
        organization_name: "Org B",
        settings: {},
      };
      vi.mocked(global.fetch)
        .mockResolvedValueOnce(
          new Response(
            JSON.stringify({
              status: "success",
              data: {
                ...tenantData,
                care_offerings_enabled: true,
                attendance_web_enabled: true,
                group_mode: "open_care",
                operational_overview_scope: "all_staff",
                show_timetable_counts: true,
                waitlist_enabled: true,
              },
            }),
            { status: 200, headers: { "Content-Type": "application/json" } },
          ),
        )
        .mockResolvedValueOnce(
          new Response(
            JSON.stringify({
              status: "success",
              data: {
                ...tenantData,
                care_offerings_enabled: false,
                attendance_web_enabled: false,
                group_mode: "fixed_groups",
                operational_overview_scope: "nonsense_from_a_newer_backend",
                show_timetable_counts: false,
                waitlist_enabled: false,
              },
            }),
            { status: 200, headers: { "Content-Type": "application/json" } },
          ),
        );

      await expect(resolveTenant("settings-school")).resolves.toMatchObject({
        careOfferingsEnabled: true,
        attendanceWebEnabled: true,
        groupMode: "open_care",
        operationalOverviewScope: "all_staff",
        showTimetableCounts: true,
        waitlistEnabled: true,
      });
      await expect(resolveTenant("settings-school")).resolves.toMatchObject({
        careOfferingsEnabled: false,
        attendanceWebEnabled: false,
        groupMode: "fixed_groups",
        // An unrecognised scope collapses to the restrictive "own" — a value
        // the client cannot interpret must never widen the UI (#2380).
        operationalOverviewScope: "own",
        showTimetableCounts: false,
        waitlistEnabled: false,
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

    it("passes through presenceMode=binary when the backend advertises it", async () => {
      const backendData = {
        status: "success",
        data: {
          tenant_id: 2,
          slug: "binary-school",
          name: "Binary School",
          subdomain: "binary",
          organization_id: 11,
          organization_name: "Org B",
          settings: {},
          presence_mode: "binary",
        },
      };
      vi.mocked(global.fetch).mockResolvedValueOnce(
        new Response(JSON.stringify(backendData), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      );
      const result = await resolveTenant("binary-school");
      expect(result?.presenceMode).toBe("binary");
    });

    it("passes through nfcEnabled=true when the backend advertises it", async () => {
      const backendData = {
        status: "success",
        data: {
          tenant_id: 2,
          slug: "nfc-school",
          name: "NFC School",
          subdomain: "nfc",
          organization_id: 11,
          organization_name: "Org B",
          settings: {},
          nfc_enabled: true,
        },
      };
      vi.mocked(global.fetch).mockResolvedValueOnce(
        new Response(JSON.stringify(backendData), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      );
      const result = await resolveTenant("nfc-school");
      expect(result?.nfcEnabled).toBe(true);
    });

    it("passes through displayEnabled=true when the backend advertises it", async () => {
      const backendData = {
        status: "success",
        data: {
          tenant_id: 3,
          slug: "display-school",
          name: "Display School",
          subdomain: "display",
          organization_id: 12,
          organization_name: "Org C",
          settings: {},
          display_enabled: true,
        },
      };
      vi.mocked(global.fetch).mockResolvedValueOnce(
        new Response(JSON.stringify(backendData), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      );
      const result = await resolveTenant("display-school");
      expect(result?.displayEnabled).toBe(true);
    });

    it("maps hidden tenants from resolve response", async () => {
      const backendData = {
        status: "success",
        data: {
          tenant_id: 3,
          slug: "demo-ogs",
          name: "Demo-OGS",
          subdomain: "demo-ogs",
          organization_id: 12,
          organization_name: "Org C",
          hidden: true,
          settings: {},
        },
      };
      vi.mocked(global.fetch).mockResolvedValueOnce(
        new Response(JSON.stringify(backendData), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      );

      const result = await resolveTenant("demo-ogs");

      expect(result?.hidden).toBe(true);
    });

    it("defaults unknown presence_mode values to detailed", async () => {
      const backendData = {
        status: "success",
        data: {
          tenant_id: 3,
          slug: "weird-school",
          name: "Weird",
          subdomain: "weird",
          organization_id: 12,
          organization_name: "Org C",
          settings: {},
          presence_mode: "futuristic", // unknown — normalize to "detailed"
        },
      };
      vi.mocked(global.fetch).mockResolvedValueOnce(
        new Response(JSON.stringify(backendData), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      );
      const result = await resolveTenant("weird-school");
      expect(result?.presenceMode).toBe("detailed");
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
        hidden: false,
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

    it("preserves the school portal handoff code", async () => {
      mockSessionFetch.mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            status: "error",
            error: "school portal accounts must log in at the school portal",
            code: "use_school_portal",
          }),
          {
            status: 403,
            headers: { "Content-Type": "application/json" },
          },
        ),
      );

      await expect(switchTenant("school-b")).rejects.toMatchObject({
        name: "TenantSwitchError",
        status: 403,
        code: "use_school_portal",
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

    it("falls back to raw text when response body is not JSON", async () => {
      mockSessionFetch.mockResolvedValueOnce(
        new Response("plain text error", { status: 503 }),
      );

      await expect(switchTenant("bad")).rejects.toMatchObject({
        name: "TenantSwitchError",
        message: "plain text error",
        status: 503,
        code: "unknown",
      } satisfies Partial<TenantSwitchError>);
    });

    it("uses message field when error field is missing", async () => {
      mockSessionFetch.mockResolvedValueOnce(
        new Response(JSON.stringify({ message: "something went wrong" }), {
          status: 400,
          headers: { "Content-Type": "application/json" },
        }),
      );

      await expect(switchTenant("bad")).rejects.toMatchObject({
        name: "TenantSwitchError",
        message: "something went wrong",
        status: 400,
        code: "unknown",
      } satisfies Partial<TenantSwitchError>);
    });

    it("falls back to raw text when JSON has no error or message field", async () => {
      mockSessionFetch.mockResolvedValueOnce(
        new Response(JSON.stringify({ foo: "bar" }), {
          status: 422,
          headers: { "Content-Type": "application/json" },
        }),
      );

      await expect(switchTenant("bad")).rejects.toMatchObject({
        name: "TenantSwitchError",
        message: '{"foo":"bar"}',
        status: 422,
        code: "unknown",
      } satisfies Partial<TenantSwitchError>);
    });
  });

  // --------------------------------------------------------------------------
  // performTenantSwitch (orchestrates switch + session update + cache clear)
  // --------------------------------------------------------------------------

  describe("performTenantSwitch", () => {
    const tokens = {
      access_token: "new-jwt",
      refresh_token: "new-refresh",
    };

    const mockSignIn = vi.fn().mockResolvedValue(undefined);
    const mockSwrMutate = vi.fn().mockResolvedValue(undefined);

    beforeEach(() => {
      mockSignIn.mockClear();
      mockSwrMutate.mockClear();
      mockClearSessionCache.mockClear();
    });

    it("performs full switch sequence and returns tokens", async () => {
      mockSessionFetch.mockResolvedValueOnce(
        new Response(JSON.stringify(tokens), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      );

      const result = await performTenantSwitch(
        "school-b",
        mockSignIn,
        mockSwrMutate,
      );

      expect(result).toEqual(tokens);

      // Step 1: switch-tenant called
      expect(mockSessionFetch).toHaveBeenCalledWith("/api/auth/switch-tenant", {
        method: "POST",
        body: JSON.stringify({ tenant_slug: "school-b" }),
      });

      // Step 2: signIn called with new tokens
      expect(mockSignIn).toHaveBeenCalledWith("credentials", {
        redirect: false,
        internalRefresh: true,
        token: "new-jwt",
        refreshToken: "new-refresh",
      });

      // Step 3: SWR cache cleared
      expect(mockSwrMutate).toHaveBeenCalledWith(
        expect.any(Function),
        undefined,
        { revalidate: false },
      );

      // Step 4: session cache cleared
      expect(mockClearSessionCache).toHaveBeenCalledTimes(1);
    });

    it("calls steps in correct order", async () => {
      const callOrder: string[] = [];

      mockSessionFetch.mockImplementation(async () => {
        callOrder.push("switchTenant");
        return new Response(JSON.stringify(tokens), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        });
      });

      mockSignIn.mockImplementation(async () => {
        callOrder.push("signIn");
      });

      mockSwrMutate.mockImplementation(async () => {
        callOrder.push("swrMutate");
      });

      mockClearSessionCache.mockImplementation(() => {
        callOrder.push("clearSessionCache");
      });

      await performTenantSwitch("school-b", mockSignIn, mockSwrMutate);

      expect(callOrder).toEqual([
        "switchTenant",
        "signIn",
        "swrMutate",
        "clearSessionCache",
      ]);
    });

    it("propagates TenantSwitchError when backend rejects", async () => {
      mockSessionFetch.mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            error: "account does not have access to this tenant",
          }),
          {
            status: 401,
            headers: { "Content-Type": "application/json" },
          },
        ),
      );

      await expect(
        performTenantSwitch("bad-tenant", mockSignIn, mockSwrMutate),
      ).rejects.toMatchObject({
        name: "TenantSwitchError",
        code: "access_denied",
      });

      // Should NOT have called signIn or cache clearing
      expect(mockSignIn).not.toHaveBeenCalled();
      expect(mockSwrMutate).not.toHaveBeenCalled();
      expect(mockClearSessionCache).not.toHaveBeenCalled();
    });
  });
});
