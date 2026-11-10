import "server-only";
import type { NextAuthConfig } from "next-auth";
import CredentialsProvider from "next-auth/providers/credentials";
import { env } from "~/env";
import { loginAtBackend, refusalFor } from "./backend-login";
import { buildAuthUser, parseJwtPayload, seededEmail } from "./jwt-claims";
import type { PortalDescriptor } from "./portals";
import { createJwtCallback, sessionCallback } from "./session-callbacks";
import { refreshTokenExpiry } from "./session-lifetime";
import { isServerSessionMutation } from "./session-mutation";
import type { AuthUser } from "./session-types";
import { redeemTraegerHandoff } from "./token-refresh";
import { validateSessionToken } from "./token-validation";

type Credentials = Record<string, string | undefined>;

const TEXT = { type: "text" } as const;

function credentialFields(portal: PortalDescriptor) {
  return {
    ...(portal.passwordLogin && {
      email: { label: "Email", type: "email" },
      password: { label: "Password", type: "password" },
    }),
    internalRefresh: { label: "Internal Refresh", ...TEXT },
    token: { label: "Token", ...TEXT },
    refreshToken: { label: "Refresh Token", ...TEXT },
    ...(portal.tenantFeatures && {
      traegerHandoff: { label: "Träger Handoff", ...TEXT },
    }),
  };
}

/** A session from a token pair the backend confirms for this portal. */
async function seededUser(
  portal: PortalDescriptor,
  token: string,
  refreshToken: string,
): Promise<AuthUser | null> {
  const payload = await validateSessionToken(
    token,
    portal.validation,
    refreshToken,
  );
  if (!payload) return null;
  return buildAuthUser(
    payload,
    token,
    refreshToken,
    seededEmail(payload, portal.identity),
    portal,
  );
}

async function passwordUser(
  portal: PortalDescriptor,
  login: NonNullable<PortalDescriptor["passwordLogin"]>,
  creds: Credentials,
  request: Request | undefined,
): Promise<AuthUser | null> {
  const email = creds.email;
  if (!email || !creds.password) return null;
  const outcome = await loginAtBackend(
    portal,
    { email, password: creds.password },
    request,
  );
  if (!outcome.ok) throw await refusalFor(login, outcome);
  const payload = parseJwtPayload(outcome.tokens.access_token);
  if (!payload) return null;
  return buildAuthUser(
    payload,
    outcome.tokens.access_token,
    outcome.tokens.refresh_token,
    email,
    portal,
  );
}

function credentialsProvider(portal: PortalDescriptor) {
  return CredentialsProvider({
    id: portal.providerId,
    name: portal.providerName,
    credentials: credentialFields(portal),
    async authorize(credentials, request) {
      const creds = credentials as Credentials | undefined;
      // HTTP credentials cannot create a session from caller-supplied tokens.
      // Only the BFF's in-process mutation context may use this seed path.
      if (creds?.internalRefresh === "true") {
        if (!isServerSessionMutation() || !creds.token || !creds.refreshToken)
          return null;
        return seededUser(portal, creds.token, creds.refreshToken);
      }

      // Absprung from the Träger portal (#4654): the hand-off token opens
      // an OGS session of this school without a sign-in. Only a session
      // the Träger opened may arrive this way.
      if (portal.tenantFeatures && creds?.traegerHandoff) {
        const pair = await redeemTraegerHandoff(
          creds.traegerHandoff,
          portal.backend.refresh,
        );
        if (!pair) return null;
        const payload = await validateSessionToken(
          pair.access_token,
          portal.validation,
          pair.refresh_token,
        );
        if (!payload?.via_traeger) return null;
        return buildAuthUser(
          payload,
          pair.access_token,
          pair.refresh_token,
          seededEmail(payload, portal.identity),
          portal,
        );
      }

      if (!portal.passwordLogin || !creds) return null;
      return passwordUser(portal, portal.passwordLogin, creds, request);
    },
  });
}

function cookieOptions(
  names: NonNullable<PortalDescriptor["cookieNames"]>,
  sameSite: "lax" | "strict",
  httpOnly: boolean,
) {
  return {
    ...(httpOnly && { httpOnly: true }),
    sameSite,
    path: "/",
    ...(names.domain && { domain: names.domain }),
    secure: process.env.NODE_ENV === "production",
  };
}

function cookieConfig(portal: PortalDescriptor): NextAuthConfig["cookies"] {
  const names = portal.cookieNames;
  if (!names) return {};
  return {
    sessionToken: {
      name: names.session,
      options: cookieOptions(names, names.sameSite, true),
    },
    // Lax: Auth.js reads it during the initial sign-in navigation.
    callbackUrl: {
      name: names.callbackUrl,
      options: cookieOptions(names, "lax", false),
    },
    csrfToken: {
      name: names.csrf,
      options: cookieOptions(names, names.sameSite, true),
    },
  };
}

/**
 * The Auth.js configuration of one portal. Every portal pins the validated
 * NEXTAUTH_SECRET: the write gate, the layout snapshots and the recovery
 * proof decode with it, and Auth.js would otherwise prefer AUTH_SECRET.
 */
export function createPortalAuthConfig(portal: PortalDescriptor) {
  return {
    secret: env.NEXTAUTH_SECRET,
    providers: [credentialsProvider(portal)],
    basePath: portal.basePath,
    callbacks: {
      redirect: portal.redirect,
      jwt: createJwtCallback(portal),
      session: sessionCallback,
    },
    trustHost: true,
    pages: { signIn: portal.signInPage },
    cookies: cookieConfig(portal),
    session: {
      strategy: "jwt" as const,
      maxAge: Math.floor(refreshTokenExpiry / 1000),
    },
  } satisfies NextAuthConfig;
}
