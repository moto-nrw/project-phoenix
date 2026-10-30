/**
 * Träger-specific NextAuth configuration ("moto träger", #4649).
 *
 * Cookie: "traeger.session-token" with NO domain (host-only), so the
 * Träger-Büro session is visible only on the configured Träger host, never
 * on tenant, operator, parents, or school hosts. Mirrors school-config.ts.
 *
 * Träger tokens carry scope "org" and the Träger (org_id), never a school
 * (ADR 0084). Every backend portal outside the Träger portal rejects them.
 *
 * Providers: Träger credentials only, and only the token seeding path: the
 * BFF calls the backend login through /api/traeger/auth/login (it may first
 * ask which Träger to open) and installs the minted pair here in its
 * server-only mutation context. A password never passes through this provider.
 * SignIn page: "/traeger/login"  (rewritten from "/login" by proxy.ts)
 * BasePath: "/api/traeger/auth"
 */

import "server-only";
import { isServerSessionMutation } from "./session-mutation";
import { validateSessionToken } from "./token-validation";
import type { NextAuthConfig } from "next-auth";
import CredentialsProvider from "next-auth/providers/credentials";
import {
  logger,
  buildAuthUser,
  refreshTokenExpiry,
  schoolRedirectCallback, // same-origin check for a host-only portal — reused
  sharedJwtCallback,
  sharedSessionCallback,
  _resetRefreshState,
} from "./shared";

export const traegerAuthConfig = {
  providers: [
    CredentialsProvider({
      id: "traeger-credentials",
      name: "Träger Credentials",
      credentials: {
        internalRefresh: { label: "Internal Refresh", type: "text" },
        token: { label: "Token", type: "text" },
        refreshToken: { label: "Refresh Token", type: "text" },
      },
      async authorize(credentials) {
        const creds = credentials as Record<string, string> | undefined;
        // HTTP credentials cannot create a session from caller-supplied tokens.
        // Only the BFF's in-process mutation context may use this seed path.
        if (creds?.internalRefresh === "true" && !isServerSessionMutation())
          return null;
        if (
          creds?.internalRefresh !== "true" ||
          !creds.token ||
          !creds.refreshToken
        ) {
          return null;
        }

        if (process.env.NODE_ENV === "development") {
          logger.debug("handling traeger internal token refresh", {});
        }

        // Only backend-verified org-scope claims cross this boundary.
        const payload = await validateSessionToken(
          creds.token,
          "org",
          creds.refreshToken,
        );
        if (!payload) return null;

        const email = payload.email ?? payload.sub ?? "";
        return buildAuthUser(
          payload,
          creds.token,
          creds.refreshToken,
          email,
          "org",
        );
      },
    }),
  ],
  basePath: "/api/traeger/auth",
  callbacks: {
    redirect: schoolRedirectCallback,
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
    // The proxy rewrites /login on the Träger host to /traeger/login.
    signIn: "/traeger/login",
  },
  cookies: {
    // Host-only (no domain) and SameSite=Strict like the school portal: the
    // Träger portal has no cross-site flow that needs the cookie.
    sessionToken: {
      name: "traeger.session-token",
      options: {
        httpOnly: true,
        sameSite: "strict" as const,
        path: "/",
        secure: process.env.NODE_ENV === "production",
      },
    },
    callbackUrl: {
      name: "traeger.callback-url",
      options: {
        sameSite: "lax" as const,
        path: "/",
        secure: process.env.NODE_ENV === "production",
      },
    },
    csrfToken: {
      name: "traeger.csrf-token",
      options: {
        httpOnly: true,
        sameSite: "strict" as const,
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
