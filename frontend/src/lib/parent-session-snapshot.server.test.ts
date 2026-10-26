import { encode } from "next-auth/jwt";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { requestHeaders } = vi.hoisted(() => ({
  requestHeaders: { current: new Headers() },
}));

vi.mock("next/headers", () => ({
  headers: vi.fn(async () => requestHeaders.current),
}));

import { readParentSessionSnapshot } from "./parent-session-snapshot.server";

// The secret the setup-common env mock hands to every reader.
const SECRET = "test-secret";
const PARENT_COOKIE = "parent.session-token";
const TENANT_COOKIE = "authjs.session-token";

const parentClaims = () => ({
  id: "41",
  name: "Paula Parent",
  email: "paula@example.com",
  token: "backend-access",
  refreshToken: "must-stay-in-the-cookie",
  refreshRecoveryProof: "must-stay-in-the-cookie-too",
  roles: [],
  permissions: [],
  firstName: "Paula",
  isAdmin: false,
  scope: "parent",
  tokenExpiry: Date.now() + 10 * 60_000,
  refreshTokenExpiry: Date.now() + 12 * 60 * 60_000,
});

async function cookieValue(
  claims: Record<string, unknown>,
  salt: string,
): Promise<string> {
  return encode({ token: claims, secret: SECRET, salt });
}

function sendCookies(cookies: Record<string, string>, extra?: HeadersInit) {
  const cookie = Object.entries(cookies)
    .map(([name, value]) => `${name}=${value}`)
    .join("; ");
  requestHeaders.current = new Headers(extra);
  if (cookie) requestHeaders.current.set("cookie", cookie);
}

beforeEach(() => {
  requestHeaders.current = new Headers();
});

describe("readParentSessionSnapshot", () => {
  it("returns the parent session from a valid cookie without refresh secrets", async () => {
    sendCookies({
      [PARENT_COOKIE]: await cookieValue(parentClaims(), PARENT_COOKIE),
    });

    const session = await readParentSessionSnapshot();

    expect(session?.user).toMatchObject({
      id: "41",
      name: "Paula Parent",
      email: "paula@example.com",
      token: "backend-access",
      firstName: "Paula",
      scope: "parent",
    });
    expect(session?.user.tenantId).toBeUndefined();
    const serialized = JSON.stringify(session);
    expect(serialized).toContain("backend-access");
    expect(serialized).not.toContain("must-stay-in-the-cookie");
  });

  it("returns nothing once the access token has expired", async () => {
    sendCookies({
      [PARENT_COOKIE]: await cookieValue(
        { ...parentClaims(), tokenExpiry: Date.now() - 1 },
        PARENT_COOKIE,
      ),
    });

    await expect(readParentSessionSnapshot()).resolves.toBeNull();
  });

  it("returns nothing once the refresh session has expired", async () => {
    sendCookies({
      [PARENT_COOKIE]: await cookieValue(
        { ...parentClaims(), refreshTokenExpiry: Date.now() - 1 },
        PARENT_COOKIE,
      ),
    });

    await expect(readParentSessionSnapshot()).resolves.toBeNull();
  });

  it("leaves a token inside the refresh window to the session route", async () => {
    sendCookies({
      [PARENT_COOKIE]: await cookieValue(
        { ...parentClaims(), tokenExpiry: Date.now() + 5 * 60_000 },
        PARENT_COOKIE,
      ),
    });

    await expect(readParentSessionSnapshot()).resolves.toBeNull();
  });

  it("returns nothing for a terminal refresh error", async () => {
    sendCookies({
      [PARENT_COOKIE]: await cookieValue(
        { ...parentClaims(), error: "RefreshTokenError" },
        PARENT_COOKIE,
      ),
    });

    await expect(readParentSessionSnapshot()).resolves.toBeNull();
  });

  it("returns nothing without a cookie", async () => {
    await expect(readParentSessionSnapshot()).resolves.toBeNull();
  });

  it("returns nothing without the parent cookie", async () => {
    sendCookies({ "parent.callback-url": "%2F" });

    await expect(readParentSessionSnapshot()).resolves.toBeNull();
  });

  it("ignores another portal's session cookie", async () => {
    sendCookies({
      [TENANT_COOKIE]: await cookieValue(
        { ...parentClaims(), scope: "", tenantId: 3 },
        TENANT_COOKIE,
      ),
    });

    await expect(readParentSessionSnapshot()).resolves.toBeNull();
  });

  it("does not decrypt another portal's cookie under the parent name", async () => {
    sendCookies({
      [PARENT_COOKIE]: await cookieValue(parentClaims(), TENANT_COOKIE),
    });

    await expect(readParentSessionSnapshot()).resolves.toBeNull();
  });

  it("rejects a token without the parent scope", async () => {
    sendCookies({
      [PARENT_COOKIE]: await cookieValue(
        { ...parentClaims(), scope: "platform" },
        PARENT_COOKIE,
      ),
    });

    await expect(readParentSessionSnapshot()).resolves.toBeNull();
  });

  it("rejects a token without an access token", async () => {
    sendCookies({
      [PARENT_COOKIE]: await cookieValue(
        { ...parentClaims(), token: "" },
        PARENT_COOKIE,
      ),
    });

    await expect(readParentSessionSnapshot()).resolves.toBeNull();
  });

  it("does not take the session from an Authorization header", async () => {
    sendCookies(
      { "parent.callback-url": "%2F" },
      {
        authorization: `Bearer ${await cookieValue(parentClaims(), PARENT_COOKIE)}`,
      },
    );

    await expect(readParentSessionSnapshot()).resolves.toBeNull();
  });
});
