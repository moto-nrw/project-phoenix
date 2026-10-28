import "server-only";
import { AsyncLocalStorage } from "node:async_hooks";
import { cache } from "react";
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
import { sessionAccountMismatchResponse } from "./session-account-check";
import type { ServerSession } from "./shared";

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
      const response = await freshReader(current.request);
      current.freshCookies.push(...response.headers.getSetCookie());

      const session = (await response.json()) as ServerSession | null;
      current.session = session;
      return session;
    })();

    return current.freshSession;
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
        // A tab that names another account than the cookie is refused
        // before any route of any portal runs (#5183).
        const accountMismatch = sessionAccountMismatchResponse(
          authRequest,
          serverRequest.auth,
          portal,
        );
        if (accountMismatch) return accountMismatch;
        return requestSession.run(current, () =>
          handler(serverRequest, authContext as Context),
        );
      }) as ResponseRouteHandler<Context>;

      const response = await wrapped(request, context);

      // Auth.js appends the outer session cookie after the route callback.
      // Append explicit retry cookies afterwards so the freshly rotated cookie
      // wins when multiple Set-Cookie headers use the same cookie name.
      for (const cookie of current.freshCookies) {
        response.headers.append("Set-Cookie", cookie);
      }
      // Every authenticated BFF route of every portal leaves through here.
      applyPrivateNoStore(response.headers);
      return response;
    };

  return { auth, uncachedAuth, withAuthResponse };
}
