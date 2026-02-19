"use client";

import { createContext, useContext } from "react";
import type { TenantInfo } from "~/lib/tenant-api";

interface TenantContextValue {
  /** The tenant slug from the URL path segment */
  tenantSlug: string;
  /** Resolved tenant metadata (null if not yet validated) */
  tenant: TenantInfo | null;
}

/**
 * Exported for direct useContext() access in hooks that need to be safe outside
 * a TenantProvider (e.g., SWR hooks used in both tenant and operator routes).
 * Prefer useTenant() for components that are always within [tenant] routes.
 */
export const TenantContext = createContext<TenantContextValue | null>(null);

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
  return (
    <TenantContext.Provider value={{ tenantSlug, tenant }}>
      {children}
    </TenantContext.Provider>
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
