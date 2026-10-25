import { beforeEach, describe, expect, it, vi } from "vitest";
import type { User } from "next-auth";

const { mockValidate } = vi.hoisted(() => ({ mockValidate: vi.fn() }));
vi.mock("./token-validation", () => ({ validateSessionToken: mockValidate }));
vi.mock("~/env", () => ({
  env: {
    API_URL: "http://server:8080",
    AUTH_JWT_EXPIRY: "15m",
    AUTH_JWT_REFRESH_EXPIRY: "1h",
    NEXTAUTH_SECRET: "test-auth-secret-with-sufficient-entropy",
  },
}));

import { traegerAuthConfig } from "./traeger-config";

type Authorize = (
  credentials: Record<string, string>,
  request?: Request,
) => Promise<User | null>;

function authorize(): Authorize {
  const provider = traegerAuthConfig.providers[0] as unknown as {
    options: { authorize: Authorize };
  };
  return provider.options.authorize;
}

describe("traegerAuthConfig", () => {
  beforeEach(() => {
    mockValidate.mockReset();
  });

  it("keeps the Träger session in its own host-only cookies", () => {
    const { cookies } = traegerAuthConfig;
    expect(cookies.sessionToken.name).toBe("traeger.session-token");
    expect(cookies.callbackUrl.name).toBe("traeger.callback-url");
    expect(cookies.csrfToken.name).toBe("traeger.csrf-token");
    for (const cookie of [
      cookies.sessionToken,
      cookies.callbackUrl,
      cookies.csrfToken,
    ]) {
      expect(cookie.options).not.toHaveProperty("domain");
    }
    expect(cookies.sessionToken.options.sameSite).toBe("strict");
    expect(cookies.sessionToken.options.httpOnly).toBe(true);
  });

  it("serves its own auth base path and login page", () => {
    expect(traegerAuthConfig.basePath).toBe("/api/traeger/auth");
    expect(traegerAuthConfig.pages.signIn).toBe("/traeger/login");
    const provider = traegerAuthConfig.providers[0] as unknown as {
      options: { id: string };
    };
    expect(provider.options.id).toBe("traeger-credentials");
  });

  it("accepts only a backend-verified org-scope token pair", async () => {
    mockValidate.mockResolvedValue({
      id: 41,
      sub: "buero@traeger.example",
      scope: "org",
      org_id: 7,
      roles: ["administration"],
      permissions: ["traeger_overview:read"],
      exp: Math.floor(Date.now() / 1000) + 900,
    });

    const user = await authorize()({
      internalRefresh: "true",
      token: "access",
      refreshToken: "refresh",
    });

    expect(mockValidate).toHaveBeenCalledWith("access", "org", "refresh");
    expect(user).toMatchObject({
      id: "41",
      scope: "org",
      orgId: 7,
      roles: ["administration"],
      permissions: ["traeger_overview:read"],
    });
  });

  it("refuses a token the backend does not confirm as org scope", async () => {
    mockValidate.mockResolvedValue(null);

    await expect(
      authorize()({
        internalRefresh: "true",
        token: "school-token",
        refreshToken: "refresh",
      }),
    ).resolves.toBeNull();
  });

  it("never takes a password directly", async () => {
    await expect(
      authorize()({ email: "buero@traeger.example", password: "secret" }),
    ).resolves.toBeNull();
    expect(mockValidate).not.toHaveBeenCalled();
  });
});
