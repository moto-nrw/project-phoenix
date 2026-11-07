import "server-only";
import type { JWT } from "next-auth/jwt";
import { AsyncLocalStorage } from "node:async_hooks";
import { createHmac, randomBytes } from "node:crypto";
import { env } from "~/env";
import { canonicalForwardedFor } from "~/lib/client-headers.server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { authLogger, parseJwtPayload } from "./jwt-claims";
import type { BackendEndpoint } from "./portals";

// The backend refuses an unexpired access token once its refresh-token family
// is revoked (#5194): logout in another tab, or a cookie a late response wrote
// back. Inside this scope the JWT callback renews at once instead of waiting
// for the expiry. A revoked family then fails its refresh and ends the session
// (RefreshTokenError), so the browser is sent to the login instead of being
// shown a session that opens nothing; a live family continues with fresh
// tokens.
const forcedRenewal = new AsyncLocalStorage<true>();

/** Runs a session read that must renew the session first. */
export function withForcedSessionRenewal<T>(
  read: () => Promise<T>,
): Promise<T> {
  return forcedRenewal.run(true, read);
}

export function isRenewalForced(): boolean {
  return forcedRenewal.getStore() === true;
}

export interface TokenPair {
  access_token: string;
  refresh_token: string;
}

export type Rotation =
  | {
      status: "success";
      result: TokenPair;
      recoveryProof: string;
      refreshTokenExpiresAt: number;
      /** Another request of this process already rotated this token. */
      shared: boolean;
    }
  | { status: "terminal"; remembered: boolean }
  | { status: "transient" };

const REFRESH_TIMEOUT_MS = 5_000;
const REFRESH_CACHE_TTL_MS = 5 * 60 * 1000;
const MAX_CACHE_ENTRIES = 100;

// Keyed by the OLD refresh token plus its independent recovery proof, so
// unrelated or partially stolen credentials cannot join a legitimate
// in-flight rotation, and one session's entries never serve another.
const activeRefreshes = new Map<string, Promise<Rotation>>();
const refreshCache = new Map<
  string,
  {
    result: TokenPair;
    recoveryProof: string;
    refreshTokenExpiresAt: number;
    expiresAt: number;
  }
>();

// Refresh tokens the backend has explicitly rejected (401/403). A rejected
// token never becomes valid again, and the session callback's terminal error
// only reaches the cookie from a context that may set cookies. Until then,
// every server-component request would repeat the doomed refresh (#2952);
// this memo makes each dead token cost one backend call.
const terminalRefreshKeys = new Map<string, number>();

/** @internal Test isolation only: the keys already isolate sessions. */
export function _resetRefreshState(): void {
  activeRefreshes.clear();
  refreshCache.clear();
  terminalRefreshKeys.clear();
}

export function createRefreshRecoveryProof(): string {
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

function recoveryProofOf(token: JWT, refreshToken: string): string {
  return typeof token.refreshRecoveryProof === "string" &&
    token.refreshRecoveryProof.length > 0
    ? token.refreshRecoveryProof
    : deriveRefreshRecoveryProof(refreshToken);
}

function refreshTokenExpiresAt(refreshToken: string): number | null {
  const payload = parseJwtPayload(refreshToken);
  if (
    !payload ||
    typeof payload.exp !== "number" ||
    !Number.isSafeInteger(payload.exp) ||
    payload.exp <= 0
  ) {
    authLogger.error("refresh token is missing a valid exp claim", {});
    return null;
  }
  return payload.exp * 1000;
}

/** Client IP and User-Agent of the request being handled, if any. */
export async function getRefreshAuditHeaders(): Promise<
  Record<string, string>
> {
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

function pruneRefreshCache(): void {
  const now = Date.now();
  for (const [key, entry] of refreshCache) {
    if (now >= entry.expiresAt) refreshCache.delete(key);
  }
  for (const [key, expiresAt] of terminalRefreshKeys) {
    if (now >= expiresAt) terminalRefreshKeys.delete(key);
  }
  if (refreshCache.size > MAX_CACHE_ENTRIES) {
    const excess = refreshCache.size - MAX_CACHE_ENTRIES;
    const keys = refreshCache.keys();
    for (let i = 0; i < excess; i++) {
      const next = keys.next();
      if (!next.done) refreshCache.delete(next.value);
    }
  }
}

// A 401/403 from the refresh endpoint is final; everything else may be retried.
// The dead token is remembered for as long as a cookie could still carry it.
function rememberTerminal(refreshKey: string, refreshTokenExpiry: unknown) {
  terminalRefreshKeys.set(
    refreshKey,
    typeof refreshTokenExpiry === "number"
      ? refreshTokenExpiry
      : Date.now() + REFRESH_CACHE_TTL_MS,
  );
  if (terminalRefreshKeys.size > MAX_CACHE_ENTRIES) {
    const oldest = terminalRefreshKeys.keys().next();
    if (!oldest.done) terminalRefreshKeys.delete(oldest.value);
  }
}

function isKnownTerminalRefresh(refreshKey: string): boolean {
  const expiresAt = terminalRefreshKeys.get(refreshKey);
  return expiresAt !== undefined && Date.now() < expiresAt;
}

/** The backend already refused to renew the cookie's refresh token. */
export function refreshAlreadyRefused(token: JWT): boolean {
  const refreshToken = token.refreshToken;
  if (typeof refreshToken !== "string" || refreshToken.length === 0) {
    return false;
  }
  return isKnownTerminalRefresh(
    `${refreshToken}\0${recoveryProofOf(token, refreshToken)}`,
  );
}

async function fetchRotation(
  endpoint: BackendEndpoint,
  refreshToken: string,
  recoveryProof: string,
  refreshKey: string,
  token: JWT,
): Promise<Rotation> {
  try {
    const response = await fetch(`${getServerApiUrl()}${endpoint.path}`, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${refreshToken}`,
        "Content-Type": "application/json",
        "X-Refresh-Recovery-Proof": recoveryProof,
        ...(await getRefreshAuditHeaders()),
      },
      signal: AbortSignal.timeout(REFRESH_TIMEOUT_MS),
    });
    if (!response.ok) {
      authLogger.warn("proactive_token_refresh_failed", {
        status: response.status,
        scope: token.scope,
      });
      if (response.status === 401 || response.status === 403) {
        rememberTerminal(refreshKey, token.refreshTokenExpiry);
        return { status: "terminal", remembered: false };
      }
      return { status: "transient" };
    }
    const body = (await response.json()) as TokenPair | { data: TokenPair };
    const tokens = endpoint.enveloped
      ? (body as { data: TokenPair }).data
      : (body as TokenPair);
    const successorExpiresAt = refreshTokenExpiresAt(tokens.refresh_token);
    if (successorExpiresAt === null) return { status: "transient" };
    const successorProof = deriveRefreshRecoveryProof(tokens.refresh_token);
    refreshCache.set(refreshKey, {
      result: tokens,
      recoveryProof: successorProof,
      refreshTokenExpiresAt: successorExpiresAt,
      expiresAt: Date.now() + REFRESH_CACHE_TTL_MS,
    });
    return {
      status: "success",
      result: tokens,
      recoveryProof: successorProof,
      refreshTokenExpiresAt: successorExpiresAt,
      shared: false,
    };
  } catch (err) {
    authLogger.warn("proactive_token_refresh_error", {
      error: err instanceof Error ? err.message : String(err),
      scope: token.scope,
    });
    return { status: "transient" };
  } finally {
    activeRefreshes.delete(refreshKey);
  }
}

/**
 * Rotates the session's refresh token once per process: a rotation another
 * request of this process already did or is doing is shared, and a token the
 * backend refused is not sent again. The regular session and the admin
 * session under a staff preview both rotate through here.
 */
export async function rotateRefreshToken(
  token: JWT,
  endpoint: BackendEndpoint,
): Promise<Rotation> {
  const refreshToken = token.refreshToken;
  if (typeof refreshToken !== "string" || !refreshToken) {
    return { status: "terminal", remembered: false };
  }
  const recoveryProof = recoveryProofOf(token, refreshToken);
  token.refreshRecoveryProof = recoveryProof;
  const refreshKey = `${refreshToken}\0${recoveryProof}`;

  pruneRefreshCache();

  const cached = refreshCache.get(refreshKey);
  if (cached && Date.now() < cached.expiresAt) {
    return {
      status: "success",
      result: cached.result,
      recoveryProof: cached.recoveryProof,
      refreshTokenExpiresAt: cached.refreshTokenExpiresAt,
      shared: true,
    };
  }
  if (isKnownTerminalRefresh(refreshKey)) {
    return { status: "terminal", remembered: true };
  }
  const inflight = activeRefreshes.get(refreshKey);
  if (inflight) return inflight;

  const rotation = fetchRotation(
    endpoint,
    refreshToken,
    recoveryProof,
    refreshKey,
    token,
  );
  activeRefreshes.set(refreshKey, rotation);
  return rotation;
}

/** Puts a successful rotation's refresh side on the session. */
export function applyRotation(
  token: JWT,
  rotation: Extract<Rotation, { status: "success" }>,
): string {
  token.refreshToken = rotation.result.refresh_token;
  token.refreshRecoveryProof = rotation.recoveryProof;
  token.refreshTokenExpiry = rotation.refreshTokenExpiresAt;
  return rotation.result.access_token;
}

/**
 * Redeems the hand-off token of an Absprung from the Träger portal (#4654)
 * on the school's host. The token is a short-lived refresh token of a new
 * OGS session; its first rotation here makes it single-use and gives the
 * session its normal lifetime. Its recovery proof is random, so a replay of
 * the token is refused even within the recovery window. Null when the
 * backend refuses it.
 */
export async function redeemTraegerHandoff(
  handoffToken: string,
  endpoint: BackendEndpoint,
): Promise<TokenPair | null> {
  if (
    typeof handoffToken !== "string" ||
    !handoffToken ||
    handoffToken.length > 16000
  ) {
    return null;
  }
  try {
    const response = await fetch(`${getServerApiUrl()}${endpoint.path}`, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${handoffToken}`,
        "Content-Type": "application/json",
        // A fresh proof per redemption: the hand-off token alone, replayed
        // within the recovery window, must not recover the session it
        // opened. A lost answer means opening the school again.
        "X-Refresh-Recovery-Proof": createRefreshRecoveryProof(),
        ...(await getRefreshAuditHeaders()),
      },
      cache: "no-store",
      signal: AbortSignal.timeout(REFRESH_TIMEOUT_MS),
    });
    if (!response.ok) {
      authLogger.warn("traeger_handoff_refused", { status: response.status });
      return null;
    }
    return (await response.json()) as TokenPair;
  } catch (err) {
    authLogger.warn("traeger_handoff_failed", {
      error: err instanceof Error ? err.message : String(err),
    });
    return null;
  }
}
