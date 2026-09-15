/**
 * Tenant-specific NextAuth configuration.
 *
 * Cookie: "${TENANT_DOMAIN-derived}.session-token" on .${TENANT_DOMAIN} (shared across tenant subdomains)
 * Providers: Teacher credentials + Discord
 * SignIn page: "/" (tenant login, resolved by subdomain proxy)
 */

import { validateSessionToken } from "./token-validation";
import type { NextAuthConfig } from "next-auth";
import DiscordProvider from "next-auth/providers/discord";
import CredentialsProvider from "next-auth/providers/credentials";
import { env } from "~/env";
import {
  logger,
  parseJwtPayload,
  buildDisplayName,
  buildAuthUser,
  performLogin,
  refreshTokenExpiry,
  sharedRedirectCallback,
  sharedJwtCallback,
  sharedSessionCallback,
  _resetRefreshState,
} from "./shared";

export const tenantAuthConfig = {
  providers: [
    DiscordProvider,
    CredentialsProvider({
      name: "Credentials",
      credentials: {
        email: { label: "Email", type: "email" },
        password: { label: "Password", type: "password" },
        tenantSlug: { label: "Tenant Slug", type: "text" },
        internalRefresh: { label: "Internal Refresh", type: "text" },
        token: { label: "Token", type: "text" },
        refreshToken: { label: "Refresh Token", type: "text" },
      },
      async authorize(credentials, _request) {
        const creds = credentials as Record<string, string> | undefined;
        const isDev = process.env.NODE_ENV === "development";

        // Handle internal token refresh
        if (
          creds?.internalRefresh === "true" &&
          creds?.token &&
          creds?.refreshToken
        ) {
          if (isDev) {
            logger.debug("handling internal token refresh", {});
          }

          const payload = await validateSessionToken(
            creds.token,
            "tenant",
            creds.refreshToken,
          );
          if (!payload) return null;

          const email = payload.email ?? payload.sub ?? "";
          return buildAuthUser(payload, creds.token, creds.refreshToken, email);
        }

        // Regular login flow
        if (!creds?.email || !creds?.password) return null;

        const loginResult = await performLogin(
          creds.email,
          creds.password,
          creds.tenantSlug ?? "",
          isDev,
        );
        if (!loginResult) return null;

        const payload = parseJwtPayload(loginResult.access_token);
        if (!payload) return null;

        if (isDev) {
          logger.debug("token payload parsed", { has_roles: !!payload.roles });
          if (payload.roles && Array.isArray(payload.roles)) {
            logger.debug("found roles in token", { roles: payload.roles });
          } else {
            logger.warn("no roles found in token", {});
          }
          logger.debug("display name", {
            name: buildDisplayName(payload, creds.email),
          });
        }

        return buildAuthUser(
          payload,
          loginResult.access_token,
          loginResult.refresh_token,
          creds.email,
        );
      },
    }),
  ],
  callbacks: {
    redirect: sharedRedirectCallback,
    jwt: sharedJwtCallback,
    session: sharedSessionCallback,
  },
  events: {
    // Clear the module-level refresh cache on sign-out so that a subsequent
    // login by a different user cannot hit stale cached tokens from the
    // previous session.
    signOut() {
      _resetRefreshState();
    },
  },
  trustHost: true,
  pages: {
    signIn: "/",
  },
  cookies: {
    // Cross-subdomain cookie sharing for tenant-to-tenant switching.
    // On production: .moto-app.de so school-a and school-b share sessions.
    // On localhost: omit domain (host-only) — tenant switching requires re-login.
    //
    // Cookie names are derived from TENANT_DOMAIN to prevent collisions when
    // environments share a parent domain (e.g., staging.moto-app.de under
    // moto-app.de). Without unique names, the browser sends both cookies and
    // Auth.js picks the wrong one → MissingCSRF errors.
    ...(() => {
      if (!env.TENANT_DOMAIN || env.TENANT_DOMAIN === "localhost") return {};

      const cookieDomain = `.${env.TENANT_DOMAIN}`;
      const cookiePrefix = env.TENANT_DOMAIN.replace(/\./g, "-");
      return {
        sessionToken: {
          name: `${cookiePrefix}.session-token`,
          options: {
            httpOnly: true,
            sameSite: "lax" as const,
            path: "/",
            domain: cookieDomain,
            secure: process.env.NODE_ENV === "production",
          },
        },
        callbackUrl: {
          name: `${cookiePrefix}.callback-url`,
          options: {
            sameSite: "lax" as const,
            path: "/",
            domain: cookieDomain,
            secure: process.env.NODE_ENV === "production",
          },
        },
        csrfToken: {
          name: `${cookiePrefix}.csrf-token`,
          options: {
            httpOnly: true,
            sameSite: "lax" as const,
            path: "/",
            domain: cookieDomain,
            secure: process.env.NODE_ENV === "production",
          },
        },
      };
    })(),
  },
  session: {
    strategy: "jwt",
    maxAge: Math.floor(refreshTokenExpiry / 1000),
  },
} satisfies NextAuthConfig;
