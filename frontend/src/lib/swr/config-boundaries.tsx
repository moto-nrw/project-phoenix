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
  // Copied, never passed on: an object that carries `use` or `provider`
  // besides the seed would reach SWR's configuration (#5007).
  return (
    <SWRConfig value={{ fallback: seed.fallback, cacheData: seed.cacheData }}>
      {children}
    </SWRConfig>
  );
}

/**
 * A subtree with its own empty cache: nothing it loads is shared with the
 * portal, and a remount (another `key`) starts empty again. It takes no
 * configuration; the provider is created here and never handed out.
 */
export function IsolatedSWRCache({
  children,
}: Readonly<{ children: ReactNode }>) {
  return (
    <SWRConfig value={{ provider: () => new Map() }}>{children}</SWRConfig>
  );
}
