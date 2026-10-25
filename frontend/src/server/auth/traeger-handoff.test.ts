import { validateSessionToken } from "./token-validation";
vi.mock("./token-validation", () => ({ validateSessionToken: vi.fn() }));
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { User } from "next-auth";
import { tenantAuthConfig } from "./tenant-config";
import { _resetRefreshState } from "./shared";

vi.mock("~/env", () => ({
  env: {
    API_URL: "http://server:8080",
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
    AUTH_JWT_EXPIRY: "15m",
    AUTH_JWT_REFRESH_EXPIRY: "1h",
    NEXTAUTH_SECRET: "test-auth-secret-with-sufficient-entropy",
    TENANT_DOMAIN: "moto-app.de",
  },
}));

vi.mock("next-auth", () => ({ CredentialsSignin: class extends Error {} }));

const mockRequestHeaders = vi.hoisted(() => vi.fn());
vi.mock("next/headers", () => ({ headers: mockRequestHeaders }));

const mockFetch = vi.fn();

function jwt(payload: Record<string, unknown>): string {
  const encode = (value: unknown) =>
    Buffer.from(JSON.stringify(value)).toString("base64url");
  return `${encode({ alg: "HS256", typ: "JWT" })}.${encode(payload)}.test`;
}

function tenantAuthorize() {
  const provider = tenantAuthConfig.providers.find(
    (p) =>
      typeof p === "object" &&
      p !== null &&
      "id" in p &&
      p.id === "credentials",
  ) as unknown as { options: Record<string, unknown> };
  return provider.options.authorize as (
    credentials: Record<string, string> | undefined,
    request: Request,
  ) => Promise<User | null>;
}

const HANDOFF = jwt({ id: 7, token: "handoff-session", tenant_id: 12 });
const REFRESH = jwt({ id: 7, token: "school-session", tenant_id: 12 });

describe("Absprung from the Träger portal (#4654)", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.stubGlobal("fetch", mockFetch);
    mockRequestHeaders.mockResolvedValue(new Headers());
    _resetRefreshState();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("redeems the hand-off token into the school's own session", async () => {
    const access = jwt({ id: 7, tenant_id: 12, via_traeger: true });
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ access_token: access, refresh_token: REFRESH }),
    });
    vi.mocked(validateSessionToken).mockResolvedValueOnce({
      id: 7,
      sub: "leitung@traeger.test",
      tenant_id: 12,
      roles: ["admin"],
      permissions: ["admin:*"],
      is_admin: true,
      via_traeger: true,
    });

    const user = await tenantAuthorize()(
      { traegerHandoff: HANDOFF },
      new Request("https://am-bach.moto-app.de"),
    );

    expect(user?.tenantId).toBe(12);
    expect(user?.token).toBe(access);
    expect(user?.refreshToken).toBe(REFRESH);
    const [url, init] = mockFetch.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("http://server:8080/auth/refresh");
    const headers = init.headers as Record<string, string>;
    expect(headers.Authorization).toBe(`Bearer ${HANDOFF}`);
    expect(headers["X-Refresh-Recovery-Proof"]).toBeTruthy();
    expect(validateSessionToken).toHaveBeenCalledWith(
      access,
      "tenant",
      REFRESH,
    );
  });

  it("refuses a session that did not come through the Träger", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        access_token: jwt({ id: 7, tenant_id: 12 }),
        refresh_token: REFRESH,
      }),
    });
    vi.mocked(validateSessionToken).mockResolvedValueOnce({
      id: 7,
      sub: "leitung@traeger.test",
      tenant_id: 12,
    });

    const user = await tenantAuthorize()(
      { traegerHandoff: HANDOFF },
      new Request("https://am-bach.moto-app.de"),
    );

    expect(user).toBeNull();
  });

  it("signs nobody in when the backend refuses the hand-off", async () => {
    mockFetch.mockResolvedValueOnce({ ok: false, status: 401 });

    const user = await tenantAuthorize()(
      { traegerHandoff: HANDOFF },
      new Request("https://am-bach.moto-app.de"),
    );

    expect(user).toBeNull();
    expect(validateSessionToken).not.toHaveBeenCalled();
  });
});
