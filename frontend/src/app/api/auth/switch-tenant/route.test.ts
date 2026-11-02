import { beforeEach, afterEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
import { defaultSession } from "~/test/route-test-helpers";

const { mockAuth, setTenantSession, setStaffPreviewSession } = vi.hoisted(
  () => ({
    mockAuth: vi.fn(),
    setTenantSession: vi.fn(),
    setStaffPreviewSession: vi.fn(),
  }),
);
vi.mock("~/server/auth", () => ({
  auth: mockAuth,
  setTenantSession,
  setStaffPreviewSession,
}));
const { POST: switchTenant } = await import("./route");
const { POST: startPreview } = await import("../staff-preview/route");
const backend = vi.fn();
const context = { params: Promise.resolve({}) };
const pair = { access_token: "server-access", refresh_token: "server-refresh" };
const preview = {
  access_token: "server-preview",
  expires_in: 900,
  target_account_id: "42",
  target_name: "Erika",
};
function request(
  path: string,
  origin: string | null = "http://localhost:3000",
) {
  return new NextRequest(`http://localhost:3000/api/auth/${path}`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...(origin === null ? {} : { Origin: origin }),
    },
    body: JSON.stringify(
      path === "switch-tenant"
        ? { tenant_slug: "school-b" }
        : { account_id: "42" },
    ),
  });
}
beforeEach(() => {
  vi.clearAllMocks();
  vi.stubGlobal("fetch", backend);
  mockAuth.mockResolvedValue(defaultSession);
  setTenantSession.mockResolvedValue(undefined);
  setStaffPreviewSession.mockResolvedValue(undefined);
});
afterEach(() => vi.unstubAllGlobals());

describe.each([
  ["switch-tenant", switchTenant, pair, setTenantSession, "/auth/logout"],
  [
    "staff-preview",
    startPreview,
    preview,
    setStaffPreviewSession,
    "/auth/staff-preview/end",
  ],
] as const)(
  "server-only session transition: %s",
  (path, handler, payload, setter, rollbackPath) => {
    it("installs the session on the server and returns only allowlisted metadata", async () => {
      backend.mockResolvedValueOnce(
        Response.json({ ...payload, unexpected_token: "must-not-leak" }),
      );
      const response = await handler(request(path), context);
      expect(response.status).toBe(200);
      expect(backend).toHaveBeenCalledWith(
        `http://server:8080/auth/${path}`,
        expect.objectContaining({
          method: "POST",
          headers: expect.objectContaining({
            Authorization: "Bearer test-token",
          }),
        }),
      );
      expect(setter).toHaveBeenCalledOnce();
      expect(await response.json()).toEqual(
        path === "switch-tenant"
          ? { success: true }
          : {
              expires_in: 900,
              target_account_id: "42",
              target_name: "Erika",
            },
      );
      if (path === "switch-tenant")
        expect(setter).toHaveBeenCalledWith("server-access", "server-refresh");
      else
        expect(setter).toHaveBeenCalledWith({
          accessToken: "server-preview",
          expiresIn: 900,
          targetAccountId: "42",
          targetName: "Erika",
        });
    });
    it.each([
      null,
      "null",
      "https://foreign.test",
      "http://other.localhost:3000",
    ])("rejects origin %s before minting", async (origin) => {
      const response = await handler(request(path, origin), context);
      expect(response.status).toBe(403);
      expect(backend).not.toHaveBeenCalled();
      expect(setter).not.toHaveBeenCalled();
    });
    it("refuses unauthenticated requests", async () => {
      mockAuth.mockResolvedValue(null);
      expect((await handler(request(path), context)).status).toBe(401);
      expect(backend).not.toHaveBeenCalled();
    });
    it.each([401, 403, 429, 503])(
      "preserves backend error bytes and headers (%s)",
      async (status) => {
        const body = JSON.stringify({
          status,
          code:
            status === 401
              ? "identity.mfa_session_required"
              : "general.permission",
          detail: "refused",
        });
        backend.mockResolvedValueOnce(
          new Response(body, {
            status,
            headers: {
              "Content-Type": "application/problem+json",
              "Retry-After": "7",
            },
          }),
        );
        const response = await handler(request(path), context);
        expect(response.status).toBe(status);
        if (status < 500) expect(await response.text()).toBe(body);
        else
          expect(await response.json()).toMatchObject({
            status,
            code: "general.unavailable",
            detail: "Service Unavailable",
          });
        expect(response.headers.get("Retry-After")).toBe("7");
        expect(setter).not.toHaveBeenCalled();
      },
    );
    it.each([false, true])(
      "rolls back a failed session install without leaking tokens, rollback failure=%s",
      async (rollbackFails) => {
        backend.mockResolvedValueOnce(Response.json(payload));
        if (rollbackFails) backend.mockRejectedValueOnce(new Error("offline"));
        else backend.mockResolvedValueOnce(new Response(null, { status: 204 }));
        setter.mockRejectedValueOnce(new Error("session refused"));
        const response = await handler(request(path), context);
        expect(response.status).toBe(500);
        const body = await response.text();
        expect(body).toContain("general.server");
        for (const value of [
          "server-access",
          "server-refresh",
          "server-preview",
        ])
          expect(body).not.toContain(value);
        expect(backend).toHaveBeenNthCalledWith(
          2,
          `http://server:8080${rollbackPath}`,
          expect.objectContaining(
            path === "switch-tenant"
              ? {
                  headers: expect.objectContaining({
                    Authorization: "Bearer server-refresh",
                  }),
                }
              : { body: JSON.stringify({ preview_token: "server-preview" }) },
          ),
        );
      },
    );
  },
);

it("refuses a nested preview before minting another audited token", async () => {
  mockAuth.mockResolvedValue({
    ...defaultSession,
    user: { ...defaultSession.user, isPreview: true },
  });
  expect((await startPreview(request("staff-preview"), context)).status).toBe(
    409,
  );
  expect(backend).not.toHaveBeenCalled();
});

it("uses the proxy-owned browser origin behind an internal rewrite", async () => {
  const rewritten = new NextRequest(
    "http://localhost:4000/api/auth/switch-tenant",
    {
      method: "POST",
      headers: {
        Origin: "http://school.localhost:3000",
        "x-moto-original-host": "school.localhost:3000",
        "x-moto-original-proto": "http",
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ tenant_slug: "school-b" }),
    },
  );
  backend.mockResolvedValueOnce(Response.json(pair));
  expect((await switchTenant(rewritten, context)).status).toBe(200);
  expect(setTenantSession).toHaveBeenCalledWith(
    pair.access_token,
    pair.refresh_token,
  );
});

it("rejects a cross-site fetch even if its Origin header claims the target", async () => {
  const crossSite = request("switch-tenant");
  crossSite.headers.set("Sec-Fetch-Site", "cross-site");
  expect((await switchTenant(crossSite, context)).status).toBe(403);
  expect(backend).not.toHaveBeenCalled();
});
