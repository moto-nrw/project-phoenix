import type { Session } from "next-auth";
import { encode } from "next-auth/jwt";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { requestHeaders } = vi.hoisted(() => ({
  requestHeaders: { current: new Headers() },
}));

vi.mock("next/headers", () => ({
  headers: vi.fn(async () => requestHeaders.current),
}));

import { readCatererSessionSnapshot } from "./caterer-session-snapshot.server";
import { readOperatorSessionSnapshot } from "./operator-session-snapshot.server";
import { readSchoolSessionSnapshot } from "./school-session-snapshot.server";
import { readTraegerSessionSnapshot } from "./traeger-session-snapshot.server";

// The secret the setup-common env mock hands to every reader.
const SECRET = "test-secret";
const TENANT_COOKIE = "authjs.session-token";
const PARENT_COOKIE = "parent.session-token";

// Three dot-separated parts like a backend JWT; none may reach the snapshot.
const ACCESS_TOKEN_PARTS = ["access-header", "access-payload", "access-sig"];
const REFRESH_TOKEN = "must-stay-in-the-cookie";
const RECOVERY_PROOF = "must-stay-in-the-cookie-too";

interface Portal {
  readonly name: string;
  readonly read: () => Promise<Session | null>;
  readonly cookie: string;
  readonly scope: string;
  /** The claims a valid token of this portal carries beyond the account. */
  readonly bound: Record<string, unknown>;
}

const PORTALS: readonly Portal[] = [
  {
    name: "school",
    read: readSchoolSessionSnapshot,
    cookie: "school.session-token",
    scope: "school",
    bound: { tenantId: 3 },
  },
  {
    name: "caterer",
    read: readCatererSessionSnapshot,
    cookie: "caterer.session-token",
    scope: "caterer",
    bound: {},
  },
  {
    name: "operator",
    read: readOperatorSessionSnapshot,
    cookie: "operator.session-token",
    scope: "platform",
    bound: {},
  },
  {
    name: "traeger",
    read: readTraegerSessionSnapshot,
    cookie: "traeger.session-token",
    scope: "org",
    bound: { orgId: 7 },
  },
];

function claimsFor(portal: Portal, overrides: Record<string, unknown> = {}) {
  return {
    id: "41",
    name: "Kim Konto",
    email: "kim@example.com",
    token: ACCESS_TOKEN_PARTS.join("."),
    refreshToken: REFRESH_TOKEN,
    refreshRecoveryProof: RECOVERY_PROOF,
    roles: ["role-a"],
    permissions: ["resource:read"],
    firstName: "Kim",
    isAdmin: false,
    scope: portal.scope,
    tokenExpiry: Date.now() + 10 * 60_000,
    refreshTokenExpiry: Date.now() + 12 * 60 * 60_000,
    ...portal.bound,
    ...overrides,
  };
}

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

async function sendPortalCookie(
  portal: Portal,
  overrides: Record<string, unknown> = {},
) {
  sendCookies({
    [portal.cookie]: await cookieValue(
      claimsFor(portal, overrides),
      portal.cookie,
    ),
  });
}

beforeEach(() => {
  requestHeaders.current = new Headers();
});

describe.each(PORTALS)("$name session snapshot", (portal) => {
  it("returns the session from a valid cookie", async () => {
    await sendPortalCookie(portal);

    const session = await portal.read();

    expect(session?.user).toMatchObject({
      id: "41",
      name: "Kim Konto",
      email: "kim@example.com",
      roles: ["role-a"],
      permissions: ["resource:read"],
      firstName: "Kim",
      isAdmin: false,
      scope: portal.scope,
      ...portal.bound,
    });
  });

  // The snapshot becomes a client prop and is serialized into the HTML and
  // RSC payload (#5005): no token, no part of one, no refresh secret.
  it("carries no token or token part into the client props", async () => {
    await sendPortalCookie(portal);

    const session = await portal.read();

    expect(session?.user.id).toBe("41");
    const serialized = JSON.stringify(session);
    expect(serialized).not.toMatch(/token|recovery/i);
    for (const part of [...ACCESS_TOKEN_PARTS, REFRESH_TOKEN, RECOVERY_PROOF]) {
      expect(serialized).not.toContain(part);
    }
  });

  it("returns nothing once the access token has expired", async () => {
    await sendPortalCookie(portal, { tokenExpiry: Date.now() - 1 });

    await expect(portal.read()).resolves.toBeNull();
  });

  it("leaves a token inside the refresh window to the session route", async () => {
    await sendPortalCookie(portal, { tokenExpiry: Date.now() + 5 * 60_000 });

    await expect(portal.read()).resolves.toBeNull();
  });

  it("returns nothing once the refresh session has expired", async () => {
    await sendPortalCookie(portal, { refreshTokenExpiry: Date.now() - 1 });

    await expect(portal.read()).resolves.toBeNull();
  });

  it("returns nothing for a terminal refresh error", async () => {
    await sendPortalCookie(portal, { error: "RefreshTokenError" });

    await expect(portal.read()).resolves.toBeNull();
  });

  it("returns nothing without a cookie", async () => {
    await expect(portal.read()).resolves.toBeNull();
  });

  it("returns nothing without the portal's session cookie", async () => {
    sendCookies({
      [portal.cookie.replace("session-token", "callback-url")]: "%2F",
    });

    await expect(portal.read()).resolves.toBeNull();
  });

  it("ignores every other portal's session cookie", async () => {
    const foreign: Record<string, string> = {
      [TENANT_COOKIE]: await cookieValue(
        claimsFor(portal, { scope: "", tenantId: 3 }),
        TENANT_COOKIE,
      ),
      [PARENT_COOKIE]: await cookieValue(
        claimsFor(portal, { scope: "parent" }),
        PARENT_COOKIE,
      ),
    };
    for (const other of PORTALS.filter((p) => p !== portal)) {
      foreign[other.cookie] = await cookieValue(claimsFor(other), other.cookie);
    }
    sendCookies(foreign);

    await expect(portal.read()).resolves.toBeNull();
  });

  it("does not decrypt another portal's cookie under its own name", async () => {
    sendCookies({
      [portal.cookie]: await cookieValue(claimsFor(portal), TENANT_COOKIE),
    });

    await expect(portal.read()).resolves.toBeNull();
  });

  it("rejects a token of another scope under its own cookie", async () => {
    for (const scope of ["", "parent", ...PORTALS.map((p) => p.scope)]) {
      if (scope === portal.scope) continue;
      await sendPortalCookie(portal, { scope });

      await expect(portal.read()).resolves.toBeNull();
    }
  });

  it("rejects a token without an access token", async () => {
    await sendPortalCookie(portal, { token: "" });

    await expect(portal.read()).resolves.toBeNull();
  });

  it("rejects a token without an account", async () => {
    await sendPortalCookie(portal, { id: undefined });

    await expect(portal.read()).resolves.toBeNull();
  });

  it("does not take the session from an Authorization header", async () => {
    sendCookies(
      { [portal.cookie.replace("session-token", "callback-url")]: "%2F" },
      {
        authorization: `Bearer ${await cookieValue(claimsFor(portal), portal.cookie)}`,
      },
    );

    await expect(portal.read()).resolves.toBeNull();
  });
});

const [school, caterer, operator, traeger] = PORTALS as [
  Portal,
  Portal,
  Portal,
  Portal,
];

describe("portal-specific claims", () => {
  it("requires the school a school token is bound to", async () => {
    await sendPortalCookie(school, { tenantId: undefined });
    await expect(readSchoolSessionSnapshot()).resolves.toBeNull();

    await sendPortalCookie(school, { tenantId: 0 });
    await expect(readSchoolSessionSnapshot()).resolves.toBeNull();
  });

  it("needs no school for a caterer, whose schools come per request", async () => {
    await sendPortalCookie(caterer);

    const session = await readCatererSessionSnapshot();

    expect(session?.user.scope).toBe("caterer");
    expect(session?.user.tenantId).toBeUndefined();
  });

  it("never falls back to a tenant cookie for the operator", async () => {
    sendCookies({
      [TENANT_COOKIE]: await cookieValue(claimsFor(operator), TENANT_COOKIE),
    });

    await expect(readOperatorSessionSnapshot()).resolves.toBeNull();
  });

  it("requires the Träger and refuses a school on a Träger token", async () => {
    await sendPortalCookie(traeger, { orgId: undefined });
    await expect(readTraegerSessionSnapshot()).resolves.toBeNull();

    await sendPortalCookie(traeger, { orgId: 0 });
    await expect(readTraegerSessionSnapshot()).resolves.toBeNull();

    await sendPortalCookie(traeger, { tenantId: 3 });
    await expect(readTraegerSessionSnapshot()).resolves.toBeNull();
  });
});
