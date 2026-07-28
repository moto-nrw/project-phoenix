import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  AccountTenantAccessApiError,
  accountTenantAccessService,
} from "./account-tenant-access-api";

const mockFetch = vi.fn();

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("accountTenantAccessService", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal("fetch", mockFetch);
  });

  it("maps backend snake_case rows to the frontend shape", async () => {
    mockFetch.mockResolvedValue(
      jsonResponse({
        data: [
          {
            tenant_id: 7,
            school_name: "OGS Nord",
            school_slug: "ogs-nord",
            school_active: true,
            organization_id: 3,
            organization_name: "Träger Köln",
            status: "active",
            activated_at: "2026-07-01T10:00:00Z",
            deactivated_at: null,
            has_person: true,
            has_staff: false,
            roles: [{ id: 1, name: "admin", base_role: null }],
          },
        ],
      }),
    );

    const result = await accountTenantAccessService.list("42");

    expect(mockFetch).toHaveBeenCalledWith(
      "/api/operator/accounts/42/tenants",
      expect.objectContaining({ credentials: "include" }),
    );
    expect(result).toEqual([
      {
        tenantId: "7",
        schoolName: "OGS Nord",
        schoolSlug: "ogs-nord",
        schoolActive: true,
        organizationId: "3",
        organizationName: "Träger Köln",
        status: "active",
        activatedAt: "2026-07-01T10:00:00Z",
        deactivatedAt: null,
        hasPerson: true,
        hasStaff: false,
        roles: [{ id: "1", name: "admin", baseRole: null }],
      },
    ]);
  });

  it("treats an unknown status as no access and missing roles as empty", async () => {
    mockFetch.mockResolvedValue(
      jsonResponse({
        data: [
          {
            tenant_id: 7,
            school_name: "OGS Nord",
            school_slug: "ogs-nord",
            school_active: true,
            organization_id: 3,
            organization_name: "Träger",
            status: "something-new",
            has_person: false,
            has_staff: false,
          },
        ],
      }),
    );

    const [entry] = await accountTenantAccessService.list("42");

    expect(entry?.status).toBe("inactive");
    expect(entry?.roles).toEqual([]);
  });

  it("sends numeric ids on grant", async () => {
    mockFetch.mockResolvedValue(jsonResponse({ data: [] }));

    await accountTenantAccessService.grant("42", {
      schoolId: "7",
      roleId: "1",
      firstName: "Ada",
      lastName: "Lovelace",
    });

    expect(mockFetch).toHaveBeenCalledWith(
      "/api/operator/accounts/42/tenants",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          school_id: 7,
          role_id: 1,
          first_name: "Ada",
          last_name: "Lovelace",
        }),
      }),
    );
  });

  it("loads assignable roles for the selected school", async () => {
    mockFetch.mockResolvedValue(
      jsonResponse({ data: [{ id: 4, name: "teamleitung" }] }),
    );

    await expect(
      accountTenantAccessService.listAssignableRoles("42", "7"),
    ).resolves.toEqual([{ id: "4", name: "teamleitung" }]);

    expect(mockFetch).toHaveBeenCalledWith(
      "/api/operator/accounts/42/tenants/7/roles",
      expect.objectContaining({ credentials: "include" }),
    );
  });

  it("throws the backend message so the modal can show the real reason", async () => {
    mockFetch.mockResolvedValue(
      jsonResponse(
        { message: "Diese Rolle existiert an der Zielschule nicht" },
        400,
      ),
    );

    await expect(
      accountTenantAccessService.updateRole("42", "7", "1"),
    ).rejects.toThrowError(
      new AccountTenantAccessApiError(
        "Diese Rolle existiert an der Zielschule nicht",
        400,
      ),
    );
  });

  it("falls back to the error field when the payload has no message", async () => {
    mockFetch.mockResolvedValue(jsonResponse({ error: "Unauthorized" }, 401));

    await expect(accountTenantAccessService.list("42")).rejects.toThrowError(
      new AccountTenantAccessApiError("Unauthorized", 401),
    );
  });

  it("stays usable when the error body is not JSON", async () => {
    mockFetch.mockResolvedValue(
      new Response("<html>gateway timeout</html>", { status: 504 }),
    );

    await expect(accountTenantAccessService.list("42")).rejects.toThrowError(
      new AccountTenantAccessApiError(
        "Die Anfrage ist fehlgeschlagen (504).",
        504,
      ),
    );
  });

  it("treats a missing data envelope as an empty list", async () => {
    mockFetch.mockResolvedValue(jsonResponse({ data: null }));

    await expect(accountTenantAccessService.list("42")).resolves.toEqual([]);
  });

  it("keeps the deactivation timestamp of a revoked access", async () => {
    mockFetch.mockResolvedValue(
      jsonResponse({
        data: [
          {
            tenant_id: 7,
            school_name: "OGS Nord",
            school_slug: "ogs-nord",
            school_active: false,
            organization_id: 3,
            organization_name: "Träger",
            status: "inactive",
            activated_at: "2026-07-01T10:00:00Z",
            deactivated_at: "2026-07-20T09:00:00Z",
            has_person: true,
            has_staff: true,
            roles: [],
          },
        ],
      }),
    );

    const [entry] = await accountTenantAccessService.list("42");

    expect(entry?.status).toBe("inactive");
    expect(entry?.deactivatedAt).toBe("2026-07-20T09:00:00Z");
    expect(entry?.schoolActive).toBe(false);
  });

  it("revokes via DELETE on the school-scoped path", async () => {
    mockFetch.mockResolvedValue(jsonResponse({ data: [] }));

    await accountTenantAccessService.revoke("42", "7");

    expect(mockFetch).toHaveBeenCalledWith(
      "/api/operator/accounts/42/tenants/7",
      expect.objectContaining({ method: "DELETE" }),
    );
  });
});
