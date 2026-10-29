import "server-only";
import { AsyncLocalStorage } from "node:async_hooks";
import { cache } from "react";
import { cookies } from "next/headers";
import { NextResponse } from "next/server";
import {
  getModifiedCookieValues,
  type ResponseCookies,
} from "next/dist/server/web/spec-extension/adapters/request-cookies";
import { withServerSessionMutation } from "./session-mutation";
import * as Sentry from "@sentry/nextjs";
import type { NextAuthRequest, NextAuthResult } from "next-auth";
import type { NextRequest } from "next/server";
import { applyPrivateNoStore } from "~/lib/private-no-store";
import { REQUEST_ID_HEADER } from "~/lib/request-id";
import type { RouteContext } from "~/lib/route-wrapper-utils.server";
import {
  applySentrySessionContext,
  sentrySessionContext,
  type SentryPortal,
} from "~/lib/sentry-context";
import { withForcedSessionRenewal, type ServerSession } from "./shared";

export type ResponseRouteHandler<Context = RouteContext> = (
  request: NextRequest,
  context?: Context,
) => Promise<Response>;

/** A route request whose `auth` is the server view, tokens included. */
export type ServerAuthRequest = NextAuthRequest & {
  auth: ServerSession | null;
};

type AuthenticatedResponseHandler<Context> = (
  request: ServerAuthRequest,
  context: Context,
) => Promise<Response>;

type SessionContext = {
  session: ServerSession | null;
  request: NextRequest;
  freshSession?: Promise<ServerSession | null>;
  freshCookies: string[];
  replacementCookies?: string[];
};

/**
 * Puts the request's context on its Sentry isolation scope, so every event
 * the route raises carries portal, account, role, school and Vorgangskennung.
 * Every field is written, so nothing of another request can remain.
 */
function tagSentryRequest(portal: SentryPortal, request: NextAuthRequest) {
  applySentrySessionContext(
    sentrySessionContext(portal, request.auth?.user ?? null),
  );
  Sentry.setTags({
    portal,
    request_id: request.headers.get(REQUEST_ID_HEADER) ?? undefined,
  });
}

/**
 * Build one portal's response-aware auth helpers around its raw Auth.js
 * instance. Each portal owns a separate AsyncLocalStorage so a tenant route
 * can never observe an operator or parent session through request context.
 */
export function createResponseAwareAuth(
  rawAuth: NextAuthResult["auth"],
  portal: SentryPortal,
) {
  const requestSession = new AsyncLocalStorage<SessionContext>();
  // The session callback returns ServerSession; Auth.js types it as Session.
  const serverAuth = rawAuth as () => Promise<ServerSession | null>;
  const cachedRawAuth = cache(() => serverAuth());

  const auth = async (): Promise<ServerSession | null> => {
    const current = requestSession.getStore();
    if (current) return current.session;
    return cachedRawAuth();
  };

  const uncachedAuth = async (): Promise<ServerSession | null> => {
    const current = requestSession.getStore();
    if (!current) return serverAuth();

    current.freshSession ??= (async () => {
      const freshReader = rawAuth(async (request) =>
        Response.json(request.auth),
      ) as ResponseRouteHandler;
      // Every caller holds a backend 401: renew now, not at the expiry.
      const response = await withForcedSessionRenewal(() =>
        freshReader(current.request),
      );
      current.freshCookies.push(...response.headers.getSetCookie());

      const session = (await response.json()) as ServerSession | null;
      current.session = session;
      return session;
    })();

    return current.freshSession;
  };

  /** Capture Auth.js server-action cookies, including deletions and chunks.
   * Next's mutable cookies are only fallbacks: handleAuth appends the old
   * session afterwards. Commit the replacement explicitly after that wrapper.
   * The pinned Next adapter retains options that cookies().getAll() omits.
   */
  const mutateSession = async <T>(
    action: () => Promise<T>,
    accepts: (result: T) => boolean,
  ): Promise<T> => {
    const current = requestSession.getStore();
    if (!current)
      throw new Error("Session mutation requires a response context");
    const result = await withServerSessionMutation(action);
    if (!accepts(result)) throw new Error("Session mutation was refused");
    const jar = await cookies();
    const response = new NextResponse(null);
    for (const cookie of getModifiedCookieValues(
      jar as unknown as ResponseCookies,
    )) {
      response.cookies.set(cookie);
    }
    current.replacementCookies = response.headers.getSetCookie();
    if (!current.replacementCookies.length)
      throw new Error("Session mutation produced no cookies");
    return result;
  };

  /**
   * Use the Auth.js handler overload so refresh Set-Cookie headers are appended
   * to every final response. Nested legacy `auth()` calls reuse request.auth
   * instead of running the JWT callback a second time.
   */
  const withAuthResponse =
    <Context = RouteContext>(
      handler: AuthenticatedResponseHandler<Context>,
    ): ResponseRouteHandler<Context> =>
    async (request, context) => {
      const current: SessionContext = {
        session: null,
        request,
        freshCookies: [],
      };
      const wrapped = rawAuth(async (authRequest, authContext) => {
        const serverRequest = authRequest as ServerAuthRequest;
        current.session = serverRequest.auth;
        current.request = authRequest;
        tagSentryRequest(portal, authRequest);
        return requestSession.run(current, () =>
          handler(serverRequest, authContext as Context),
        );
      }) as ResponseRouteHandler<Context>;

      const response = await wrapped(request, context);

      // A 401 for an unexpired token means its family may be revoked
      // (#5194). Routes without their own retry still renew the session, so
      // a revoked one leaves with an ended session cookie instead of one the
      // browser keeps presenting.
      if (
        response.status === 401 &&
        current.session?.user?.token &&
        !current.freshSession
      ) {
        try {
          await requestSession.run(current, uncachedAuth);
        } catch {
          // The 401 stands; the next request renews again.
        }
      }

      return finishResponse(response, current);
    };

  function finishResponse(
    response: Response,
    current: SessionContext,
  ): Response {
    // Auth.js appends the outer session cookie after the route callback.
    // Append explicit retry cookies afterwards so the freshly rotated cookie
    // wins when multiple Set-Cookie headers use the same cookie name.
    for (const cookie of current.freshCookies) {
      response.headers.append("Set-Cookie", cookie);
    }
    if (current.replacementCookies) {
      // Discard ALL older writes of the replaced names, including chunks.
      // A longer old rotation must not resurrect trailing session chunks.
      const replacements = current.replacementCookies;
      const names = replacements.map((cookie) =>
        cookie.slice(0, cookie.indexOf("=")),
      );
      const bases = names.map((name) => name.replace(/\.\d+$/, ""));
      const previous = response.headers.getSetCookie();
      response.headers.delete("Set-Cookie");
      for (const cookie of previous) {
        const name = cookie.slice(0, cookie.indexOf("="));
        if (!bases.includes(name.replace(/\.\d+$/, "")))
          response.headers.append("Set-Cookie", cookie);
      }
      for (const cookie of replacements)
        response.headers.append("Set-Cookie", cookie);
    }
    // Every authenticated BFF route of every portal leaves through here.
    applyPrivateNoStore(response.headers);
    return response;
  }

  /** Public session minting: capture server-action cookies without reading or
   * renewing a previous session or applying its tenant-host mismatch guard. */
  const withSessionResponse =
    (handler: ResponseRouteHandler): ResponseRouteHandler =>
    async (request, context) => {
      const current: SessionContext = {
        session: null,
        request,
        freshCookies: [],
      };
      const response = await requestSession.run(current, () =>
        handler(request, context),
      );
      return finishResponse(response, current);
    };

  return {
    auth,
    uncachedAuth,
    withAuthResponse,
    mutateSession,
    withSessionResponse,
  };
}
