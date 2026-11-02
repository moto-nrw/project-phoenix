import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
import type { ServerSession } from "./shared";

const { mockResolve } = vi.hoisted(() => ({ mockResolve: vi.fn() }));

vi.mock("~/lib/tenant-resolve.server", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/tenant-resolve.server")>()),
  resolveTenantCached: mockResolve,
}));

const { sessionTenantMismatchResponse } = await import("./tenant-host-check");
const { TenantResolveStatusError } =
  await import("~/lib/tenant-resolve.server");

function request(host: string, path = "/api/students", originalHost?: string) {
  return new NextRequest(`http://${host}${path}`, {
    headers: {
      host,
      "x-forwarded-for": "203.0.113.7",
      ...(originalHost && { "x-moto-original-host": originalHost }),
    },
  });
}

function session(tenantId?: number): ServerSession {
  return {
    expires: "2099-01-01T00:00:00.000Z",
    user: { id: "1", token: "access-token", tenantId },
  } as unknown as ServerSession;
}

beforeEach(() => {
  vi.stubEnv("TENANT_DOMAIN", "localhost");
  mockResolve.mockReset();
  mockResolve.mockImplementation(async (slug: string) => ({
    tenant_id: slug === "schule-a" ? 1 : 2,
  }));
});

afterEach(() => {
  vi.unstubAllEnvs();
});

describe("sessionTenantMismatchResponse", () => {
  it("lets a session of the host's school pass", async () => {
    const result = await sessionTenantMismatchResponse(
      request("schule-a.localhost:3000"),
      session(1),
    );

    expect(result).toBeNull();
    expect(mockResolve).toHaveBeenCalledWith(
      "schule-a",
      expect.objectContaining({ "X-Forwarded-For": "203.0.113.7" }),
    );
  });

  it("refuses a session of another school with its registered code", async () => {
    const result = await sessionTenantMismatchResponse(
      request("schule-a.localhost:3000"),
      session(2),
    );

    expect(result?.status).toBe(409);
    expect(await result?.json()).toMatchObject({
      status: 409,
      code: "identity.session_tenant_mismatch",
    });
  });

  it("uses the proxy-preserved host when Next.js sees an internal host", async () => {
    const result = await sessionTenantMismatchResponse(
      request("next-internal:3000", "/api/students", "schule-a.localhost:3000"),
      session(2),
    );

    expect(result?.status).toBe(409);
    expect(await result?.json()).toMatchObject({
      status: 409,
      code: "identity.session_tenant_mismatch",
    });
    expect(mockResolve).toHaveBeenCalledWith("schule-a", expect.anything());
  });

  it("allows the session of the proxy-preserved host", async () => {
    const result = await sessionTenantMismatchResponse(
      request("next-internal:3000", "/api/students", "schule-a.localhost:3000"),
      session(1),
    );

    expect(result).toBeNull();
    expect(mockResolve).toHaveBeenCalledWith("schule-a", expect.anything());
  });

  it("checks a case-insensitive proxy-preserved school host", async () => {
    const result = await sessionTenantMismatchResponse(
      request("next-internal:3000", "/api/students", "schule-a.LOCALHOST:3000"),
      session(2),
    );

    expect(result?.status).toBe(409);
    expect(await result?.json()).toMatchObject({
      code: "identity.session_tenant_mismatch",
    });
    expect(mockResolve).toHaveBeenCalledWith("schule-a", expect.anything());
  });

  it.each([
    "/api/auth/switch-tenant",
    "/api/auth/token",
    "/api/auth/logout",
    "/api/auth/staff-preview/end",
  ])("lets %s pass so the tab can switch back or sign out", async (path) => {
    const result = await sessionTenantMismatchResponse(
      request("schule-a.localhost:3000", path),
      session(2),
    );

    expect(result).toBeNull();
    expect(mockResolve).not.toHaveBeenCalled();
  });

  it("lets a host of another portal pass", async () => {
    const result = await sessionTenantMismatchResponse(
      request("operator.localhost:3000"),
      session(2),
    );

    expect(result).toBeNull();
    expect(mockResolve).not.toHaveBeenCalled();
  });

  // Path mode (<domain>/<school>/...) names no school in its host; the
  // tenant layout sends it to the subdomain. A tab still on it cannot be
  // bound to its school, so a school's session stops on the bare domain.
  it("refuses a school's session on the bare domain", async () => {
    const result = await sessionTenantMismatchResponse(
      request("localhost:3000"),
      session(2),
    );

    expect(result?.status).toBe(409);
    expect(await result?.json()).toMatchObject({
      code: "identity.session_tenant_mismatch",
    });
    expect(mockResolve).not.toHaveBeenCalled();
  });

  it("refuses a school's session on the proxy-preserved bare domain", async () => {
    const result = await sessionTenantMismatchResponse(
      request("next-internal:3000", "/api/students", "localhost:3000"),
      session(2),
    );

    expect(result?.status).toBe(409);
    expect(await result?.json()).toMatchObject({
      code: "identity.session_tenant_mismatch",
    });
    expect(mockResolve).not.toHaveBeenCalled();
  });

  it.each(["/api/invitations/accept", "/api/logs"])(
    "lets %s, which never acts for the session's school, through",
    async (path) => {
      const result = await sessionTenantMismatchResponse(
        request("localhost:3000", path),
        session(2),
      );

      expect(result).toBeNull();
    },
  );

  it("lets a request without a tenant session pass to the route", async () => {
    const result = await sessionTenantMismatchResponse(
      request("schule-a.localhost:3000"),
      null,
    );

    expect(result).toBeNull();
    expect(mockResolve).not.toHaveBeenCalled();
  });

  it.each([
    [429, 429],
    [404, 503],
    [500, 503],
  ])(
    "refuses the request when the host lookup answers %i",
    async (lookupStatus, status) => {
      mockResolve.mockRejectedValueOnce(
        new TenantResolveStatusError(lookupStatus),
      );

      const result = await sessionTenantMismatchResponse(
        request("schule-a.localhost:3000"),
        session(1),
      );

      expect(result?.status).toBe(status);
    },
  );

  it("refuses the request when the host lookup does not answer", async () => {
    mockResolve.mockRejectedValueOnce(new TypeError("fetch failed"));

    const result = await sessionTenantMismatchResponse(
      request("schule-a.localhost:3000"),
      session(1),
    );

    expect(result?.status).toBe(503);
  });
});
