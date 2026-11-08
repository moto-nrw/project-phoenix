import "server-only";
import type { DefaultSession, Session, User } from "next-auth";

declare module "next-auth" {
  // The browser's view of a session (#5005): useSession(), getSession(),
  // the session routes and the layout snapshots. It carries no access or
  // refresh token; server code reads them through ServerSession. The user
  // lists DefaultSession's fields itself, so no User field reaches it.
  interface Session extends DefaultSession {
    user: {
      id: string;
      name?: string | null;
      email?: string | null;
      image?: string | null;
      roles?: string[];
      roleIsSystem?: boolean[];
      permissions?: string[];
      firstName?: string;
      isAdmin?: boolean;
      tenantId?: number;
      orgId?: number;
      scope?: string;
      // Absprung aus dem Träger-Portal (#4654): diese OGS-Sitzung kam über
      // das Träger-Büro, ohne eigenen Zugang zur Schule.
      viaTraeger?: boolean;
      // Admin staff-view preview (#2893): true while the session renders the
      // tenant portal through a read-only preview token; previewTargetName /
      // previewTargetAccountId identify the previewed staff member for the
      // banner and the end-of-preview audit call.
      isPreview?: boolean;
      previewTargetName?: string;
      previewTargetAccountId?: string;
    };
    error?: "RefreshTokenExpired" | "RefreshTokenError";
  }

  // Importable from client code too, so it carries no token (#5005); the
  // credentials providers return AuthUser.
  interface User {
    roles?: string[];
    roleIsSystem?: boolean[];
    permissions?: string[];
    firstName?: string;
    isAdmin?: boolean;
    tenantId?: number;
    orgId?: number;
    scope?: string;
    viaTraeger?: boolean;
  }
}

// The encrypted session cookie's content. next-auth/jwt is server-only; the
// browser token ratchet refuses its import outside server code (#5005).
declare module "next-auth/jwt" {
  interface JWT {
    id?: string;
    token?: string;
    refreshToken?: string;
    roles?: string[];
    roleIsSystem?: boolean[];
    permissions?: string[];
    firstName?: string;
    isAdmin?: boolean;
    tenantId?: number;
    orgId?: number;
    scope?: string;
    viaTraeger?: boolean;
    tokenExpiry?: number;
    refreshTokenExpiry?: number;
    refreshRecoveryProof?: string;
    error?: "RefreshTokenExpired" | "RefreshTokenError";
    needsRefresh?: boolean;
    // Admin staff-view preview (#2893). While previewTargetAccountId is set,
    // `token`/`tokenExpiry` hold the READ-ONLY preview token and the admin's
    // own access token is parked in previewAdminToken; `refreshToken` stays
    // the admin's — no refresh token ever exists in the target's name.
    previewTargetAccountId?: string;
    previewTargetName?: string;
    previewAdminToken?: string;
    previewAdminTokenExpiry?: number;
    previewAdminId?: string;
    previewAdminEmail?: string;
  }
}

/**
 * The session as server code sees it: what the session callback returns,
 * including the access and refresh token. Only a portal's auth() and its
 * route wrappers hand it out; the session routes strip it to Session before
 * it reaches the browser (browser-session.ts, #5005).
 */
export type ServerSession = Session & {
  user: Session["user"] & { token?: string; refreshToken?: string };
};

/** What the credentials providers return: the user with its token pair. */
export type AuthUser = User & { token?: string; refreshToken?: string };
