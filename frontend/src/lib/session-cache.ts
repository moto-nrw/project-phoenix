// lib/session-cache.ts
// Caches getSession() results to avoid redundant calls when multiple
// service files fetch data in parallel (e.g. 5 parallel fetches = 1 session call).

import { getSession } from "next-auth/react";

let cached: {
  session: Awaited<ReturnType<typeof getSession>>;
  expiry: number;
} | null = null;
let inflight: Promise<Awaited<ReturnType<typeof getSession>>> | null = null;

const TTL_MS = 10_000; // 10 second cache window

/**
 * Invalidate the cached session so the next call fetches a fresh one.
 * Call this after a successful token refresh so stale tokens aren't reused.
 */
export function clearSessionCache() {
  cached = null;
}

export async function getCachedSession() {
  const now = Date.now();
  if (cached && now < cached.expiry) return cached.session;
  if (inflight) return inflight;

  inflight = getSession()
    .then((session) => {
      cached = { session, expiry: Date.now() + TTL_MS };
      return session;
    })
    .finally(() => {
      inflight = null;
    });

  return inflight;
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
    const { handleAuthFailure } = await import("./auth-api");
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
