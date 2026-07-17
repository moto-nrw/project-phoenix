import { AsyncLocalStorage } from "node:async_hooks";
import { cache } from "react";
import type { NextAuthRequest, NextAuthResult, Session } from "next-auth";
import type { NextRequest } from "next/server";
import type { RouteContext } from "~/lib/route-wrapper-utils.server";

export type ResponseRouteHandler<Context = RouteContext> = (
  request: NextRequest,
  context?: Context,
) => Promise<Response>;

type AuthenticatedResponseHandler<Context> = (
  request: NextAuthRequest,
  context: Context,
) => Promise<Response>;

type SessionContext = { session: Session | null };

/**
 * Build one portal's response-aware auth helpers around its raw Auth.js
 * instance. Each portal owns a separate AsyncLocalStorage so a tenant route
 * can never observe an operator or parent session through request context.
 */
export function createResponseAwareAuth(rawAuth: NextAuthResult["auth"]) {
  const requestSession = new AsyncLocalStorage<SessionContext>();

  const readSession = async (): Promise<Session | null> => {
    const current = requestSession.getStore();
    if (current) return current.session;
    return rawAuth();
  };

  const auth = cache(readSession);

  /**
   * Use the Auth.js handler overload so refresh Set-Cookie headers are appended
   * to every final response. Nested legacy `auth()` calls reuse request.auth
   * instead of running the JWT callback a second time.
   */
  const withAuthResponse = <Context = RouteContext>(
    handler: AuthenticatedResponseHandler<Context>,
  ): ResponseRouteHandler<Context> => {
    const wrapped = rawAuth(async (request, context) =>
      requestSession.run({ session: request.auth }, () =>
        handler(request, context as Context),
      ),
    );

    // Auth.js permits void-returning middleware callbacks in its public type.
    // This factory accepts only Response-returning route callbacks.
    return wrapped as ResponseRouteHandler<Context>;
  };

  return { auth, uncachedAuth: readSession, withAuthResponse };
}
