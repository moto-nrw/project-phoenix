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
 * Returns the current session, reusing a cached result within a 10s window.
 * Concurrent calls share the same in-flight promise (request deduplication).
 */
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
