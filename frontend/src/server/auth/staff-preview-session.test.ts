import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { tenantAuthConfig as authConfig } from "./tenant-config";
import { _resetRefreshState } from "./shared";
import type { Session, User } from "next-auth";
import type { JWT } from "next-auth/jwt";

// Mock ~/env
vi.mock("~/env", () => ({
  env: {
    API_URL: "http://server:8080",
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
    AUTH_JWT_EXPIRY: "15m",
    AUTH_JWT_REFRESH_EXPIRY: "1h",
    NEXTAUTH_SECRET: "test-auth-secret-with-sufficient-entropy",
  },
}));

const mockFetch = vi.fn();

function encodeJwt(payload: Record<string, unknown>): string {
  const header = Buffer.from(
    JSON.stringify({ alg: "HS256", typ: "JWT" }),
  ).toString("base64url");
  const body = Buffer.from(JSON.stringify(payload)).toString("base64url");
  return `${header}.${body}.test`;
}

function previewJwt(overrides: Record<string, unknown> = {}): string {
  return encodeJwt({
    id: 42,
    sub: "erika@example.com",
    first_name: "Erika",
    last_name: "Beispiel",
    roles: ["user"],
    permissions: ["students:read"],
    is_admin: false,
    tenant_id: 7,
    read_only: true,
    acting_admin_id: 1,
    ...overrides,
  });
}

function adminJwt(): string {
  return encodeJwt({
    id: 1,
    sub: "admin@example.com",
    first_name: "Anna",
    last_name: "Admin",
    roles: ["admin"],
    permissions: ["admin:*"],
    is_admin: true,
    tenant_id: 7,
  });
}

function refreshJwt(token: string): string {
  return encodeJwt({
    id: 1,
    token,
    exp: Math.floor(Date.now() / 1000) + 60 * 60,
  });
}

function adminToken(): Record<string, unknown> {
  return {
    id: "1",
    email: "admin@example.com",
    name: "Anna Admin",
    token: adminJwt(),
    refreshToken: refreshJwt("rt-1"),
    roles: ["admin"],
    permissions: ["admin:*"],
    isAdmin: true,
    tenantId: 7,
    tokenExpiry: Date.now() + 10 * 60 * 1000,
    refreshTokenExpiry: Date.now() + 60 * 60 * 1000,
  };
}

function callJwt(
  token: Record<string, unknown>,
  session?: Record<string, unknown>,
) {
  return authConfig.callbacks?.jwt?.({
    token,
    user: undefined as unknown as User,
    account: null,
    profile: undefined,
    trigger: "update",
    isNewUser: false,
    session,
  });
}

async function startPreview(token: Record<string, unknown>) {
  const started = await callJwt(token, {
    previewStart: {
      accessToken: previewJwt(),
      expiresIn: 900,
      targetAccountId: 42,
      targetName: "Erika Beispiel",
    },
  });
  if (!started) throw new Error("jwt callback returned nothing");
  return started as unknown as Record<string, unknown>;
}

describe("JWT callback — staff preview (#2893)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal("fetch", mockFetch);
    _resetRefreshState();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("previewStart swaps onto the preview token and parks the admin state", async () => {
    const token = adminToken();
    const parkedAccess = token.token;
    const result = await startPreview(token);

    expect(result.token).not.toBe(parkedAccess);
    expect(result.previewAdminToken).toBe(parkedAccess);
    expect(result.previewTargetAccountId).toBe(42);
    expect(result.previewTargetName).toBe("Erika Beispiel");
    // the session now carries the TARGET's identity and permissions
    expect(result.id).toBe("42");
    expect(result.email).toBe("erika@example.com");
    expect(result.roles).toEqual(["user"]);
    expect(result.permissions).toEqual(["students:read"]);
    expect(result.isAdmin).toBe(false);
    expect(result.name).toBe("Erika Beispiel");
    // the admin's refresh token stays in place — none exists for the target
    expect(result.refreshToken).toBe(token.refreshToken);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("previewStart refuses a token without the read_only claim", async () => {
    const token = adminToken();
    const before = token.token;
    const result = await callJwt(token, {
      previewStart: {
        accessToken: previewJwt({ read_only: undefined }),
        expiresIn: 900,
        targetAccountId: 42,
        targetName: "Erika Beispiel",
      },
    });

    expect(result?.token).toBe(before);
    expect(result?.previewTargetAccountId).toBeUndefined();
  });

  it("previewEnd restores the parked admin state", async () => {
    const token = adminToken();
    const parkedAccess = token.token;
    const started = await startPreview(token);

    const ended = (await callJwt(started, {
      previewEnd: true,
    })) as unknown as Record<string, unknown>;

    expect(ended.token).toBe(parkedAccess);
    expect(ended.id).toBe("1");
    expect(ended.email).toBe("admin@example.com");
    expect(ended.roles).toEqual(["admin"]);
    expect(ended.permissions).toEqual(["admin:*"]);
    expect(ended.isAdmin).toBe(true);
    expect(ended.previewTargetAccountId).toBeUndefined();
    expect(ended.previewAdminToken).toBeUndefined();
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("keeps a fresh preview token untouched (no fetch)", async () => {
    const started = await startPreview(adminToken());

    const result = (await callJwt(started)) as unknown as Record<
      string,
      unknown
    >;

    expect(mockFetch).not.toHaveBeenCalled();
    expect(result.token).toBe(started.token);
    expect(result.previewTargetAccountId).toBe(42);
  });

  it("re-mints an expiring preview token with the fresh admin token", async () => {
    const started = await startPreview(adminToken());
    started.tokenExpiry = Date.now() + 60 * 1000; // inside the buffer
    const remintedJwt = previewJwt({ first_name: "Erika" });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: () =>
        Promise.resolve({
          access_token: remintedJwt,
          expires_in: 900,
          target_name: "Erika Beispiel",
        }),
    });

    const result = (await callJwt(started)) as unknown as Record<
      string,
      unknown
    >;

    expect(mockFetch).toHaveBeenCalledTimes(1);
    const [url, init] = mockFetch.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("http://server:8080/auth/staff-preview");
    expect((init.headers as Record<string, string>).Authorization).toBe(
      `Bearer ${started.previewAdminToken as string}`,
    );
    expect(result.token).toBe(remintedJwt);
    expect(result.previewTargetAccountId).toBe(42);
    expect((result.tokenExpiry as number) > Date.now()).toBe(true);
  });

  it("refreshes the admin token first when it is stale, then re-mints", async () => {
    const started = await startPreview(adminToken());
    started.tokenExpiry = Date.now() + 60 * 1000;
    started.previewAdminTokenExpiry = Date.now() - 1000; // stale admin token

    const newAdminAccess = adminJwt();
    mockFetch
      .mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            access_token: newAdminAccess,
            refresh_token: refreshJwt("rt-2"),
          }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: () =>
          Promise.resolve({
            access_token: previewJwt(),
            expires_in: 900,
            target_name: "Erika Beispiel",
          }),
      });

    const result = (await callJwt(started)) as unknown as Record<
      string,
      unknown
    >;

    expect(mockFetch).toHaveBeenCalledTimes(2);
    const [refreshUrl] = mockFetch.mock.calls[0] as [string];
    const [mintUrl] = mockFetch.mock.calls[1] as [string];
    expect(refreshUrl).toBe("http://server:8080/auth/refresh");
    expect(mintUrl).toBe("http://server:8080/auth/staff-preview");
    expect(result.previewAdminToken).toBe(newAdminAccess);
    expect(result.previewTargetAccountId).toBe(42);
    // the rotated refresh token replaced the old one
    expect(result.refreshToken).not.toBe(adminToken().refreshToken);
  });

  it("ends the preview when the re-mint is rejected (target no longer previewable)", async () => {
    const token = adminToken();
    const parkedAccess = token.token;
    const started = await startPreview(token);
    started.tokenExpiry = Date.now() + 60 * 1000;

    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 403,
      json: () => Promise.resolve({}),
    });

    const result = (await callJwt(started)) as unknown as Record<
      string,
      unknown
    >;

    expect(result.token).toBe(parkedAccess);
    expect(result.previewTargetAccountId).toBeUndefined();
    expect(result.roles).toEqual(["admin"]);
  });

  it("terminates the whole session when the admin refresh is rejected", async () => {
    const started = await startPreview(adminToken());
    started.tokenExpiry = Date.now() + 60 * 1000;
    started.previewAdminTokenExpiry = Date.now() - 1000;

    mockFetch.mockResolvedValueOnce({ ok: false, status: 401 });

    const result = (await callJwt(started)) as unknown as Record<
      string,
      unknown
    >;

    expect(result.error).toBe("RefreshTokenError");
  });

  it("keeps the preview on a transient re-mint failure", async () => {
    const started = await startPreview(adminToken());
    started.tokenExpiry = Date.now() + 60 * 1000;
    const previousToken = started.token;

    mockFetch.mockRejectedValueOnce(new Error("network down"));

    const result = (await callJwt(started)) as unknown as Record<
      string,
      unknown
    >;

    expect(result.token).toBe(previousToken);
    expect(result.previewTargetAccountId).toBe(42);
    expect(result.error).toBeUndefined();
  });

  it("session callback exposes the preview state", async () => {
    const started = await startPreview(adminToken());

    const session = authConfig.callbacks?.session?.({
      session: { user: {} } as unknown as Session,
      token: started as unknown as JWT,
    } as never) as Session;

    expect(session.user.isPreview).toBe(true);
    expect(session.user.previewTargetName).toBe("Erika Beispiel");
    expect(session.user.previewTargetAccountId).toBe(42);
    expect(session.user.roles).toEqual(["user"]);
  });

  it("session callback reports no preview on a regular session", () => {
    const session = authConfig.callbacks?.session?.({
      session: { user: {} } as unknown as Session,
      token: adminToken() as unknown as JWT,
    } as never) as Session;

    expect(session.user.isPreview).toBe(false);
    expect(session.user.previewTargetName).toBeUndefined();
  });
});
