import { describe, expect, it } from "vitest";
import { toBrowserSession, withBrowserSession } from "./browser-session";
import type { ServerSession } from "./shared";

const serverSession = (): ServerSession => ({
  expires: "2099-01-01T00:00:00.000Z",
  user: {
    id: "7",
    email: "erika@example.com",
    token: "access-token",
    refreshToken: "refresh-token",
    roles: ["admin"],
    tenantId: 3,
    isPreview: false,
  },
});

describe("toBrowserSession (#5005)", () => {
  it("keeps the session's fields and drops both tokens", () => {
    const session = toBrowserSession(serverSession());

    expect(session).toEqual({
      expires: "2099-01-01T00:00:00.000Z",
      user: {
        id: "7",
        email: "erika@example.com",
        roles: ["admin"],
        tenantId: 3,
        isPreview: false,
      },
    });
  });

  it("drops a field outside the allowlist", () => {
    const withExtra = serverSession();
    (withExtra.user as Record<string, unknown>).previewAdminToken = "parked";

    expect(toBrowserSession(withExtra).user).not.toHaveProperty(
      "previewAdminToken",
    );
  });

  it("keeps the error of a stripped session", () => {
    const stripped = {
      ...serverSession(),
      error: "RefreshTokenError" as const,
    };

    expect(toBrowserSession(stripped).error).toBe("RefreshTokenError");
  });
});

describe("withBrowserSession (#5005)", () => {
  const answer = (body: unknown, headers: HeadersInit = {}) =>
    withBrowserSession(
      async (_request: Request) =>
        new Response(JSON.stringify(body), {
          status: 200,
          headers: { "Content-Type": "application/json", ...headers },
        }),
    );

  it("rewrites the session answer and keeps its cookies", async () => {
    const handler = answer(serverSession(), {
      "Set-Cookie": "authjs.session-token=rotated; Path=/; HttpOnly",
    });

    const response = await handler(
      new Request("http://localhost/api/auth/session"),
    );

    const body = await response.text();
    expect(body).not.toContain("access-token");
    expect(body).not.toContain("refresh-token");
    expect(response.headers.getSetCookie()).toEqual([
      "authjs.session-token=rotated; Path=/; HttpOnly",
    ]);
  });

  it("passes a missing session through", async () => {
    const response = await answer(null)(
      new Request("http://localhost/api/auth/session"),
    );

    expect(await response.json()).toBeNull();
  });

  it("leaves other Auth.js actions alone", async () => {
    const response = await answer({ csrfToken: "csrf" })(
      new Request("http://localhost/api/auth/csrf"),
    );

    expect(await response.json()).toEqual({ csrfToken: "csrf" });
  });
});
