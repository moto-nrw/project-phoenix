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

type SessionContext = {
  session: Session | null;
  request: NextRequest;
  freshSession?: Promise<Session | null>;
  freshCookies: string[];
};

/**
 * Build one portal's response-aware auth helpers around its raw Auth.js
 * instance. Each portal owns a separate AsyncLocalStorage so a tenant route
 * can never observe an operator or parent session through request context.
 */
export function createResponseAwareAuth(rawAuth: NextAuthResult["auth"]) {
  const requestSession = new AsyncLocalStorage<SessionContext>();
  const cachedRawAuth = cache(() => rawAuth());

  const auth = async (): Promise<Session | null> => {
    const current = requestSession.getStore();
    if (current) return current.session;
    return cachedRawAuth();
  };

  const uncachedAuth = async (): Promise<Session | null> => {
    const current = requestSession.getStore();
    if (!current) return rawAuth();

    current.freshSession ??= (async () => {
      const freshReader = rawAuth(async (request) =>
        Response.json(request.auth),
      ) as ResponseRouteHandler;
      const response = await freshReader(current.request);
      current.freshCookies.push(...response.headers.getSetCookie());

      const session = (await response.json()) as Session | null;
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
        current.session = authRequest.auth;
        current.request = authRequest;
        return requestSession.run(current, () =>
          handler(authRequest, authContext as Context),
        );
      }) as ResponseRouteHandler<Context>;

      const response = await wrapped(request, context);

      // Auth.js appends the outer session cookie after the route callback.
      // Append explicit retry cookies afterwards so the freshly rotated cookie
      // wins when multiple Set-Cookie headers use the same cookie name.
      for (const cookie of current.freshCookies) {
        response.headers.append("Set-Cookie", cookie);
      }
      return response;
    };

  return { auth, uncachedAuth, withAuthResponse };
}
