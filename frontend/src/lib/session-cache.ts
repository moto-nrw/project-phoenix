// lib/session-cache.ts
// Caches getSession() results to avoid redundant calls when multiple
// service files fetch data in parallel (e.g. 5 parallel fetches = 1 session call).

/**
 * sessionStorage key used to distinguish a deliberate logout from a
 * session-expiry redirect.  Shared between the logout flow
 * (shell-auth-context) and the login page ([tenant]/page.tsx).
 */
export const DELIBERATE_LOGOUT_KEY = "deliberateLogout";

import { getSession } from "next-auth/react";

let cached: {
  session: Awaited<ReturnType<typeof getSession>>;
  expiry: number;
} | null = null;
let inflight: Promise<Awaited<ReturnType<typeof getSession>>> | null = null;

// Track the tenant ID of the cached session.
// When the user switches tenants, the new session will have a different tenant_id,
// which automatically invalidates the stale cache entry.
let cachedTenantId: number | undefined;

const TTL_MS = 10_000; // 10 second cache window

/**
 * Invalidate the cached session so the next call fetches a fresh one.
 * Call this after a successful token refresh or tenant switch so stale tokens aren't reused.
 */
export function clearSessionCache() {
  cached = null;
  cachedTenantId = undefined;
  inflight = null;
}

export async function getCachedSession() {
  const now = Date.now();
  if (cached && now < cached.expiry) {
    // Verify the cached session still belongs to the same tenant.
    // After a tenant switch, the session's tenant_id changes, so a cache miss
    // forces a fresh getSession() with the updated JWT.
    const sessionTenantId = (
      cached.session?.user as { tenantId?: number } | undefined
    )?.tenantId;
    if (sessionTenantId === cachedTenantId) {
      return cached.session;
    }
    // Tenant changed — invalidate
    cached = null;
  }
  if (inflight) return inflight;

  // A request may only touch module state while it is still the current
  // `inflight`. clearSessionCache() (token refresh, tenant switch) nulls
  // `inflight`, so a lookup that was already in flight at that point must
  // neither write its stale, pre-refresh result back into the cache nor
  // clear a newer lookup that started after the invalidation.
  const request: Promise<Awaited<ReturnType<typeof getSession>>> = getSession()
    .then((session) => {
      if (inflight === request) {
        cachedTenantId = (session?.user as { tenantId?: number } | undefined)
          ?.tenantId;
        cached = { session, expiry: Date.now() + TTL_MS };
      }
      return session;
    })
    .finally(() => {
      if (inflight === request) {
        inflight = null;
      }
    });
  inflight = request;

  return request;
}

/**
 * Fetch with automatic session auth and 401 → refresh → retry.
 * Drop-in replacement for `fetch()` that handles expired tokens transparently.
 * On unrecoverable auth failure, signs out via handleAuthFailure().
 */
export async function sessionFetch(
  url: string,
  init?: RequestInit,
): Promise<Response> {
  const session = await getCachedSession();
  const token = session?.user?.token;

  if (!token) {
    throw new Error("No authentication token available");
  }

  const mergedInit: RequestInit = {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init?.headers,
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
  };

  const response = await fetch(url, mergedInit);

  if (response.status === 401) {
    clearSessionCache();
    // Import from auth-failure directly (auth-api only re-exports it):
    // going through auth-api would close a module cycle
    // session-cache -> auth-api -> auth-service -> api-transport -> session-cache
    // now that the API clients import this module statically (#2123).
    const { handleAuthFailure } = await import("./auth-failure");
    const refreshed = await handleAuthFailure();
    if (refreshed) {
      const freshSession = await getCachedSession();
      const freshToken = freshSession?.user?.token;
      return fetch(url, {
        ...init,
        headers: {
          "Content-Type": "application/json",
          ...init?.headers,
          ...(freshToken ? { Authorization: `Bearer ${freshToken}` } : {}),
        },
      });
    }
    // handleAuthFailure already signed out
    throw new Error("Authentication expired");
  }

  return response;
}
