"use client";

import { createContext, useContext, useMemo } from "react";
import type { PresenceMode, TenantInfo } from "~/lib/tenant-api";

interface TenantContextValue {
  /** The tenant slug from the URL path segment */
  tenantSlug: string;
  /** Resolved tenant metadata (null if not yet validated) */
  tenant: TenantInfo | null;
}

// Kept unexported: consumers use the hooks below. An export was tried
// earlier to let sibling modules subscribe directly, but knip flagged it
// as unused (its static analysis doesn't follow `~/` alias imports through
// the hook file reliably), and the global test mock in `src/test/setup.ts`
// doesn't replicate `createContext` output — which broke every test that
// rendered a component reading presence state. Co-locating the hook here
// sidesteps both issues.
const TenantContext = createContext<TenantContextValue | null>(null);

/**
 * Provides tenant context to all child components within the [tenant] route segment.
 * The layout.tsx at [tenant]/layout.tsx wraps its children with this provider.
 */
export function TenantProvider({
  tenantSlug,
  tenant,
  children,
}: {
  tenantSlug: string;
  tenant: TenantInfo | null;
  children: React.ReactNode;
}) {
  const value = useMemo(() => ({ tenantSlug, tenant }), [tenantSlug, tenant]);

  return (
    <TenantContext.Provider value={value}>{children}</TenantContext.Provider>
  );
}

/**
 * Access the current tenant context.
 * Must be used within a TenantProvider (i.e., under the [tenant] route segment).
 */
export function useTenant(): TenantContextValue {
  const ctx = useContext(TenantContext);
  if (!ctx) {
    throw new Error(
      "useTenant must be used within a TenantProvider (under [tenant] route)",
    );
  }
  return ctx;
}

/**
 * Returns the current tenant slug, or null if outside a TenantProvider.
 * Safe to call from any component — never throws. Used by SWR hooks to
 * prefix cache keys for cross-tenant isolation without requiring a TenantProvider.
 * @public
 */
export function useTenantSlugSafe(): string | null {
  const ctx = useContext(TenantContext);
  return ctx?.tenantSlug ?? null;
}

/**
 * Returns the current tenant's presence mode ("detailed" | "binary").
 *
 * Falls back to "detailed" when called outside a TenantProvider or before
 * the tenant metadata has resolved — matches the backend's safe default so
 * first-paint shows the richer view even if the tenant turns out to be
 * binary (the switch happens transparently on the next render).
 *
 * Safe to call from any component. Co-located with the TenantContext it
 * reads from, so the context can stay unexported — the global test mock
 * in `src/test/setup.ts` re-exports this stub directly.
 */
export function usePresenceMode(): PresenceMode {
  const ctx = useContext(TenantContext);
  return ctx?.tenant?.presenceMode ?? "detailed";
}
