/**
 * Shared auth utilities used by both tenant and operator NextAuth configs.
 *
 * Extracted to avoid duplication between tenant-config.ts and operator-config.ts.
 * Contains: JWT parsing, user building, login functions, token refresh logic,
 * callbacks, and NextAuth module augmentation.
 */

import type { DefaultSession, NextAuthConfig, User } from "next-auth";
import { createHmac, randomBytes } from "node:crypto";
import { env } from "~/env";
import { canonicalForwardedFor } from "~/lib/client-headers.server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

// ---------------------------------------------------------------------------
// Logger
// ---------------------------------------------------------------------------

export const logger = createLogger({ component: "NextAuthConfig" });

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface JwtPayload {
  id: string | number;
  exp?: number;
  token?: string;
  sub?: string;
  username?: string;
  first_name?: string;
  last_name?: string;
  email?: string;
  roles?: string[];
  permissions?: string[];
  is_admin?: boolean;
  tenant_id?: number;
  org_id?: number;
  scope?: string;
  read_only?: boolean;
  acting_admin_id?: number;
}

// ---------------------------------------------------------------------------
// Module augmentation (shared across both configs)
// ---------------------------------------------------------------------------

declare module "next-auth" {
  interface Session extends DefaultSession {
    user: {
      id: string;
      token?: string;
      refreshToken?: string;
      roles?: string[];
      permissions?: string[];
      firstName?: string;
      isAdmin?: boolean;
      tenantId?: number;
      orgId?: number;
      scope?: string;
      // Admin staff-view preview (#2893): true while the session renders the
      // tenant portal through a read-only preview token; previewTargetName /
      // previewTargetAccountId identify the previewed staff member for the
      // banner and the end-of-preview audit call.
      isPreview?: boolean;
      previewTargetName?: string;
      previewTargetAccountId?: string;
    } & DefaultSession["user"];
    error?: "RefreshTokenExpired" | "RefreshTokenError";
  }

  interface User {
    token?: string;
    refreshToken?: string;
    roles?: string[];
    permissions?: string[];
    firstName?: string;
    isAdmin?: boolean;
    tenantId?: number;
    orgId?: number;
    scope?: string;
  }

  interface JWT {
    id?: string;
    token?: string;
    refreshToken?: string;
    roles?: string[];
    permissions?: string[];
    firstName?: string;
    isAdmin?: boolean;
    tenantId?: number;
    orgId?: number;
    scope?: string;
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

// ---------------------------------------------------------------------------
// Utility functions
// ---------------------------------------------------------------------------

function parseDurationToMs(duration: string): number {
  const regex = /^(\d+)([hm])$/;
  const match = regex.exec(duration);
  if (!match) return 12 * 60 * 60 * 1000; // 12h default
  const amount = match[1]!;
  const unit = match[2]!;
  const num = Number.parseInt(amount, 10);
  return unit === "h" ? num * 60 * 60 * 1000 : num * 60 * 1000;
}

const accessTokenExpiry = parseDurationToMs(env.AUTH_JWT_EXPIRY);
export const refreshTokenExpiry = parseDurationToMs(
  env.AUTH_JWT_REFRESH_EXPIRY,
);

export function parseJwtPayload(tokenString: string): JwtPayload | null {
  const tokenParts = tokenString.split(".");
  if (tokenParts.length !== 3) {
    logger.error("invalid jwt token format", {});
    return null;
  }

  const payloadPart = tokenParts[1];
  if (!payloadPart) {
    logger.error("invalid jwt token part", {});
    return null;
  }

  try {
    return JSON.parse(
      Buffer.from(payloadPart, "base64").toString(),
    ) as JwtPayload;
  } catch (e) {
    logger.error("error parsing jwt", {
      error: e instanceof Error ? e.message : String(e),
    });
    return null;
  }
}

export function buildDisplayName(
  payload: JwtPayload,
  fallbackEmail: string,
  ultimateFallback = "User",
): string {
  if (payload.first_name && payload.last_name) {
    return `${payload.first_name} ${payload.last_name}`;
  }
  if (payload.first_name) {
    return payload.first_name;
  }
  return payload.username ?? (fallbackEmail || ultimateFallback);
}

/**
 * Sync JWT token fields from a decoded payload. Used in all three refresh
 * code paths (cached, in-flight dedup, fresh) to keep session claims current.
 */
function syncTokenFromPayload(
  token: Record<string, unknown>,
  payload: JwtPayload,
): void {
  token.tenantId = payload.tenant_id;
  token.orgId = payload.org_id;
  token.roles = payload.roles ?? [];
  token.permissions = payload.permissions ?? [];
  token.isAdmin = payload.is_admin ?? false;
  token.scope = payload.scope;
  token.name = buildDisplayName(payload, (token.email as string) ?? "");
  // Kept in sync alongside name: the staff-preview swap (#2893) rides this
  // helper too, and session.user.firstName must follow the token's identity —
  // entering, renewing, and leaving a preview included.
  token.firstName = payload.first_name;
  // Operator JWTs store email as username; keep session in sync after email change.
  // Tenant-scoped users don't need this: their email comes from the NextAuth
  // sign-in flow (authorize callback) and doesn't change via JWT refresh —
  // only operators can change their email through the platform UI.
  if (payload.scope === "platform" && payload.username) {
    token.email = payload.username;
  }
}

export function buildAuthUser(
  payload: JwtPayload,
  token: string,
  refreshToken: string,
  email: string,
  scope?: string,
): User {
  const roles =
    payload.roles && Array.isArray(payload.roles) ? payload.roles : [];
  const permissions =
    payload.permissions && Array.isArray(payload.permissions)
      ? payload.permissions
      : [];

  return {
    id: String(payload.id),
    name: buildDisplayName(payload, email),
    email: email,
    token: token,
    refreshToken: refreshToken,
    roles: scope === "platform" ? ["operator"] : roles,
    permissions: permissions,
    firstName: payload.first_name,
    isAdmin: payload.is_admin ?? false,
    tenantId: payload.tenant_id,
    orgId: payload.org_id,
    scope: scope ?? payload.scope,
  };
}

// ---------------------------------------------------------------------------
// Login functions
// ---------------------------------------------------------------------------

export async function createOperatorLoginError(code: string): Promise<Error> {
  const { CredentialsSignin } = await import("next-auth");
  const error = new CredentialsSignin();
  error.code = code;
  return error;
}

export async function performOperatorLogin(
  email: string,
  password: string,
  isDev: boolean,
  forwardHeaders?: Record<string, string>,
): Promise<{
  access_token: string;
  refresh_token: string;
  status?: number;
} | null> {
  const apiUrl = getServerApiUrl();

  if (isDev) {
    logger.debug("attempting operator login", {
      api_url: `${apiUrl}/operator/auth/login`,
    });
  }

  try {
    const response = await fetch(`${apiUrl}/operator/auth/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json", ...forwardHeaders },
      body: JSON.stringify({ email, password }),
    });

    if (isDev) {
      logger.debug("operator login response received", {
        status: response.status,
      });
    }

    if (!response.ok) {
      const text = await response.text();
      logger.error("operator login failed", {
        status: response.status,
        error: text,
      });
      return { access_token: "", refresh_token: "", status: response.status };
    }

    const envelope = (await response.json()) as {
      status: string;
      data: {
        access_token: string;
        refresh_token: string;
        operator: { id: number; email: string; display_name: string };
      };
    };

    if (isDev) {
      logger.debug("operator login response parsed", {
        has_tokens: !!envelope.data.access_token,
      });
    }

    return {
      access_token: envelope.data.access_token,
      refresh_token: envelope.data.refresh_token,
    };
  } catch (error) {
    logger.error("operator authentication error", {
      error: error instanceof Error ? error.message : String(error),
    });
    return null;
  }
}

export async function performParentLogin(
  email: string,
  password: string,
  isDev: boolean,
  forwardHeaders?: Record<string, string>,
): Promise<{
  access_token: string;
  refresh_token: string;
  status?: number;
  code?: string;
} | null> {
  const apiUrl = getServerApiUrl();

  if (isDev) {
    logger.debug("attempting parent login", {
      api_url: `${apiUrl}/parent/auth/login`,
    });
  }

  try {
    const response = await fetch(`${apiUrl}/parent/auth/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json", ...forwardHeaders },
      body: JSON.stringify({ email, password }),
    });

    if (isDev) {
      logger.debug("parent login response received", {
        status: response.status,
      });
    }

    if (!response.ok) {
      const text = await response.text();
      // Backend sends { status, error, code } on failures. The code field
      // disambiguates 401-invalid-credentials from 401-account-inactive and
      // identifies the 403-not-a-guardian (staff-in-parent-portal) case.
      let code: string | undefined;
      try {
        const parsed = JSON.parse(text) as { code?: unknown };
        if (typeof parsed.code === "string") code = parsed.code;
      } catch {
        // Non-JSON body (e.g. gateway error page) — leave code undefined.
      }
      logger.error("parent login failed", {
        status: response.status,
        error: text,
      });
      return {
        access_token: "",
        refresh_token: "",
        status: response.status,
        code,
      };
    }

    const envelope = (await response.json()) as {
      status: string;
      data: {
        access_token: string;
        refresh_token: string;
      };
    };

    if (isDev) {
      logger.debug("parent login response parsed", {
        has_tokens: !!envelope.data.access_token,
      });
    }

    return {
      access_token: envelope.data.access_token,
      refresh_token: envelope.data.refresh_token,
    };
  } catch (error) {
    logger.error("parent authentication error", {
      error: error instanceof Error ? error.message : String(error),
    });
    return null;
  }
}

export async function performSchoolLogin(
  email: string,
  password: string,
  isDev: boolean,
  forwardHeaders?: Record<string, string>,
): Promise<{
  access_token: string;
  refresh_token: string;
  status?: number;
  code?: string;
} | null> {
  const apiUrl = getServerApiUrl();

  if (isDev) {
    logger.debug("attempting school login", {
      api_url: `${apiUrl}/school/auth/login`,
    });
  }

  try {
    const response = await fetch(`${apiUrl}/school/auth/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json", ...forwardHeaders },
      body: JSON.stringify({ email, password }),
    });

    if (isDev) {
      logger.debug("school login response received", {
        status: response.status,
      });
    }

    if (!response.ok) {
      const text = await response.text();
      // Backend sends { status, error, code } on failures. The code field
      // disambiguates invalid_credentials / account_inactive from the
      // 403 no_school_portal_role (non-Lehrkraft hitting the school
      // portal).
      let code: string | undefined;
      try {
        const parsed = JSON.parse(text) as { code?: unknown };
        if (typeof parsed.code === "string") code = parsed.code;
      } catch {
        // Non-JSON body (e.g. gateway error page) — leave code undefined.
      }
      logger.error("school login failed", {
        status: response.status,
        error: text,
      });
      return {
        access_token: "",
        refresh_token: "",
        status: response.status,
        code,
      };
    }

    // Top-level LoginResponse (no envelope), same MFA-aware shape as the
    // tenant login: status "authenticated" carries the token pair; the MFA
    // branches ("mfa_required" / "mfa_enrollment_required") are driven by
    // the login page through lib/mfa-api, never through this authorize
    // path — report them as a coded non-success so authorize rejects.
    const body = (await response.json()) as {
      status: string;
      access_token?: string;
      refresh_token?: string;
    };

    if (body.status !== "authenticated" || !body.access_token) {
      return {
        access_token: "",
        refresh_token: "",
        status: 200,
        code: body.status,
      };
    }

    if (isDev) {
      logger.debug("school login response parsed", {
        has_tokens: !!body.access_token,
      });
    }

    return {
      access_token: body.access_token,
      refresh_token: body.refresh_token ?? "",
    };
  } catch (error) {
    logger.error("school authentication error", {
      error: error instanceof Error ? error.message : String(error),
    });
    return null;
  }
}

export async function performLogin(
  email: string,
  password: string,
  tenantSlug: string,
  isDev: boolean,
): Promise<{ access_token: string; refresh_token: string } | null> {
  const apiUrl = getServerApiUrl();

  if (isDev) {
    logger.debug("attempting login", {
      api_url: `${apiUrl}/auth/login`,
      tenant_slug: tenantSlug,
    });
  }

  try {
    const body: Record<string, string> = { email, password };
    if (tenantSlug) {
      body.tenant_slug = tenantSlug;
    }

    const response = await fetch(`${apiUrl}/auth/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });

    if (isDev) {
      logger.debug("login response received", { status: response.status });
    }

    if (!response.ok) {
      const text = await response.text();
      logger.error("login failed", { status: response.status, error: text });
      return null;
    }

    const responseData = (await response.json()) as {
      access_token: string;
      refresh_token: string;
    };

    if (isDev) {
      logger.debug("login response parsed", {
        has_tokens: !!responseData.access_token,
      });
    }

    return responseData;
  } catch (error) {
    logger.error("authentication error", {
      error: error instanceof Error ? error.message : String(error),
    });
    return null;
  }
}

// ---------------------------------------------------------------------------
// Singleflight state for proactive token refresh
// ---------------------------------------------------------------------------

type RefreshResult = { access_token: string; refresh_token: string };
type RefreshAttempt =
  | {
      status: "success";
      result: RefreshResult;
      recoveryProof: string;
      refreshTokenExpiresAt: number;
    }
  | { status: "terminal" }
  | { status: "transient" };

// Per-session maps: keyed by the OLD refresh token plus its independent
// recovery proof so unrelated or partially stolen credentials cannot join a
// legitimate in-flight rotation.
const activeRefreshes = new Map<string, Promise<RefreshAttempt>>();
const refreshCacheMap = new Map<
  string,
  {
    result: RefreshResult;
    recoveryProof: string;
    refreshTokenExpiresAt: number;
    expiresAt: number;
  }
>();
const REFRESH_CACHE_TTL_MS = 5 * 60 * 1000;
const MAX_CACHE_ENTRIES = 100;

function createRefreshRecoveryProof(): string {
  return randomBytes(32).toString("base64url");
}

// A persisted refresh session must always map to the same recovery proof,
// including across frontend processes. Recovery responses re-mint its JWT and
// can therefore have different `iat` claims, so derive from the stable database
// token claim rather than from the serialized JWT. Keying the derivation with
// the Auth.js secret keeps the proof unavailable to callers that stole only the
// exposed backend access/refresh pair.
function deriveRefreshRecoveryProof(refreshToken: string): string {
  const payload = parseJwtPayload(refreshToken);
  const proofMaterial =
    typeof payload?.token === "string" && payload.token.length > 0
      ? payload.token
      : refreshToken;
  return createHmac("sha256", env.NEXTAUTH_SECRET)
    .update("phoenix-refresh-recovery-v1\0")
    .update(proofMaterial)
    .digest("base64url");
}

function refreshTokenExpiresAt(refreshToken: string): number | null {
  const payload = parseJwtPayload(refreshToken);
  if (
    !payload ||
    typeof payload.exp !== "number" ||
    !Number.isSafeInteger(payload.exp) ||
    payload.exp <= 0
  ) {
    logger.error("refresh token is missing a valid exp claim", {});
    return null;
  }
  return payload.exp * 1000;
}

async function getRefreshAuditHeaders(): Promise<Record<string, string>> {
  try {
    const { headers } = await import("next/headers");
    const incoming = await headers();
    const forwardedFor = canonicalForwardedFor(incoming);
    const userAgent = incoming.get("user-agent");
    return {
      ...(forwardedFor && { "X-Forwarded-For": forwardedFor }),
      ...(userAgent && { "User-Agent": userAgent }),
    };
  } catch {
    // Auth callbacks can also run outside a request (for example in tests or
    // maintenance code). Missing audit context must not break session renewal.
    return {};
  }
}

/** Evict expired entries so the maps don't leak memory. */
function pruneRefreshCache(): void {
  const now = Date.now();
  for (const [key, entry] of refreshCacheMap) {
    if (now >= entry.expiresAt) refreshCacheMap.delete(key);
  }
  // Safety cap: if still too large, drop oldest entries
  if (refreshCacheMap.size > MAX_CACHE_ENTRIES) {
    const excess = refreshCacheMap.size - MAX_CACHE_ENTRIES;
    const keys = refreshCacheMap.keys();
    for (let i = 0; i < excess; i++) {
      const next = keys.next();
      if (!next.done) refreshCacheMap.delete(next.value);
    }
  }
}

/** @internal Reset module-level refresh state (test isolation only) */
export function _resetRefreshState(): void {
  activeRefreshes.clear();
  refreshCacheMap.clear();
}

/** @internal Exposed for unit testing only */
export const _testHelpers = {
  parseJwtPayload,
  buildDisplayName,
  buildAuthUser,
  performLogin,
  performOperatorLogin,
  performParentLogin,
  performSchoolLogin,
  parseDurationToMs,
} as const;

// ---------------------------------------------------------------------------
// Admin staff-view preview (#2893)
//
// While a preview is active, `token.token` holds the READ-ONLY preview JWT
// (the target staff member's identity + permissions, minted by the backend),
// the admin's own access token is parked in `previewAdminToken`, and
// `refreshToken` stays the admin's. The preview lives only as long as the
// admin session: near expiry the callback refreshes the ADMIN token and asks
// the backend for a fresh preview token — no refresh token ever exists in
// the target's name.
// ---------------------------------------------------------------------------

const PREVIEW_REFRESH_BUFFER_MS = 5 * 60 * 1000;
const PREVIEW_ADMIN_TOKEN_MIN_TTL_MS = 60 * 1000;
const PREVIEW_REMINT_TIMEOUT_MS = 5_000;

interface PreviewStartUpdate {
  accessToken: string;
  expiresIn: number;
  // Account IDs are int64 on the backend and stay strings on this side —
  // a JavaScript number would round IDs beyond 2^53.
  targetAccountId: string;
  targetName: string;
}

function isPreviewStartUpdate(value: unknown): value is PreviewStartUpdate {
  if (typeof value !== "object" || value === null) return false;
  const v = value as Record<string, unknown>;
  return (
    typeof v.accessToken === "string" &&
    typeof v.expiresIn === "number" &&
    typeof v.targetAccountId === "string" &&
    v.targetAccountId.length > 0 &&
    typeof v.targetName === "string"
  );
}

function isPreviewActive(token: Record<string, unknown>): boolean {
  return typeof token.previewTargetAccountId === "string";
}

/** Swap the session onto the preview token, parking the admin's own state. */
function applyPreviewStart(
  token: Record<string, unknown>,
  update: PreviewStartUpdate,
): void {
  const payload = parseJwtPayload(update.accessToken);
  // Only genuine read-only preview tokens may be injected here — anything
  // else would smuggle a full session under the preview flag.
  if (!payload?.read_only) {
    logger.warn("staff_preview_start_rejected", {
      reason: "token is not a read-only preview token",
    });
    return;
  }
  token.previewAdminToken = token.token;
  token.previewAdminTokenExpiry = token.tokenExpiry;
  token.previewAdminId = token.id;
  token.previewAdminEmail = token.email;
  token.previewTargetAccountId = update.targetAccountId;
  token.previewTargetName = update.targetName;
  token.token = update.accessToken;
  token.tokenExpiry = Date.now() + update.expiresIn * 1000;
  token.id = String(payload.id);
  if (payload.sub) token.email = payload.sub;
  syncTokenFromPayload(token, payload);
  logger.info("staff_preview_started", {
    target_account_id: update.targetAccountId,
  });
}

/** Restore the parked admin state and clear every preview field. */
function restoreAdminFromPreview(token: Record<string, unknown>): void {
  token.token = (token.previewAdminToken as string | undefined) ?? "";
  token.tokenExpiry = token.previewAdminTokenExpiry;
  token.id = token.previewAdminId;
  token.email = token.previewAdminEmail;
  const adminToken = token.token as string;
  const payload = adminToken ? parseJwtPayload(adminToken) : null;
  if (payload) syncTokenFromPayload(token, payload);
  token.previewTargetAccountId = undefined;
  token.previewTargetName = undefined;
  token.previewAdminToken = undefined;
  token.previewAdminTokenExpiry = undefined;
  token.previewAdminId = undefined;
  token.previewAdminEmail = undefined;
  logger.info("staff_preview_ended");
}

/**
 * Refresh the ADMIN's token pair while a preview is active. Shares the
 * module-level cache and in-flight dedup with the regular refresh path (same
 * key scheme), so concurrent callbacks never rotate the same refresh token
 * twice.
 */
async function refreshAdminTokenForPreview(
  token: Record<string, unknown>,
): Promise<RefreshAttempt> {
  const currentRefreshToken = token.refreshToken as string | undefined;
  if (!currentRefreshToken) return { status: "terminal" };
  const currentRecoveryProof =
    typeof token.refreshRecoveryProof === "string" &&
    token.refreshRecoveryProof.length > 0
      ? token.refreshRecoveryProof
      : deriveRefreshRecoveryProof(currentRefreshToken);
  token.refreshRecoveryProof = currentRecoveryProof;
  const refreshKey = `${currentRefreshToken}\0${currentRecoveryProof}`;

  pruneRefreshCache();

  const cached = refreshCacheMap.get(refreshKey);
  if (cached && Date.now() < cached.expiresAt) {
    return {
      status: "success",
      result: cached.result,
      recoveryProof: cached.recoveryProof,
      refreshTokenExpiresAt: cached.refreshTokenExpiresAt,
    };
  }

  const inflight = activeRefreshes.get(refreshKey);
  if (inflight) return inflight;

  const refreshPromise = (async (): Promise<RefreshAttempt> => {
    try {
      const auditHeaders = await getRefreshAuditHeaders();
      const response = await fetch(`${getServerApiUrl()}/auth/refresh`, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${currentRefreshToken}`,
          "Content-Type": "application/json",
          "X-Refresh-Recovery-Proof": currentRecoveryProof,
          ...auditHeaders,
        },
        signal: AbortSignal.timeout(PREVIEW_REMINT_TIMEOUT_MS),
      });

      if (response.ok) {
        const tokens = (await response.json()) as RefreshResult;
        const successorRecoveryProof = deriveRefreshRecoveryProof(
          tokens.refresh_token,
        );
        const successorExpiresAt = refreshTokenExpiresAt(tokens.refresh_token);
        if (successorExpiresAt === null) {
          return { status: "transient" };
        }
        refreshCacheMap.set(refreshKey, {
          result: tokens,
          recoveryProof: successorRecoveryProof,
          refreshTokenExpiresAt: successorExpiresAt,
          expiresAt: Date.now() + REFRESH_CACHE_TTL_MS,
        });
        return {
          status: "success",
          result: tokens,
          recoveryProof: successorRecoveryProof,
          refreshTokenExpiresAt: successorExpiresAt,
        };
      }
      logger.warn("staff_preview_admin_refresh_failed", {
        status: response.status,
      });
      return response.status === 401 || response.status === 403
        ? { status: "terminal" }
        : { status: "transient" };
    } catch (err) {
      logger.warn("staff_preview_admin_refresh_error", {
        error: err instanceof Error ? err.message : String(err),
      });
      return { status: "transient" };
    } finally {
      activeRefreshes.delete(refreshKey);
    }
  })();

  activeRefreshes.set(refreshKey, refreshPromise);
  return refreshPromise;
}

/**
 * Record the end of a preview that the SESSION ended on its own — the target
 * lost eligibility, or the admin session itself died. The interactive
 * "Vorschau beenden" button posts the same call from the browser; this is the
 * same ending, just without a click, and it must reach the audit trail the
 * same way. The endpoint is public and token-proved: the signed preview token
 * is the credential, so the end is recordable even when every admin token has
 * expired (laptop closed for a week mid-preview). Best effort: the preview is
 * over regardless of what this call answers, and the backend records one end
 * per preview instance however often it is asked (unique index on the
 * preview id).
 */
async function recordPreviewEnd(
  previewToken: string | undefined,
): Promise<void> {
  if (!previewToken) return;
  try {
    const response = await fetch(
      `${getServerApiUrl()}/auth/staff-preview/end`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ preview_token: previewToken }),
        signal: AbortSignal.timeout(PREVIEW_REMINT_TIMEOUT_MS),
      },
    );
    if (!response.ok) {
      logger.warn("staff_preview_auto_end_not_recorded", {
        status: response.status,
      });
    }
  } catch (err) {
    logger.warn("staff_preview_auto_end_error", {
      error: err instanceof Error ? err.message : String(err),
    });
  }
}

/**
 * End a preview because the SESSION is lost, not because anyone clicked:
 * the admin refresh failed terminally or the refresh token expired. The
 * session callback strips the token right after, so this is the last moment
 * the preview token is in hand — without recording here, the audit trail
 * keeps a "started" that never ends.
 *
 * Best effort, in this order: record first (the signed preview token is the
 * credential — no admin token needs to be alive for it), then restore the
 * admin state so no preview field survives into the dead session.
 */
async function endPreviewOnSessionLoss(
  token: Record<string, unknown>,
): Promise<void> {
  await recordPreviewEnd(token.token as string | undefined);
  restoreAdminFromPreview(token);
}

/**
 * Keep an active preview session alive: refresh the admin token when needed
 * and re-mint the preview token near its expiry. Returns true while the
 * preview stays active (caller returns the token) and false when the preview
 * ended (caller falls through to the regular refresh path with the restored
 * admin state).
 */
async function maintainPreviewSession(
  token: Record<string, unknown>,
  now: number,
): Promise<boolean> {
  const previewExpiry = (token.tokenExpiry as number | undefined) ?? 0;
  if (now < previewExpiry - PREVIEW_REFRESH_BUFFER_MS) return true;

  // Step 1: a fresh admin access token to re-mint with.
  let adminAccessToken = token.previewAdminToken as string | undefined;
  const adminExpiry =
    (token.previewAdminTokenExpiry as number | undefined) ?? 0;
  if (!adminAccessToken || now > adminExpiry - PREVIEW_ADMIN_TOKEN_MIN_TTL_MS) {
    const attempt = await refreshAdminTokenForPreview(token);
    if (attempt.status === "success") {
      token.refreshToken = attempt.result.refresh_token;
      token.refreshRecoveryProof = attempt.recoveryProof;
      token.refreshTokenExpiry = attempt.refreshTokenExpiresAt;
      token.previewAdminToken = attempt.result.access_token;
      token.previewAdminTokenExpiry = Date.now() + accessTokenExpiry;
      adminAccessToken = attempt.result.access_token;
    } else if (attempt.status === "terminal") {
      // The admin session itself is gone — everything ends, preview included.
      // The preview still gets its end event before the session is stripped.
      await endPreviewOnSessionLoss(token);
      token.error = "RefreshTokenError";
      token.needsRefresh = true;
      return true;
    } else {
      return true; // transient — retry on the next callback
    }
  }

  // Step 2: re-mint the preview token as the admin. No dedup needed — the
  // mint does not rotate anything, a concurrent double-mint is harmless.
  try {
    const response = await fetch(`${getServerApiUrl()}/auth/staff-preview`, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${adminAccessToken}`,
        "Content-Type": "application/json",
      },
      // The expiring token comes along as previous_token: it proves this is
      // the SAME preview instance, so the backend renews it instead of
      // recording a second "preview started" in the audit trail.
      body: JSON.stringify({
        account_id: token.previewTargetAccountId,
        previous_token: token.token,
      }),
      signal: AbortSignal.timeout(PREVIEW_REMINT_TIMEOUT_MS),
    });

    if (response.ok) {
      const data = (await response.json()) as {
        access_token: string;
        expires_in: number;
        target_name: string;
      };
      token.token = data.access_token;
      token.tokenExpiry = Date.now() + data.expires_in * 1000;
      token.previewTargetName = data.target_name;
      const payload = parseJwtPayload(data.access_token);
      if (payload) {
        token.id = String(payload.id);
        if (payload.sub) token.email = payload.sub;
        syncTokenFromPayload(token, payload);
      }
      logger.info("staff_preview_token_reminted");
      return true;
    }

    if (response.status === 401) {
      // Stale admin access token — drop it so the next callback refreshes.
      token.previewAdminToken = undefined;
      token.previewAdminTokenExpiry = undefined;
      return true;
    }

    // 403/404/…: the target is no longer previewable (deactivated, role
    // changed, admin rights revoked) — end the preview, back to admin. This
    // ending is as real as one the admin clicks, so it gets the same audit
    // entry: without it the trail would show a preview that started and never
    // ended. Recorded BEFORE the restore, while the preview token is still in
    // hand — it is the proof of which preview is being closed.
    logger.warn("staff_preview_remint_rejected", { status: response.status });
    await recordPreviewEnd(token.token as string | undefined);
    restoreAdminFromPreview(token);
    return false;
  } catch (err) {
    logger.warn("staff_preview_remint_error", {
      error: err instanceof Error ? err.message : String(err),
    });
    return true; // transient — retry on the next callback
  }
}

// ---------------------------------------------------------------------------
// Shared callbacks
// ---------------------------------------------------------------------------

export const sharedRedirectCallback: NonNullable<
  NonNullable<NextAuthConfig["callbacks"]>["redirect"]
> = ({ url, baseUrl }) => {
  if (url.startsWith("/")) return `${baseUrl}${url}`;

  const urlObj = new URL(url);
  const baseObj = new URL(baseUrl);

  if (urlObj.origin === baseObj.origin) return url;

  if (
    urlObj.hostname.endsWith("localhost") &&
    baseObj.hostname.endsWith("localhost")
  ) {
    return url;
  }

  const getParentDomain = (hostname: string) => {
    const parts = hostname.split(".");
    return parts.length > 2 ? parts.slice(-2).join(".") : hostname;
  };
  if (getParentDomain(urlObj.hostname) === getParentDomain(baseObj.hostname)) {
    return url;
  }

  logger.warn("redirect_blocked", {
    url,
    baseUrl,
    reason: "origin mismatch",
  });
  return baseUrl;
};

export const operatorRedirectCallback: NonNullable<
  NonNullable<NextAuthConfig["callbacks"]>["redirect"]
> = ({ url, baseUrl }) => {
  if (url.startsWith("/")) return `${baseUrl}${url}`;

  const urlObj = new URL(url);
  const baseObj = new URL(baseUrl);

  if (urlObj.origin === baseObj.origin) return url;

  if (
    urlObj.hostname.endsWith("localhost") &&
    baseObj.hostname.endsWith("localhost")
  ) {
    return url;
  }

  // Allow when one hostname is a subdomain of the other
  // (e.g. operator.staging.moto-app.de is a child of staging.moto-app.de)
  if (
    urlObj.hostname.endsWith(`.${baseObj.hostname}`) ||
    baseObj.hostname.endsWith(`.${urlObj.hostname}`)
  ) {
    return url;
  }

  logger.warn("operator_redirect_blocked", {
    url,
    baseUrl,
    reason: "origin mismatch",
  });
  return baseUrl;
};

/**
 * School sessions are host-only, so their redirects must stay on that exact
 * origin. Parsing relative URLs before comparing origins also rejects
 * protocol-relative paths such as "//other-host.example".
 */
export const schoolRedirectCallback: NonNullable<
  NonNullable<NextAuthConfig["callbacks"]>["redirect"]
> = ({ url, baseUrl }) => {
  try {
    const redirectUrl = url.startsWith("/")
      ? new URL(url, baseUrl)
      : new URL(url);
    const baseUrlObject = new URL(baseUrl);

    if (redirectUrl.origin === baseUrlObject.origin) return redirectUrl.href;
  } catch {
    // An invalid callback target is handled like any other foreign origin.
  }

  logger.warn("school_redirect_blocked", {
    url,
    baseUrl,
    reason: "origin mismatch",
  });
  return baseUrl;
};

export const sharedJwtCallback: NonNullable<
  NonNullable<NextAuthConfig["callbacks"]>["jwt"]
> = async ({ token, user, trigger, session }) => {
  const isDev = process.env.NODE_ENV === "development";

  if (isDev) {
    const callerId = `jwt-callback-${Date.now()}-${Math.random().toString(36).slice(2, 11)}`;
    const stack = new Error("Stack trace for caller identification").stack;
    const caller = stack?.split("\n")[3]?.trim() ?? "Unknown caller";
    logger.debug("jwt callback invoked", {
      caller_id: callerId,
      caller,
      has_user: !!user,
      has_refresh_token: !!token.refreshToken,
      token_expiry: token.tokenExpiry
        ? new Date(token.tokenExpiry as number).toISOString()
        : "not set",
    });
  }

  // Initial sign in
  if (user) {
    token.id = user.id;
    token.name = user.name;
    token.email = user.email;
    token.token = user.token ?? "";
    token.refreshToken = user.refreshToken ?? "";
    token.roles = user.roles;
    token.permissions = user.permissions;
    token.firstName = user.firstName;
    token.isAdmin = user.isAdmin;
    token.tenantId = user.tenantId;
    token.orgId = user.orgId;
    token.scope = user.scope;
    token.tokenExpiry = Date.now() + accessTokenExpiry;
    token.refreshTokenExpiry = Date.now() + refreshTokenExpiry;
    token.refreshRecoveryProof = createRefreshRecoveryProof();
    token.error = undefined;
    token.needsRefresh = undefined;

    if (isDev) {
      logger.debug("authentication token configuration", {
        access_token_expiry: env.AUTH_JWT_EXPIRY,
        access_expires_at: new Date(token.tokenExpiry as number).toISOString(),
        refresh_token_expiry: env.AUTH_JWT_REFRESH_EXPIRY,
        refresh_expires_at: new Date(
          token.refreshTokenExpiry as number,
        ).toISOString(),
      });
    }
  }

  // Refresh buffer: how far before access token expiry we proactively refresh.
  // Also used to force an immediate refresh after email change confirmation.
  const REFRESH_BUFFER_MS = 5 * 60 * 1000;

  // Handle client-side session update (e.g. profile name or email change)
  if (trigger === "update" && session) {
    const update = session as Record<string, unknown>;
    if (typeof update.name === "string") {
      token.name = update.name;
    }
    if (typeof update.email === "string") {
      token.email = update.email;
    }
    if (update.emailChanged === true) {
      // Force proactive refresh on the next JWT callback by placing
      // tokenExpiry just inside the REFRESH_BUFFER_MS window (1min margin).
      // The refresh re-reads operator.Email from the DB, syncing the
      // session to the newly confirmed email. If refresh fails, the
      // token is still in the future (now < tokenExpiry), so
      // RefreshTokenError is NOT set and the user keeps their session.
      token.tokenExpiry = Date.now() + (REFRESH_BUFFER_MS - 60_000);
    }
    // Admin staff-view preview (#2893): enter with the freshly minted
    // read-only token, leave by restoring the parked admin state. A start
    // while a preview is already active is ignored — it would overwrite the
    // parked admin tokens with preview tokens.
    if (
      update.previewStart !== undefined &&
      !isPreviewActive(token) &&
      isPreviewStartUpdate(update.previewStart)
    ) {
      applyPreviewStart(token, update.previewStart);
    }
    if (update.previewEnd === true && isPreviewActive(token)) {
      restoreAdminFromPreview(token);
    }
  }

  // Check if refresh token is expired
  if (
    token.refreshTokenExpiry &&
    Date.now() > (token.refreshTokenExpiry as number)
  ) {
    logger.warn("refresh token expired", {
      expires_at: new Date(token.refreshTokenExpiry as number).toISOString(),
    });
    // A running preview ends with the session it hangs on — and it ends in
    // the audit trail too, before the session callback drops the token.
    if (isPreviewActive(token)) {
      await endPreviewOnSessionLoss(token);
    }
    token.error = "RefreshTokenExpired";
    token.needsRefresh = true;
    return token;
  }

  // RefreshTokenError is terminal and is set only after the backend has
  // explicitly rejected the refresh session (401/403) after access expiry.
  // Timeouts, network interruptions, and 5xx responses preserve the refresh
  // token so a sleeping/offline tablet can retry when connectivity returns.
  // Once terminal, the session callback strips both tokens and requires login.
  if (token.error === "RefreshTokenError") {
    return token;
  }

  // Admin staff-view preview (#2893): while active, the preview branch owns
  // token maintenance (admin refresh + preview re-mint). Only when the
  // preview just ended does control fall through, so the restored admin
  // token can refresh below if it is stale.
  if (isPreviewActive(token)) {
    const stillPreviewing = await maintainPreviewSession(token, Date.now());
    if (stillPreviewing) {
      return token;
    }
  }

  // Proactive token refresh
  const REFRESH_TIMEOUT_MS = 5_000;

  const now = Date.now();
  const tokenExpiry = token.tokenExpiry as number;
  if (
    token.tokenExpiry &&
    token.refreshToken &&
    token.refreshTokenExpiry &&
    now > tokenExpiry - REFRESH_BUFFER_MS &&
    now < (token.refreshTokenExpiry as number)
  ) {
    const currentRefreshToken = token.refreshToken as string;
    const currentRecoveryProof =
      typeof token.refreshRecoveryProof === "string" &&
      token.refreshRecoveryProof.length > 0
        ? token.refreshRecoveryProof
        : deriveRefreshRecoveryProof(currentRefreshToken);
    token.refreshRecoveryProof = currentRecoveryProof;
    const refreshKey = `${currentRefreshToken}\0${currentRecoveryProof}`;

    // Periodic cleanup of expired cache entries
    pruneRefreshCache();

    // Check per-token cache
    const cached = refreshCacheMap.get(refreshKey);
    if (cached && Date.now() < cached.expiresAt) {
      token.token = cached.result.access_token;
      token.refreshToken = cached.result.refresh_token;
      token.refreshRecoveryProof = cached.recoveryProof;
      token.tokenExpiry = Date.now() + accessTokenExpiry;
      token.refreshTokenExpiry = cached.refreshTokenExpiresAt;
      token.error = undefined;
      token.needsRefresh = undefined;
      const cachedPayload = parseJwtPayload(cached.result.access_token);
      if (cachedPayload) {
        syncTokenFromPayload(token, cachedPayload);
      }
      logger.info("proactive_token_refresh_deduplicated");
      return token;
    }

    // Join in-flight refresh for the SAME token (per-token dedup)
    const inflight = activeRefreshes.get(refreshKey);
    if (inflight) {
      const attempt = await inflight;
      if (attempt.status === "success") {
        token.token = attempt.result.access_token;
        token.refreshToken = attempt.result.refresh_token;
        token.refreshRecoveryProof = attempt.recoveryProof;
        token.tokenExpiry = Date.now() + accessTokenExpiry;
        token.refreshTokenExpiry = attempt.refreshTokenExpiresAt;
        token.error = undefined;
        token.needsRefresh = undefined;
        const inflightPayload = parseJwtPayload(attempt.result.access_token);
        if (inflightPayload) {
          syncTokenFromPayload(token, inflightPayload);
        }
        logger.info("proactive_token_refresh_succeeded");
      } else if (attempt.status === "terminal" && now > tokenExpiry) {
        token.error = "RefreshTokenError";
        token.needsRefresh = true;
        logger.warn("token_refresh_terminal_failure_post_expiry");
      } else if (attempt.status === "transient") {
        logger.warn("token_refresh_deferred_after_transient_failure");
      }
      return token;
    }

    // Start new refresh (keyed by this specific token)
    const isOperator = token.scope === "platform";
    const refreshUrl = isOperator
      ? `${getServerApiUrl()}/operator/auth/refresh`
      : `${getServerApiUrl()}/auth/refresh`;

    const refreshPromise = (async (): Promise<RefreshAttempt> => {
      try {
        const auditHeaders = await getRefreshAuditHeaders();
        const response = await fetch(refreshUrl, {
          method: "POST",
          headers: {
            Authorization: `Bearer ${currentRefreshToken}`,
            "Content-Type": "application/json",
            "X-Refresh-Recovery-Proof": currentRecoveryProof,
            ...auditHeaders,
          },
          signal: AbortSignal.timeout(REFRESH_TIMEOUT_MS),
        });

        if (response.ok) {
          let tokens: RefreshResult;
          if (isOperator) {
            const envelope = (await response.json()) as {
              data: RefreshResult;
            };
            tokens = envelope.data;
          } else {
            tokens = (await response.json()) as RefreshResult;
          }
          const successorRecoveryProof = deriveRefreshRecoveryProof(
            tokens.refresh_token,
          );
          const successorExpiresAt = refreshTokenExpiresAt(
            tokens.refresh_token,
          );
          if (successorExpiresAt === null) {
            return { status: "transient" };
          }
          refreshCacheMap.set(refreshKey, {
            result: tokens,
            recoveryProof: successorRecoveryProof,
            refreshTokenExpiresAt: successorExpiresAt,
            expiresAt: Date.now() + REFRESH_CACHE_TTL_MS,
          });
          return {
            status: "success",
            result: tokens,
            recoveryProof: successorRecoveryProof,
            refreshTokenExpiresAt: successorExpiresAt,
          };
        }
        logger.warn("proactive_token_refresh_failed", {
          status: response.status,
          scope: token.scope,
        });
        return response.status === 401 || response.status === 403
          ? { status: "terminal" }
          : { status: "transient" };
      } catch (err) {
        logger.warn("proactive_token_refresh_error", {
          error: err instanceof Error ? err.message : String(err),
          scope: token.scope,
        });
        return { status: "transient" };
      } finally {
        activeRefreshes.delete(refreshKey);
      }
    })();

    activeRefreshes.set(refreshKey, refreshPromise);

    const attempt = await refreshPromise;
    if (attempt.status === "success") {
      token.token = attempt.result.access_token;
      token.refreshToken = attempt.result.refresh_token;
      token.refreshRecoveryProof = attempt.recoveryProof;
      token.tokenExpiry = Date.now() + accessTokenExpiry;
      token.refreshTokenExpiry = attempt.refreshTokenExpiresAt;
      token.error = undefined;
      token.needsRefresh = undefined;
      const refreshedPayload = parseJwtPayload(attempt.result.access_token);
      if (refreshedPayload) {
        syncTokenFromPayload(token, refreshedPayload);
      }
      logger.info("proactive_token_refresh_succeeded");
    } else if (attempt.status === "terminal" && now > tokenExpiry) {
      token.error = "RefreshTokenError";
      token.needsRefresh = true;
      logger.warn("token_refresh_terminal_failure_post_expiry");
    } else if (attempt.status === "transient") {
      logger.warn("token_refresh_deferred_after_transient_failure");
    }
  }

  return token;
};

export const sharedSessionCallback: NonNullable<
  NonNullable<NextAuthConfig["callbacks"]>["session"]
> = ({ session, token }) => {
  if (
    token.error === "RefreshTokenExpired" ||
    token.error === "RefreshTokenError" ||
    !token.token
  ) {
    return {
      ...session,
      user: {
        ...session.user,
        id: (token.id as string) || "",
        email: token.email ?? "",
        token: "",
        refreshToken: "",
        roles: [],
        permissions: [],
        firstName: (token.firstName as string) || "",
        isAdmin: false,
        scope: token.scope as string | undefined,
      },
      error: token.error,
    };
  }

  return {
    ...session,
    user: {
      ...session.user,
      id: token.id as string,
      email: token.email ?? "",
      token: token.token as string,
      refreshToken: token.refreshToken as string,
      roles: token.roles as string[],
      permissions: token.permissions as string[],
      firstName: token.firstName as string,
      isAdmin: (token.isAdmin as boolean) ?? false,
      tenantId: token.tenantId as number | undefined,
      orgId: token.orgId as number | undefined,
      scope: token.scope as string | undefined,
      isPreview: isPreviewActive(token),
      previewTargetName: token.previewTargetName as string | undefined,
      previewTargetAccountId: token.previewTargetAccountId as
        string | undefined,
    },
  };
};
