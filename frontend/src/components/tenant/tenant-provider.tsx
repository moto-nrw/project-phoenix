"use client";

import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  resolveTenant,
  type PresenceMode,
  type TenantInfo,
} from "~/lib/tenant-api";
import { subscribeSettingsChanged } from "~/lib/settings-broadcast";

interface TenantContextValue {
  /** The tenant slug from the URL path segment */
  tenantSlug: string;
  /** Resolved tenant metadata (null if not yet validated) */
  tenant: TenantInfo | null;
}

// Unexported — knip flags exports it can't trace through the `~/` alias and
// the test mock in setup.ts can't replicate createContext output.
const TenantContext = createContext<TenantContextValue | null>(null);

/**
 * Tenant context for the [tenant] route segment.
 *
 * Cross-tab settings sync subscribes to three signals (centralised here so
 * pages with N avatars don't fan out into N identical resolve calls):
 *  1. BroadcastChannel — same-origin tabs.
 *  2. `phoenix:tenant-settings-stale` — SSE-backed cross-origin path
 *     (operator → tenant); see use-global-sse.ts.
 *  3. visibilitychange — fallback when SSE was disconnected during commit.
 */
export function TenantProvider({
  tenantSlug,
  tenant: serverTenant,
  children,
}: {
  tenantSlug: string;
  tenant: TenantInfo | null;
  children: React.ReactNode;
}) {
  // Mirror the server prop so broadcasts can replace it without re-routing
  // through layout.tsx; re-sync when the prop itself changes.
  const [tenant, setTenant] = useState<TenantInfo | null>(serverTenant);
  useEffect(() => {
    setTenant(serverTenant);
  }, [serverTenant]);

  // Sequence-token to drop out-of-order resolveTenant responses when
  // multiple signals land in the same tick.
  const requestSeqRef = useRef(0);

  useEffect(() => {
    if (!tenantSlug) return undefined;
    requestSeqRef.current = 0;

    const refetch = () => {
      const token = ++requestSeqRef.current;
      void resolveTenant(tenantSlug).then((fresh) => {
        if (token !== requestSeqRef.current) return;
        if (fresh) setTenant(fresh);
      });
    };

    const unsubscribeBroadcast = subscribeSettingsChanged(refetch);

    const onTenantSettingsStale = () => refetch();
    if (typeof window !== "undefined") {
      window.addEventListener(
        "phoenix:tenant-settings-stale",
        onTenantSettingsStale,
      );
    }

    const onVisibilityChange = () => {
      if (
        typeof document !== "undefined" &&
        document.visibilityState === "visible"
      ) {
        refetch();
      }
    };
    if (typeof document !== "undefined") {
      document.addEventListener("visibilitychange", onVisibilityChange);
    }

    return () => {
      unsubscribeBroadcast();
      if (typeof window !== "undefined") {
        window.removeEventListener(
          "phoenix:tenant-settings-stale",
          onTenantSettingsStale,
        );
      }
      if (typeof document !== "undefined") {
        document.removeEventListener("visibilitychange", onVisibilityChange);
      }
    };
  }, [tenantSlug]);

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
 * Returns the full tenant context value, or null if outside a TenantProvider.
 * Safe variant of `useTenant` for hooks that must keep React's hook-order
 * invariant intact (a try/catch around the throwing variant would break
 * subsequent hooks on first render).
 */
export function useTenantSafe(): TenantContextValue | null {
  return useContext(TenantContext);
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
