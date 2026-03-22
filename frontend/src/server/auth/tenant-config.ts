/**
 * Tenant-specific NextAuth configuration.
 *
 * Cookie: "next-auth.session-token" on .${TENANT_DOMAIN} (shared across tenant subdomains)
 * Providers: Teacher credentials + Discord
 * SignIn page: "/" (tenant login, resolved by subdomain middleware)
 */

import type { NextAuthConfig } from "next-auth";
import DiscordProvider from "next-auth/providers/discord";
import CredentialsProvider from "next-auth/providers/credentials";
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

          const payload = parseJwtPayload(creds.token);
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
  trustHost: true,
  pages: {
    signIn: "/",
  },
  cookies: {
    // Cross-subdomain cookie sharing for tenant-to-tenant switching.
    // On production: .moto-app.de so school-a and school-b share sessions.
    // On localhost: omit domain (host-only) — tenant switching requires re-login.
    ...(() => {
      const isLocalhost =
        !process.env.TENANT_DOMAIN || process.env.TENANT_DOMAIN === "localhost";
      const cookieDomain =
        isLocalhost || !process.env.TENANT_DOMAIN
          ? undefined
          : `.${process.env.TENANT_DOMAIN}`;
      return cookieDomain !== undefined
        ? {
            sessionToken: {
              name: "next-auth.session-token",
              options: {
                httpOnly: true,
                sameSite: "lax" as const,
                path: "/",
                domain: cookieDomain,
                secure: process.env.NODE_ENV === "production",
              },
            },
            callbackUrl: {
              name: "next-auth.callback-url",
              options: {
                sameSite: "lax" as const,
                path: "/",
                domain: cookieDomain,
                secure: process.env.NODE_ENV === "production",
              },
            },
            csrfToken: {
              name: "next-auth.csrf-token",
              options: {
                httpOnly: true,
                sameSite: "lax" as const,
                path: "/",
                domain: cookieDomain,
                secure: process.env.NODE_ENV === "production",
              },
            },
          }
        : {};
    })(),
  },
  session: {
    strategy: "jwt",
    maxAge: Math.floor(refreshTokenExpiry / 1000),
  },
} satisfies NextAuthConfig;
