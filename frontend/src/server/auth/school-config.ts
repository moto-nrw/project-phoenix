/**
 * School-specific NextAuth configuration ("moto schule", #2207).
 *
 * Cookie: "school.session-token" with NO domain (host-only).
 * The school cookie is only visible on the configured school host, never
 * leaking to tenant, operator, or parents hosts. Mirrors parent-config.ts;
 * see operator-config.ts for the reference shape.
 *
 * Unlike parent tokens, school tokens are tenant-bound (the backend pins the
 * school where the account holds a school-portal role), so the session claims
 * carry a real tenantId — nothing here needs to differ for that, the claims
 * come straight from the JWT payload.
 *
 * Providers: school credentials only.
 * SignIn page: "/school/login"  (rewritten from "/login" by proxy.ts)
 * BasePath: "/api/school/auth"
 */

import type { NextAuthConfig } from "next-auth";
import CredentialsProvider from "next-auth/providers/credentials";
import { canonicalForwardedFor } from "~/lib/client-headers.server";
import {
  logger,
  parseJwtPayload,
  buildAuthUser,
  performSchoolLogin,
  createOperatorLoginError, // generic CredentialsSignin wrapper — reused
  refreshTokenExpiry,
  schoolRedirectCallback,
  sharedJwtCallback,
  sharedSessionCallback,
  _resetRefreshState,
} from "./shared";

export const schoolAuthConfig = {
  providers: [
    CredentialsProvider({
      id: "school-credentials",
      name: "School Credentials",
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

        // Internal token seeding — used after the login page (or the MFA
        // exchange) already obtained a school token pair from the backend,
        // and by the proactive refresh path. Mirrors the tenant flow.
        if (
          creds?.internalRefresh === "true" &&
          creds?.token &&
          creds?.refreshToken
        ) {
          if (isDev) {
            logger.debug("handling school internal token refresh", {});
          }

          const payload = parseJwtPayload(creds.token);
          if (!payload) return null;

          const email = payload.email ?? payload.sub ?? "";
          return buildAuthUser(
            payload,
            creds.token,
            creds.refreshToken,
            email,
            "school",
          );
        }

        // Regular school login flow. The login PAGE goes through
        // lib/mfa-api (MFA-aware) and then seeds via internalRefresh; this
        // path exists for non-MFA accounts and programmatic sign-in.
        if (!creds?.email || !creds?.password) return null;

        const forwardHeaders: Record<string, string> = {};
        const forwardedFor = canonicalForwardedFor(request?.headers ?? null);
        if (forwardedFor) {
          forwardHeaders["X-Forwarded-For"] = forwardedFor;
        }

        const loginResult = await performSchoolLogin(
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
          //   "invalid_credentials"    → wrong password OR unknown email
          //   "account_inactive"       → account disabled
          //   "no_school_portal_role"  → password accepted, but no
          //                              Lehrkraft role at any school
          //   "mfa_required" / "mfa_enrollment_required" → the account
          //     needs its second factor; only the login page (mfa-api)
          //     can drive that flow, so this authorize path rejects.
          if (code === "account_inactive")
            throw await createOperatorLoginError("account_inactive");
          if (code === "no_school_portal_role")
            throw await createOperatorLoginError("no_school_portal_role");
          if (code === "mfa_required" || code === "mfa_enrollment_required")
            throw await createOperatorLoginError(code);
          throw await createOperatorLoginError("invalid_credentials");
        }

        const payload = parseJwtPayload(loginResult.access_token);
        if (!payload) return null;

        return buildAuthUser(
          payload,
          loginResult.access_token,
          loginResult.refresh_token,
          creds.email,
          "school",
        );
      },
    }),
  ],
  basePath: "/api/school/auth",
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
    // The proxy rewrites /login on the school host to /school/login
    // internally — NextAuth uses this signIn path verbatim, so it must
    // match the App Router layout (app/school/login/page.tsx).
    signIn: "/school/login",
  },
  cookies: {
    // Host-only: no domain set, so the browser scopes the cookie to the
    // configured school host exactly. Tenant, operator, and parents hosts
    // never see this cookie, so a leak in either direction is impossible
    // by design.
    //
    // SameSite=Strict on the session + CSRF cookies (issue #2207 spec):
    // the school portal has no cross-site GET flows that depend on cookie
    // presence — login is POST-only, no third-party callbacks.
    // callbackUrl stays Lax because NextAuth uses it during initial
    // sign-in navigations.
    sessionToken: {
      name: "school.session-token",
      options: {
        httpOnly: true,
        sameSite: "strict" as const,
        path: "/",
        secure: process.env.NODE_ENV === "production",
      },
    },
    callbackUrl: {
      name: "school.callback-url",
      options: {
        sameSite: "lax" as const,
        path: "/",
        secure: process.env.NODE_ENV === "production",
      },
    },
    csrfToken: {
      name: "school.csrf-token",
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
