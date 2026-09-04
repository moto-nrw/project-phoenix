import { beforeEach, describe, expect, it, vi } from "vitest";

const { mockGetToken } = vi.hoisted(() => ({ mockGetToken: vi.fn() }));

vi.mock("next-auth/jwt", () => ({ getToken: mockGetToken }));
vi.mock("next/headers", () => ({
  headers: vi.fn(
    async () => new Headers({ cookie: "moto-app-de.session-token=encrypted" }),
  ),
}));
vi.mock("~/env", () => ({
  env: {
    NEXTAUTH_SECRET: "test-secret",
    NEXTAUTH_URL: "https://school.moto-app.de",
  },
}));
vi.mock("~/server/auth/tenant-config", () => ({
  tenantAuthConfig: {
    cookies: { sessionToken: { name: "moto-app-de.session-token" } },
  },
}));
vi.mock("~/server/auth/shared", () => ({
  ACCESS_TOKEN_REFRESH_BUFFER_MS: 5 * 60 * 1000,
}));

import { readTenantSessionSnapshot } from "./tenant-session-snapshot.server";

const validToken = () => ({
  id: "7",
  name: "Ada Admin",
  email: "ada@example.com",
  token: "backend-access",
  refreshToken: "must-not-leave-server-cookie",
  roles: ["admin"],
  permissions: ["admin:*"],
  tenantId: 3,
  orgId: 2,
  tokenExpiry: Date.now() + 10 * 60_000,
  refreshTokenExpiry: Date.now() + 24 * 60 * 60_000,
  exp: Math.floor(Date.now() / 1000) + 3600,
});

beforeEach(() => {
  mockGetToken.mockReset();
});

describe("readTenantSessionSnapshot", () => {
  it("decodes the configured cookie without exposing its refresh token", async () => {
    mockGetToken.mockResolvedValue(validToken());

    const session = await readTenantSessionSnapshot();

    expect(mockGetToken).toHaveBeenCalledWith(
      expect.objectContaining({
        secret: "test-secret",
        secureCookie: true,
        cookieName: "moto-app-de.session-token",
        salt: "moto-app-de.session-token",
      }),
    );
    const requestHeaders = mockGetToken.mock.calls[0]?.[0]?.req
      .headers as Headers;
    expect(requestHeaders.get("cookie")).toContain("encrypted");
    expect(session?.user).toMatchObject({
      id: "7",
      token: "backend-access",
      tenantId: 3,
      roles: ["admin"],
    });
    expect(session?.user.refreshToken).toBeUndefined();
  });

  it("defers to the response-aware route inside the proactive refresh window", async () => {
    mockGetToken.mockResolvedValue({
      ...validToken(),
      tokenExpiry: Date.now() + 5 * 60 * 1000,
    });

    await expect(readTenantSessionSnapshot()).resolves.toBeNull();
  });

  it("rejects an incomplete or terminal token", async () => {
    mockGetToken.mockResolvedValue({
      ...validToken(),
      error: "RefreshTokenError",
    });

    await expect(readTenantSessionSnapshot()).resolves.toBeNull();
  });

  it("defers to the route handler when the refresh session expired", async () => {
    mockGetToken.mockResolvedValue({
      ...validToken(),
      refreshTokenExpiry: Date.now() - 1,
    });

    await expect(readTenantSessionSnapshot()).resolves.toBeNull();
  });
});
