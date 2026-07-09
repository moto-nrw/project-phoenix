/**
 * Operator-specific NextAuth configuration.
 *
 * Cookie: "operator.session-token" with NO domain (host-only).
 * This means the operator cookie is only visible on operator.{domain},
 * never leaking to tenant subdomains.
 *
 * Providers: Operator credentials only (no Discord)
 * SignIn page: "/operator/login"
 * BasePath: "/api/operator/auth"
 */

import type { NextAuthConfig } from "next-auth";
import CredentialsProvider from "next-auth/providers/credentials";
import { canonicalForwardedFor } from "~/lib/client-headers.server";
import {
  logger,
  parseJwtPayload,
  buildAuthUser,
  performOperatorLogin,
  createOperatorLoginError,
  refreshTokenExpiry,
  operatorRedirectCallback,
  sharedJwtCallback,
  sharedSessionCallback,
  _resetRefreshState,
} from "./shared";

export const operatorAuthConfig = {
  providers: [
    CredentialsProvider({
      id: "operator-credentials",
      name: "Operator Credentials",
      credentials: {
        email: { label: "Email", type: "email" },
        password: { label: "Password", type: "password" },
        internalRefresh: { label: "Internal Refresh", type: "text" },
        token: { label: "Token", type: "text" },
        refreshToken: { label: "Refresh Token", type: "text" },
      },
      async authorize(credentials, request) {
        const creds = credentials as Record<string, string> | undefined;
        const isDev = process.env.NODE_ENV === "development";

        // Handle internal token refresh
        if (
          creds?.internalRefresh === "true" &&
          creds?.token &&
          creds?.refreshToken
        ) {
          if (isDev) {
            logger.debug("handling operator internal token refresh", {});
          }

          const payload = parseJwtPayload(creds.token);
          if (!payload) return null;

          const email = payload.username ?? payload.email ?? "";
          return buildAuthUser(
            payload,
            creds.token,
            creds.refreshToken,
            email,
            "platform",
          );
        }

        // Regular operator login flow
        if (!creds?.email || !creds?.password) return null;

        // Forward client IP headers for audit logs and rate limiting
        const forwardHeaders: Record<string, string> = {};
        const forwardedFor = canonicalForwardedFor(request?.headers ?? null);
        if (forwardedFor) {
          forwardHeaders["X-Forwarded-For"] = forwardedFor;
        }

        const loginResult = await performOperatorLogin(
          creds.email,
          creds.password,
          isDev,
          forwardHeaders,
        );

        // Handle error responses with specific status codes
        if (!loginResult || (loginResult.status && !loginResult.access_token)) {
          const status = loginResult?.status;
          if (status === 403)
            throw await createOperatorLoginError("account_inactive");
          if (status === 429)
            throw await createOperatorLoginError("rate_limited");
          throw await createOperatorLoginError("invalid_credentials");
        }

        const payload = parseJwtPayload(loginResult.access_token);
        if (!payload) return null;

        return buildAuthUser(
          payload,
          loginResult.access_token,
          loginResult.refresh_token,
          creds.email,
          "platform",
        );
      },
    }),
  ],
  basePath: "/api/operator/auth",
  callbacks: {
    redirect: operatorRedirectCallback,
    jwt: sharedJwtCallback,
    session: sharedSessionCallback,
  },
  events: {
    signOut() {
      _resetRefreshState();
    },
  },
  trustHost: true,
  pages: {
    signIn: "/operator/login",
  },
  cookies: {
    // Host-only cookies: no domain set, so the browser scopes them to
    // the exact hostname (operator.moto-app.de). They are invisible to
    // tenant subdomains (school-a.moto-app.de), preventing session leaks.
    sessionToken: {
      name: "operator.session-token",
      options: {
        httpOnly: true,
        sameSite: "lax" as const,
        path: "/",
        secure: process.env.NODE_ENV === "production",
      },
    },
    callbackUrl: {
      name: "operator.callback-url",
      options: {
        sameSite: "lax" as const,
        path: "/",
        secure: process.env.NODE_ENV === "production",
      },
    },
    csrfToken: {
      name: "operator.csrf-token",
      options: {
        httpOnly: true,
        sameSite: "lax" as const,
        path: "/",
        secure: process.env.NODE_ENV === "production",
      },
    },
  },
  session: {
    strategy: "jwt",
    maxAge: Math.floor(refreshTokenExpiry / 1000),
  },
} satisfies NextAuthConfig;
