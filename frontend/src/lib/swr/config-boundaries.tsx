"use client";

/**
 * The only SWR configuration code outside ~/lib/swr may set (#5007). SWR's
 * own SWRConfig hands out the cache and a raw mutate (`defaultValue`,
 * `provider`); these components take data or nothing, never a cache.
 */

// swr/no-direct-import permits package imports inside this shared wrapper only.
import { SWRConfig } from "swr";
import type { ReactNode } from "react";

/**
 * Server seeds of the session's shell under their full scoped keys:
 * `fallback` for the first render, `cacheData` as the mount revalidation.
 * Stamp them with stampSWRSeeds; the read side shows unstamped values only
 * under the fallback rules of the generation lock.
 */
export function SWRSeedBoundary({
  seed,
  children,
}: Readonly<{
  seed: {
    fallback: Record<string, unknown>;
    cacheData: Record<string, unknown>;
  };
  children: ReactNode;
}>) {
  return <SWRConfig value={seed}>{children}</SWRConfig>;
}

/**
 * A subtree with its own empty cache: nothing it loads is shared with the
 * portal, and a remount (another `key`) starts empty again.
 */
export function IsolatedSWRCache({
  children,
}: Readonly<{ children: ReactNode }>) {
  return (
    <SWRConfig value={{ provider: () => new Map() }}>{children}</SWRConfig>
  );
}
