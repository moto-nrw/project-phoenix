import type { Session } from "next-auth";
import { encode } from "next-auth/jwt";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { env } from "~/env";
import type { SessionFamilyState } from "~/server/auth/token-validation";

const { requestHeaders } = vi.hoisted(() => ({
  requestHeaders: { current: new Headers() },
}));

vi.mock("next/headers", () => ({
  headers: vi.fn(async () => requestHeaders.current),
}));
const { familyState } = vi.hoisted(() => ({
  familyState: vi.fn<() => Promise<SessionFamilyState>>(),
}));

// #5399: a snapshot needs a family Identity & Access confirms live.
vi.mock("~/server/auth/token-validation", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/server/auth/token-validation")>()),
  sessionFamilyState: familyState,
}));

const { backendStatus } = vi.hoisted(() => ({
  backendStatus: { current: 401 },
}));
vi.mock("~/lib/backend-client.server", () => ({
  sendBackend: vi.fn(
    async () =>
      new Response(JSON.stringify({ code: "auth.invalid_token" }), {
        status: backendStatus.current,
      }),
  ),
}));

import { PORTALS as PORTAL_TABLE } from "~/server/auth/portals";
import {
  _resetRefreshState,
  rotateRefreshToken,
} from "~/server/auth/token-refresh";
import {
  readPortalSessionSnapshot,
  type PortalKey,
} from "./portal-session-snapshot.server";

const snapshotOf = (key: PortalKey) => async (): Promise<Session | null> =>
  (await readPortalSessionSnapshot(key))?.session ?? null;
const readTenantSessionSnapshot = snapshotOf("tenant");
const readParentSessionSnapshot = snapshotOf("parent");
const readSchoolSessionSnapshot = snapshotOf("school");
const readCatererSessionSnapshot = snapshotOf("caterer");
const readOperatorSessionSnapshot = snapshotOf("operator");
const readTraegerSessionSnapshot = snapshotOf("traeger");

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
  /** Further scopes a session of this portal may carry. */
  readonly alsoAccepts?: readonly string[];
  /** The validation portal of the family check. */
  readonly validation?: string;
  /** The claims a valid token of this portal carries beyond the account. */
  readonly bound: Record<string, unknown>;
}

const PORTALS: readonly Portal[] = [
  {
    name: "tenant",
    read: readTenantSessionSnapshot,
    cookie: TENANT_COOKIE,
    scope: "",
    // An Absprung out of the Träger portal opens a tenant session.
    alsoAccepts: ["tenant", "org"],
    validation: "tenant",
    bound: { tenantId: 3 },
  },
  {
    name: "parent",
    read: readParentSessionSnapshot,
    cookie: PARENT_COOKIE,
    scope: "parent",
    bound: {},
  },
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
  _resetRefreshState();
  backendStatus.current = 401;
  familyState.mockReset();
  familyState.mockResolvedValue("live");
});

describe.each(PORTALS)("$name session snapshot", (portal) => {
  it.each(["ended", "unknown"] as const)(
    "leaves a session of a %s family to the session route (#5399)",
    async (state) => {
      await sendPortalCookie(portal);
      familyState.mockResolvedValue(state);

      await expect(portal.read()).resolves.toBeNull();
      expect(familyState).toHaveBeenCalledWith(
        ACCESS_TOKEN_PARTS.join("."),
        portal.validation ?? portal.scope,
      );
    },
  );

  // #6187: an unbooked module closes the portal's pages, not its session.
  it.each([
    ["notBooked", true],
    ["live", false],
  ] as const)(
    "hydrates a session of a %s family, not booked: %s",
    async (state, notBooked) => {
      await sendPortalCookie(portal);
      familyState.mockResolvedValue(state);

      const snapshot = await readPortalSessionSnapshot(
        portal.name as PortalKey,
      );

      expect(snapshot?.session.user.id).toBe("41");
      expect(snapshot?.notBooked).toBe(notBooked);
    },
  );

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

  // The redirect loop of the #6187/#6188 integration run: the backend
  // refused the cookie's refresh token, the session route and every BFF
  // route read the session as ended, but a snapshot of the still valid
  // access token showed it as signed in. The login page then sent the
  // browser back to the portal, whose first request ended at the login.
  it("leaves a session whose refresh the backend refused to the session route", async () => {
    await sendPortalCookie(portal);
    const refused = await rotateRefreshToken(
      claimsFor(portal) as never,
      PORTAL_TABLE[portal.name as PortalKey].backend.refresh,
    );
    expect(refused).toEqual({ status: "terminal", remembered: false });

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
    const foreign: Record<string, string> = {};
    for (const other of PORTALS.filter((p) => p !== portal)) {
      foreign[other.cookie] = await cookieValue(claimsFor(other), other.cookie);
    }
    sendCookies(foreign);

    await expect(portal.read()).resolves.toBeNull();
  });

  it("does not decrypt another portal's cookie under its own name", async () => {
    sendCookies({
      [portal.cookie]: await cookieValue(
        claimsFor(portal),
        portal.cookie === TENANT_COOKIE ? PARENT_COOKIE : TENANT_COOKIE,
      ),
    });

    await expect(portal.read()).resolves.toBeNull();
  });

  it("rejects a token of another scope under its own cookie", async () => {
    for (const scope of PORTALS.map((p) => p.scope)) {
      if (scope === portal.scope || portal.alsoAccepts?.includes(scope))
        continue;
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

const [tenant, , school, caterer, operator, traeger] = PORTALS as [
  Portal,
  Portal,
  Portal,
  Portal,
  Portal,
  Portal,
];

describe("portal-specific claims", () => {
  // Without TENANT_DOMAIN the tenant keeps Auth.js' default name, which
  // Auth.js prefixes with __Secure- on https.
  it("reads the tenant's __Secure- cookie on https without TENANT_DOMAIN", async () => {
    const mutableEnv = env as { NEXTAUTH_URL: string; TENANT_DOMAIN?: string };
    expect(mutableEnv.TENANT_DOMAIN).toBeUndefined();
    const previous = mutableEnv.NEXTAUTH_URL;
    mutableEnv.NEXTAUTH_URL = "https://moto.example";
    try {
      const secureCookie = `__Secure-${TENANT_COOKIE}`;
      sendCookies({
        [secureCookie]: await cookieValue(claimsFor(tenant), secureCookie),
      });
      await expect(readTenantSessionSnapshot()).resolves.toMatchObject({
        user: { id: "41" },
      });

      sendCookies({
        [TENANT_COOKIE]: await cookieValue(claimsFor(tenant), TENANT_COOKIE),
      });
      await expect(readTenantSessionSnapshot()).resolves.toBeNull();
    } finally {
      mutableEnv.NEXTAUTH_URL = previous;
    }
  });

  it("requires the school of a tenant session", async () => {
    await sendPortalCookie(tenant, { tenantId: undefined });

    await expect(readTenantSessionSnapshot()).resolves.toBeNull();
  });

  it("keeps the tenant access token on the server for the shell preload", async () => {
    await sendPortalCookie(tenant);

    const snapshot = await readPortalSessionSnapshot("tenant");

    expect(snapshot?.accessToken).toBe(ACCESS_TOKEN_PARTS.join("."));
    expect(JSON.stringify(snapshot?.session)).not.toContain(
      ACCESS_TOKEN_PARTS[1],
    );
  });

  it("shows the first render what the session route answers (F3)", async () => {
    // viaTraeger drives the shell's Träger mode, roleIsSystem the role label.
    await sendPortalCookie(tenant, {
      viaTraeger: true,
      roleIsSystem: [true],
      previewTargetAccountId: "9",
      previewTargetName: "Pia Preview",
    });

    const session = await readTenantSessionSnapshot();

    expect(session?.user).toMatchObject({
      viaTraeger: true,
      roleIsSystem: [true],
      isPreview: true,
      previewTargetName: "Pia Preview",
      previewTargetAccountId: "9",
    });
  });

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
