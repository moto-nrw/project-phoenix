import { beforeEach, describe, expect, it, vi } from "vitest";
import type { NextAuthRequest, NextAuthResult } from "next-auth";
import { NextRequest } from "next/server";
import type { ServerSession } from "./shared";

const { lookupTenantMock } = vi.hoisted(() => ({
  lookupTenantMock: vi.fn(),
}));
vi.mock("~/lib/tenant-resolve.server", () => ({
  lookupTenant: lookupTenantMock,
  TENANT_RATE_LIMITED: "rate-limited",
}));
vi.mock("~/env", () => ({ env: { TENANT_DOMAIN: "moto-app.de" } }));
vi.mock("@sentry/nextjs", () => ({ setUser: vi.fn(), setTags: vi.fn() }));

const { rejectForeignTenantSession, tenantSubdomainOfHost } =
  await import("./tenant-host-session");
const { createResponseAwareAuth } = await import("./route-handler");

/** Session of school 2, as the shared cookie holds it after a switch in another tab. */
function sessionOfTenant(tenantId: number | undefined): ServerSession {
  return {
    user: { id: "7", token: "access-b", refreshToken: "refresh-b", tenantId },
    expires: "2099-01-01T00:00:00.000Z",
  } as ServerSession;
}

/**
 * A tenant BFF request as the route sees it: the proxy records the browser's
 * host in X-Moto-Original-Host on every request it passes on.
 */
function request(
  path: string,
  init: { host?: string; method?: string } = {},
): NextRequest {
  const host = init.host ?? "school-a.moto-app.de";
  return new NextRequest(`http://frontend:3000${path}`, {
    method: init.method ?? "GET",
    headers: { "x-moto-original-host": host },
  });
}

beforeEach(() => {
  lookupTenantMock.mockReset();
  lookupTenantMock.mockImplementation(async (subdomain: string) =>
    subdomain === "school-a"
      ? { tenant_id: 1, subdomain: "school-a" }
      : subdomain === "school-b"
        ? { tenant_id: 2, subdomain: "school-b" }
        : null,
  );
});

describe("rejectForeignTenantSession", () => {
  it("refuses a session of another school than the host with the registered code", async () => {
    const response = await rejectForeignTenantSession(
      request("/api/students"),
      sessionOfTenant(2),
    );

    expect(response?.status).toBe(409);
    expect(response?.headers.get("content-type")).toBe(
      "application/problem+json",
    );
    await expect(response?.json()).resolves.toMatchObject({
      status: 409,
      code: "identity.tenant_session_mismatch",
    });
    expect(lookupTenantMock).toHaveBeenCalledWith(
      "school-a",
      expect.any(Object),
    );
  });

  it("lets a session of the host's school through", async () => {
    await expect(
      rejectForeignTenantSession(request("/api/students"), sessionOfTenant(1)),
    ).resolves.toBeNull();
  });

  it("checks against the host the proxy recorded", async () => {
    const response = await rejectForeignTenantSession(
      request("/api/students", { host: "school-b.moto-app.de" }),
      sessionOfTenant(1),
    );

    expect(response?.status).toBe(409);
    expect(lookupTenantMock).toHaveBeenCalledWith(
      "school-b",
      expect.any(Object),
    );
  });

  it("refuses a session on a subdomain no school has", async () => {
    const response = await rejectForeignTenantSession(
      request("/api/students", { host: "gone.moto-app.de" }),
      sessionOfTenant(1),
    );

    expect(response?.status).toBe(409);
  });

  it.each([
    ["the lookup fails", () => Promise.reject(new Error("backend down"))],
    ["the lookup is rate limited", () => Promise.resolve("rate-limited")],
  ])(
    "keeps the request away from the backend when %s",
    async (_case, lookup) => {
      lookupTenantMock.mockImplementation(lookup);

      const response = await rejectForeignTenantSession(
        request("/api/students"),
        sessionOfTenant(1),
      );

      expect(response?.status).toBe(503);
      await expect(response?.json()).resolves.toMatchObject({
        code: "general.unavailable",
      });
    },
  );

  it.each([
    "/api/auth/switch-tenant",
    "/api/auth/staff-preview/end",
    "/api/auth/token",
    "/api/auth/logout",
    "/api/logs",
  ])("lets the session route %s through for the switch back", async (path) => {
    await expect(
      rejectForeignTenantSession(
        request(path, { method: "POST" }),
        sessionOfTenant(2),
      ),
    ).resolves.toBeNull();
    expect(lookupTenantMock).not.toHaveBeenCalled();
  });

  it("does not exempt a data route below a session route's path", async () => {
    const response = await rejectForeignTenantSession(
      request("/api/auth/account-tenants"),
      sessionOfTenant(2),
    );

    expect(response?.status).toBe(409);
  });

  it.each([
    ["no session", null],
    ["a session without a school", sessionOfTenant(undefined)],
  ])("leaves %s to the route", async (_case, session) => {
    await expect(
      rejectForeignTenantSession(request("/api/students"), session),
    ).resolves.toBeNull();
    expect(lookupTenantMock).not.toHaveBeenCalled();
  });

  it.each(["moto-app.de", "operator.moto-app.de", "example.org"])(
    "does not check on %s, which has no school",
    async (host) => {
      await expect(
        rejectForeignTenantSession(
          request("/api/students", { host }),
          sessionOfTenant(2),
        ),
      ).resolves.toBeNull();
      expect(lookupTenantMock).not.toHaveBeenCalled();
    },
  );
});

describe("tenantSubdomainOfHost", () => {
  it.each([
    ["school-a.moto-app.de", "school-a"],
    ["school-a.moto-app.de:443", "school-a"],
    ["moto-app.de", null],
    ["api.moto-app.de", null],
    ["a.b.moto-app.de", null],
    ["school-a.moto-app.de.evil.org", null],
    ["", null],
  ])("%s → %s", (host, expected) => {
    expect(tenantSubdomainOfHost(host)).toBe(expected);
  });
});

/** Auth.js stand-in: hands the route the given session and re-issues its cookie. */
function rawAuthWith(session: ServerSession | null) {
  return vi.fn((handler: unknown) => async (req: NextRequest) => {
    const authRequest = req as NextAuthRequest;
    authRequest.auth = session;
    const response = await (
      handler as (request: NextAuthRequest) => Promise<Response>
    )(authRequest);
    response.headers.append(
      "Set-Cookie",
      "moto-app-de.session-token=same; Path=/; HttpOnly; Secure; SameSite=Lax",
    );
    return response;
  }) as unknown as NextAuthResult["auth"];
}

describe("tenant routes behind the check", () => {
  it.each([
    ["a read", "GET", "/api/students"],
    [
      "a write without an ID of the tab's school",
      "POST",
      "/api/time-tracking/check-in",
    ],
    ["the event stream", "GET", "/api/sse/events"],
  ])(
    "never run %s with another school's session",
    async (_case, method, path) => {
      const backend = vi.fn(async () => Response.json({ ok: true }));
      const { withAuthResponse } = createResponseAwareAuth(
        rawAuthWith(sessionOfTenant(2)),
        "tenant",
        { rejectSession: rejectForeignTenantSession },
      );

      const response = await withAuthResponse(backend)(
        request(path, { method }),
      );

      expect(response.status).toBe(409);
      expect(backend).not.toHaveBeenCalled();
      expect(response.headers.get("cache-control")).toContain("no-store");
    },
  );

  it("runs the route for the host's own school", async () => {
    const backend = vi.fn(async () => Response.json({ ok: true }));
    const { withAuthResponse } = createResponseAwareAuth(
      rawAuthWith(sessionOfTenant(1)),
      "tenant",
      { rejectSession: rejectForeignTenantSession },
    );

    const response = await withAuthResponse(backend)(
      request("/api/time-tracking/check-in", { method: "POST" }),
    );

    expect(response.status).toBe(200);
    expect(backend).toHaveBeenCalledTimes(1);
  });
});
