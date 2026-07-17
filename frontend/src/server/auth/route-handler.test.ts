import { describe, expect, it, vi } from "vitest";
import type { NextAuthRequest, NextAuthResult, Session } from "next-auth";
import { NextRequest } from "next/server";
import { createResponseAwareAuth } from "./route-handler";

const tenantSession = {
  user: { id: "42", name: "Tablet User", token: "access-2" },
  expires: "2099-01-01T00:00:00.000Z",
} as Session;

function createRawAuth(session: Session | null, cookieName: string) {
  const rawAuth = vi.fn((handler?: unknown) => {
    if (typeof handler !== "function") return Promise.resolve(session);

    return async (request: NextRequest, context?: unknown) => {
      const authRequest = request as NextAuthRequest;
      authRequest.auth = session;
      const response = await (
        handler as (
          request: NextAuthRequest,
          context?: unknown,
        ) => Promise<Response>
      )(authRequest, context);
      response.headers.append(
        "Set-Cookie",
        `${cookieName}=rotated-session; Path=/; HttpOnly; Secure; SameSite=Lax`,
      );
      return response;
    };
  });

  return rawAuth as unknown as NextAuthResult["auth"];
}

describe("createResponseAwareAuth", () => {
  it("preserves Auth.js rotation cookies on the final route response", async () => {
    const rawAuth = createRawAuth(tenantSession, "tenant.session-token");
    const helpers = createResponseAwareAuth(rawAuth);
    const nestedSessions: Array<Session | null> = [];
    const route = helpers.withAuthResponse(async () => {
      nestedSessions.push(await helpers.auth());
      nestedSessions.push(await helpers.uncachedAuth());
      return Response.json({ ok: true });
    });

    const response = await route(
      new NextRequest("https://school.moto-app.de/api/students"),
    );

    expect(response.status).toBe(200);
    expect(response.headers.get("set-cookie")).toContain(
      "tenant.session-token=rotated-session",
    );
    expect(nestedSessions).toEqual([tenantSession, tenantSession]);
    // One call installs the response-aware handler. Nested reads must use the
    // request context and must not invoke a second JWT callback/rotation.
    expect(vi.mocked(rawAuth)).toHaveBeenCalledTimes(1);
  });

  it("keeps tenant and operator request contexts isolated", async () => {
    const operatorSession = {
      user: { id: "7", name: "Operator", token: "operator-access" },
      expires: "2099-01-01T00:00:00.000Z",
    } as Session;
    const tenant = createResponseAwareAuth(
      createRawAuth(tenantSession, "tenant.session-token"),
    );
    const operatorRawAuth = createRawAuth(
      operatorSession,
      "operator.session-token",
    );
    const operator = createResponseAwareAuth(operatorRawAuth);

    const route = tenant.withAuthResponse(async () => {
      expect(await tenant.auth()).toBe(tenantSession);
      expect(await operator.auth()).toBe(operatorSession);
      return new Response(null, { status: 204 });
    });

    await expect(
      route(new NextRequest("https://school.moto-app.de/api/auth/account")),
    ).resolves.toMatchObject({ status: 204 });
    // The operator read falls back to its own raw instance; it cannot observe
    // the tenant AsyncLocalStorage value.
    expect(vi.mocked(operatorRawAuth)).toHaveBeenCalledTimes(1);
  });

  it("preserves rotation cookies on handled error responses", async () => {
    const helpers = createResponseAwareAuth(
      createRawAuth(tenantSession, "tenant.session-token"),
    );
    const route = helpers.withAuthResponse(async () =>
      Response.json({ error: "backend unavailable" }, { status: 503 }),
    );

    const response = await route(
      new NextRequest("https://school.moto-app.de/api/sse/events"),
    );

    expect(response.status).toBe(503);
    expect(response.headers.get("set-cookie")).toContain("rotated-session");
  });
});
