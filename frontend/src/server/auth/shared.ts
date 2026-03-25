/**
 * Shared auth utilities used by both tenant and operator NextAuth configs.
 *
 * Extracted to avoid duplication between tenant-config.ts and operator-config.ts.
 * Contains: JWT parsing, user building, login functions, token refresh logic,
 * callbacks, and NextAuth module augmentation.
 */

import type { DefaultSession, NextAuthConfig, User } from "next-auth";
import { env } from "~/env";
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
  sub?: string;
  username?: string;
  first_name?: string;
  last_name?: string;
  email?: string;
  roles?: string[];
  is_admin?: boolean;
  tenant_id?: number;
  org_id?: number;
  scope?: string;
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
      firstName?: string;
      isAdmin?: boolean;
      tenantId?: number;
      orgId?: number;
      scope?: string;
    } & DefaultSession["user"];
    error?: "RefreshTokenExpired" | "RefreshTokenError";
  }

  interface User {
    token?: string;
    refreshToken?: string;
    roles?: string[];
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
    firstName?: string;
    isAdmin?: boolean;
    tenantId?: number;
    orgId?: number;
    scope?: string;
    tokenExpiry?: number;
    refreshTokenExpiry?: number;
    error?: "RefreshTokenExpired" | "RefreshTokenError";
    needsRefresh?: boolean;
  }
}

// ---------------------------------------------------------------------------
// Utility functions
// ---------------------------------------------------------------------------

export function parseDurationToMs(duration: string): number {
  const regex = /^(\d+)([hm])$/;
  const match = regex.exec(duration);
  if (!match) return 12 * 60 * 60 * 1000; // 12h default
  const amount = match[1]!;
  const unit = match[2]!;
  const num = Number.parseInt(amount, 10);
  return unit === "h" ? num * 60 * 60 * 1000 : num * 60 * 1000;
}

export const accessTokenExpiry = parseDurationToMs(env.AUTH_JWT_EXPIRY);
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

export function buildAuthUser(
  payload: JwtPayload,
  token: string,
  refreshToken: string,
  email: string,
  scope?: string,
): User {
  const roles =
    payload.roles && Array.isArray(payload.roles) ? payload.roles : [];

  return {
    id: String(payload.id),
    name: buildDisplayName(payload, email),
    email: email,
    token: token,
    refreshToken: refreshToken,
    roles: scope === "platform" ? ["operator"] : roles,
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

// Per-token maps: keyed by the OLD refresh token string so that concurrent
// requests from different users (or the same user across tabs) never clobber
// each other's deduplication state.
const activeRefreshes = new Map<string, Promise<RefreshResult | null>>();
const refreshCacheMap = new Map<
  string,
  { result: RefreshResult; expiresAt: number }
>();
const REFRESH_CACHE_TTL_MS = 5 * 60 * 1000;
const MAX_CACHE_ENTRIES = 100;

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
  parseDurationToMs,
} as const;

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

  logger.warn("operator_redirect_blocked", {
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
    token.firstName = user.firstName;
    token.isAdmin = user.isAdmin;
    token.tenantId = user.tenantId;
    token.orgId = user.orgId;
    token.scope = user.scope;
    token.tokenExpiry = Date.now() + accessTokenExpiry;
    token.refreshTokenExpiry = Date.now() + refreshTokenExpiry;
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

  // Handle client-side session update (e.g. profile name change)
  if (trigger === "update" && session) {
    const update = session as Record<string, unknown>;
    if (typeof update.name === "string") {
      token.name = update.name;
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
    token.error = "RefreshTokenExpired";
    token.needsRefresh = true;
    return token;
  }

  // Proactive token refresh
  const REFRESH_BUFFER_MS = 5 * 60 * 1000;
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

    // Periodic cleanup of expired cache entries
    pruneRefreshCache();

    // Check per-token cache
    const cached = refreshCacheMap.get(currentRefreshToken);
    if (cached && Date.now() < cached.expiresAt) {
      token.token = cached.result.access_token;
      token.refreshToken = cached.result.refresh_token;
      token.tokenExpiry = Date.now() + accessTokenExpiry;
      token.refreshTokenExpiry = Date.now() + refreshTokenExpiry;
      token.error = undefined;
      token.needsRefresh = undefined;
      const cachedPayload = parseJwtPayload(cached.result.access_token);
      if (cachedPayload) {
        token.tenantId = cachedPayload.tenant_id;
        token.orgId = cachedPayload.org_id;
        token.roles = cachedPayload.roles ?? [];
        token.isAdmin = cachedPayload.is_admin ?? false;
        token.scope = cachedPayload.scope;
      }
      logger.info("proactive_token_refresh_deduplicated");
      return token;
    }

    // Join in-flight refresh for the SAME token (per-token dedup)
    const inflight = activeRefreshes.get(currentRefreshToken);
    if (inflight) {
      const result = await inflight;
      if (result) {
        token.token = result.access_token;
        token.refreshToken = result.refresh_token;
        token.tokenExpiry = Date.now() + accessTokenExpiry;
        token.refreshTokenExpiry = Date.now() + refreshTokenExpiry;
        token.error = undefined;
        token.needsRefresh = undefined;
        const inflightPayload = parseJwtPayload(result.access_token);
        if (inflightPayload) {
          token.tenantId = inflightPayload.tenant_id;
          token.orgId = inflightPayload.org_id;
          token.roles = inflightPayload.roles ?? [];
          token.isAdmin = inflightPayload.is_admin ?? false;
          token.scope = inflightPayload.scope;
        }
        logger.info("proactive_token_refresh_succeeded");
      } else if (now > tokenExpiry) {
        token.error = "RefreshTokenError";
        token.needsRefresh = true;
        logger.warn("token_refresh_failed_post_expiry");
      }
      return token;
    }

    // Start new refresh (keyed by this specific token)
    const isOperator = token.scope === "platform";
    const refreshUrl = isOperator
      ? `${getServerApiUrl()}/operator/auth/refresh`
      : `${getServerApiUrl()}/auth/refresh`;

    const refreshPromise = (async (): Promise<RefreshResult | null> => {
      try {
        const response = await fetch(refreshUrl, {
          method: "POST",
          headers: {
            Authorization: `Bearer ${currentRefreshToken}`,
            "Content-Type": "application/json",
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
          refreshCacheMap.set(currentRefreshToken, {
            result: tokens,
            expiresAt: Date.now() + REFRESH_CACHE_TTL_MS,
          });
          return tokens;
        }
        logger.warn("proactive_token_refresh_failed", {
          status: response.status,
          scope: token.scope,
        });
        return null;
      } catch (err) {
        logger.warn("proactive_token_refresh_error", {
          error: err instanceof Error ? err.message : String(err),
          scope: token.scope,
        });
        return null;
      } finally {
        activeRefreshes.delete(currentRefreshToken);
      }
    })();

    activeRefreshes.set(currentRefreshToken, refreshPromise);

    const result = await refreshPromise;
    if (result) {
      token.token = result.access_token;
      token.refreshToken = result.refresh_token;
      token.tokenExpiry = Date.now() + accessTokenExpiry;
      token.refreshTokenExpiry = Date.now() + refreshTokenExpiry;
      token.error = undefined;
      token.needsRefresh = undefined;
      const refreshedPayload = parseJwtPayload(result.access_token);
      if (refreshedPayload) {
        token.tenantId = refreshedPayload.tenant_id;
        token.orgId = refreshedPayload.org_id;
        token.roles = refreshedPayload.roles ?? [];
        token.isAdmin = refreshedPayload.is_admin ?? false;
        token.scope = refreshedPayload.scope;
      }
      logger.info("proactive_token_refresh_succeeded");
    } else if (now > tokenExpiry) {
      token.error = "RefreshTokenError";
      token.needsRefresh = true;
      logger.warn("token_refresh_failed_post_expiry");
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
      firstName: token.firstName as string,
      isAdmin: (token.isAdmin as boolean) ?? false,
      tenantId: token.tenantId as number | undefined,
      orgId: token.orgId as number | undefined,
      scope: token.scope as string | undefined,
    },
  };
};
