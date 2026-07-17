import { describe, expect, it, vi } from "vitest";
import type { NextAuthRequest, NextAuthResult, Session } from "next-auth";
import { NextRequest } from "next/server";
import { createResponseAwareAuth } from "./route-handler";

const tenantSession = {
  user: { id: "42", name: "Tablet User", token: "access-2" },
  expires: "2099-01-01T00:00:00.000Z",
} as Session;

function createRawAuth(
  sessionOrSessions: Session | null | Array<Session | null>,
  cookieName: string,
) {
  const sessions = Array.isArray(sessionOrSessions)
    ? sessionOrSessions
    : [sessionOrSessions];
  let invocation = 0;
  const nextSession = () =>
    sessions[Math.min(invocation++, sessions.length - 1)] ?? null;
  const rawAuth = vi.fn((handler?: unknown) => {
    if (typeof handler !== "function") return Promise.resolve(nextSession());

    return async (request: NextRequest, context?: unknown) => {
      const session = nextSession();
      const currentInvocation = invocation;
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
        `${cookieName}=rotated-session-${currentInvocation}; Path=/; HttpOnly; Secure; SameSite=Lax`,
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
      nestedSessions.push(await helpers.auth());
      return Response.json({ ok: true });
    });

    const response = await route(
      new NextRequest("https://school.moto-app.de/api/students"),
    );

    expect(response.status).toBe(200);
    expect(response.headers.get("set-cookie")).toContain(
      "tenant.session-token=rotated-session-1",
    );
    expect(nestedSessions).toEqual([tenantSession, tenantSession]);
    expect(vi.mocked(rawAuth)).toHaveBeenCalledTimes(1);
  });

  it("forces one response-aware session reread and keeps its cookie last", async () => {
    const refreshedSession = {
      ...tenantSession,
      user: { ...tenantSession.user, token: "access-3" },
    } as Session;
    const rawAuth = createRawAuth(
      [tenantSession, refreshedSession],
      "tenant.session-token",
    );
    const helpers = createResponseAwareAuth(rawAuth);
    const route = helpers.withAuthResponse(async () => {
      expect(await helpers.auth()).toBe(tenantSession);
      const firstFreshSession = await helpers.uncachedAuth();
      expect(firstFreshSession).toEqual(refreshedSession);
      await expect(helpers.uncachedAuth()).resolves.toBe(firstFreshSession);
      await expect(helpers.auth()).resolves.toBe(firstFreshSession);
      return Response.json({ ok: true });
    });

    const response = await route(
      new NextRequest("https://school.moto-app.de/api/students"),
    );

    expect(response.headers.getSetCookie()).toEqual([
      expect.stringContaining("tenant.session-token=rotated-session-1"),
      expect.stringContaining("tenant.session-token=rotated-session-2"),
    ]);
    expect(vi.mocked(rawAuth)).toHaveBeenCalledTimes(2);
  });

  it("retries a backend 401 with a freshly read Auth.js session", async () => {
    const expiredSession = {
      ...tenantSession,
      user: { ...tenantSession.user, token: "expired-access" },
    } as Session;
    const refreshedSession = {
      ...tenantSession,
      user: { ...tenantSession.user, token: "fresh-access" },
    } as Session;
    const rawAuth = createRawAuth(
      [expiredSession, refreshedSession],
      "tenant.session-token",
    );
    const helpers = createResponseAwareAuth(rawAuth);
    const backend = vi.fn(async (token: string | undefined) =>
      token === "fresh-access"
        ? Response.json({ ok: true })
        : Response.json({ code: "TOKEN_EXPIRED" }, { status: 401 }),
    );
    const route = helpers.withAuthResponse(async () => {
      const initial = await helpers.auth();
      let response = await backend(initial?.user?.token);

      if (response.status === 401) {
        const fresh = await helpers.uncachedAuth();
        response = await backend(fresh?.user?.token);
      }

      return response;
    });

    const response = await route(
      new NextRequest("https://school.moto-app.de/api/students"),
    );

    expect(response.status).toBe(200);
    expect(backend).toHaveBeenNthCalledWith(1, "expired-access");
    expect(backend).toHaveBeenNthCalledWith(2, "fresh-access");
    expect(response.headers.getSetCookie().at(-1)).toContain(
      "tenant.session-token=rotated-session-2",
    );
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
