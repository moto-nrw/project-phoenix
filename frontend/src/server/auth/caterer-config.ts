/**
 * Caterer-specific NextAuth configuration (ADR 0075).
 *
 * Cookie: "caterer.session-token" with NO domain (host-only), so the caterer
 * session is visible on the caterer host only. Mirrors parent-config.ts:
 * caterer tokens carry no school, like parent tokens.
 *
 * Providers: caterer credentials only.
 * SignIn page: "/caterer/login"  (rewritten from "/login" by proxy.ts)
 * BasePath: "/api/caterer/auth"
 */

import { validateSessionToken } from "./token-validation";
import type { NextAuthConfig } from "next-auth";
import CredentialsProvider from "next-auth/providers/credentials";
import { analyticsSessionHeaders } from "~/lib/analytics-session-header.server";
import { wireErrorCode } from "~/lib/api-error";
import { canonicalForwardedFor } from "~/lib/client-headers.server";
import {
  parseJwtPayload,
  buildAuthUser,
  performCatererLogin,
  createOperatorLoginError, // generic CredentialsSignin wrapper — reused
  refreshTokenExpiry,
  operatorRedirectCallback, // generic origin/subdomain check — reused
  sharedJwtCallback,
  sharedSessionCallback,
  _resetRefreshState,
} from "./shared";

const sessionCookie = (sameSite: "strict" | "lax", httpOnly: boolean) => ({
  httpOnly,
  sameSite,
  path: "/",
  secure: process.env.NODE_ENV === "production",
});

export const catererAuthConfig = {
  providers: [
    CredentialsProvider({
      id: "caterer-credentials",
      name: "Caterer Credentials",
      credentials: {
        email: { label: "Email", type: "email" },
        password: { label: "Password", type: "password" },
        internalRefresh: { label: "Internal Refresh", type: "text" },
        token: { label: "Token", type: "text" },
        refreshToken: { label: "Refresh Token", type: "text" },
      },
      async authorize(credentials, request) {
        const creds = credentials as Record<string, string> | undefined;

        // Internal token refresh, as in the other portals.
        if (
          creds?.internalRefresh === "true" &&
          creds?.token &&
          creds?.refreshToken
        ) {
          const payload = await validateSessionToken(
            creds.token,
            "caterer",
            creds.refreshToken,
          );
          if (!payload) return null;
          return buildAuthUser(
            payload,
            creds.token,
            creds.refreshToken,
            payload.email ?? payload.sub ?? "",
            "caterer",
          );
        }

        if (!creds?.email || !creds?.password) return null;

        const forwardHeaders: Record<string, string> = analyticsSessionHeaders(
          request?.headers,
        );
        const forwardedFor = canonicalForwardedFor(request?.headers ?? null);
        if (forwardedFor) {
          forwardHeaders["X-Forwarded-For"] = forwardedFor;
        }

        const loginResult = await performCatererLogin(
          creds.email,
          creds.password,
          forwardHeaders,
        );
        if (!loginResult || (loginResult.status && !loginResult.access_token)) {
          if (loginResult?.status === 429)
            throw await createOperatorLoginError("rate_limited");
          // Backend body codes:
          //   "identity.invalid_credentials"      → wrong password, unknown
          //     address, or an account that is no caterer (masked alike)
          //   "identity.session_account_inactive" → account disabled
          //   "identity.caterer_without_school"   → no school is supplied
          const backendCode = wireErrorCode(loginResult?.code);
          if (backendCode === "identity.session_account_inactive")
            throw await createOperatorLoginError("account_inactive");
          if (backendCode === "identity.caterer_without_school")
            throw await createOperatorLoginError("caterer_without_school");
          throw await createOperatorLoginError("invalid_credentials");
        }

        const payload = parseJwtPayload(loginResult.access_token);
        if (!payload) return null;
        return buildAuthUser(
          payload,
          loginResult.access_token,
          loginResult.refresh_token,
          creds.email,
          "caterer",
        );
      },
    }),
  ],
  basePath: "/api/caterer/auth",
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
    // The proxy rewrites /login on the caterer host to /caterer/login.
    signIn: "/caterer/login",
  },
  cookies: {
    // Host-only and SameSite=Strict, like the parents portal: the caterer
    // portal has no cross-site flow that needs the session cookie.
    sessionToken: {
      name: "caterer.session-token",
      options: sessionCookie("strict", true),
    },
    callbackUrl: {
      name: "caterer.callback-url",
      options: sessionCookie("lax", false),
    },
    csrfToken: {
      name: "caterer.csrf-token",
      options: sessionCookie("strict", true),
    },
  },
  session: {
    strategy: "jwt",
    maxAge: Math.floor(refreshTokenExpiry / 1000),
  },
} satisfies NextAuthConfig;
