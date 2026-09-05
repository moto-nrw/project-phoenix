/**
 * Parent-specific NextAuth configuration.
 *
 * Cookie: "parent.session-token" with NO domain (host-only).
 * The parent cookie is only visible on the configured parents host, never
 * leaking to tenant subdomains or the operator subdomain. Mirrors the
 * operator-config.ts pattern; see that file for the reference shape.
 *
 * Providers: parent credentials only (no Discord, no operator)
 * SignIn page: "/parents/login"  (rewritten from "/login" by proxy.ts)
 * BasePath: "/api/parent/auth"
 */

import { validateSessionToken } from "./token-validation";
import type { NextAuthConfig } from "next-auth";
import CredentialsProvider from "next-auth/providers/credentials";
import { canonicalForwardedFor } from "~/lib/client-headers.server";
import {
  logger,
  parseJwtPayload,
  buildAuthUser,
  performParentLogin,
  createOperatorLoginError, // generic CredentialsSignin wrapper — reused
  refreshTokenExpiry,
  operatorRedirectCallback, // generic origin/subdomain check — reused
  sharedJwtCallback,
  sharedSessionCallback,
  _resetRefreshState,
} from "./shared";

export const parentAuthConfig = {
  providers: [
    CredentialsProvider({
      id: "parent-credentials",
      name: "Parent Credentials",
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

        // Internal token refresh — mirrors operator path.
        if (
          creds?.internalRefresh === "true" &&
          creds?.token &&
          creds?.refreshToken
        ) {
          if (isDev) {
            logger.debug("handling parent internal token refresh", {});
          }

          const payload = await validateSessionToken(
            creds.token,
            "parent",
            creds.refreshToken,
          );
          if (!payload) return null;

          const email = payload.email ?? payload.sub ?? "";
          return buildAuthUser(
            payload,
            creds.token,
            creds.refreshToken,
            email,
            "parent",
          );
        }

        // Regular parent login flow.
        if (!creds?.email || !creds?.password) return null;

        const forwardHeaders: Record<string, string> = {};
        const forwardedFor = canonicalForwardedFor(request?.headers ?? null);
        if (forwardedFor) {
          forwardHeaders["X-Forwarded-For"] = forwardedFor;
        }

        const loginResult = await performParentLogin(
          creds.email,
          creds.password,
          isDev,
          forwardHeaders,
        );

        if (!loginResult || (loginResult.status && !loginResult.access_token)) {
          const status = loginResult?.status;
          const code = loginResult?.code;
          if (status === 429)
            throw await createOperatorLoginError("rate_limited");
          // Backend body codes:
          //   "invalid_credentials" → wrong password OR unknown email (masked)
          //   "account_inactive"    → parent account disabled by the school
          //   "not_a_guardian"      → staff account hitting the parent portal
          //
          // not_a_guardian used to be masked as invalid_credentials to avoid
          // confirming the email belongs to staff. That mask protected
          // nothing: LoginParentWithAudit runs validateLoginCredentials
          // (including the password check) BEFORE findGuardianTenantForAccount
          // (backend/services/auth/auth_login_parent.go), so this code is only
          // ever reachable by someone who already proved they own the account.
          // Meanwhile the mask cost real users the one hint they needed, and
          // parents hitting the mirror case on the staff login were resetting
          // their password over and over.
          if (code === "account_inactive")
            throw await createOperatorLoginError("account_inactive");
          if (code === "not_a_guardian")
            throw await createOperatorLoginError("not_a_guardian");
          throw await createOperatorLoginError("invalid_credentials");
        }

        const payload = parseJwtPayload(loginResult.access_token);
        if (!payload) return null;

        return buildAuthUser(
          payload,
          loginResult.access_token,
          loginResult.refresh_token,
          creds.email,
          "parent",
        );
      },
    }),
  ],
  basePath: "/api/parent/auth",
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
    // The proxy rewrites /login on the parents host to /parents/login
    // internally — NextAuth uses this signIn path verbatim, so it must
    // match the App Router layout (app/parents/login/page.tsx).
    signIn: "/parents/login",
  },
  cookies: {
    // Host-only: no domain set, so the browser scopes the cookie to
    // the configured parents host exactly. Tenant + operator subdomains
    // never see this cookie, so a leak in either direction is
    // impossible by design.
    //
    // SameSite=Strict on the session + CSRF cookies hardens against
    // cross-site request forgery on a portal that handles guardian
    // PII (children, enrollment status). The parent portal has no
    // cross-site GET flows that depend on cookie presence — login is
    // POST-only, no third-party callbacks. callbackUrl stays Lax
    // because NextAuth uses it during initial sign-in navigations.
    sessionToken: {
      name: "parent.session-token",
      options: {
        httpOnly: true,
        sameSite: "strict" as const,
        path: "/",
        secure: process.env.NODE_ENV === "production",
      },
    },
    callbackUrl: {
      name: "parent.callback-url",
      options: {
        sameSite: "lax" as const,
        path: "/",
        secure: process.env.NODE_ENV === "production",
      },
    },
    csrfToken: {
      name: "parent.csrf-token",
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
