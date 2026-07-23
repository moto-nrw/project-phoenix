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
  /** Whether visible tenant URLs carry the slug in the host or path. */
  routingMode: TenantRoutingMode;
}

export type TenantRoutingMode = "path" | "subdomain";

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
  routingMode = "path",
  children,
}: {
  tenantSlug: string;
  tenant: TenantInfo | null;
  routingMode?: TenantRoutingMode;
  children: React.ReactNode;
}) {
  // Mirror the server prop so broadcasts can replace it without re-routing
  // through layout.tsx; re-sync when the prop itself changes.
  const [tenant, setTenant] = useState<TenantInfo | null>(serverTenant);
  useEffect(() => {
    setTenant(serverTenant);
  }, [serverTenant]);

  // Sequence-token to drop out-of-order resolveTenant responses when
  // multiple signals land in the same tick. Kept monotonic across slug
  // changes so an in-flight resolve from a previous slug can't satisfy
  // the token check after the counter rolls back to zero. Bumped in
  // this effect's cleanup so navigation away from slug A invalidates
  // any A-resolve still in flight, even if no B-refetch fires before
  // A's response lands.
  const requestSeqRef = useRef(0);

  useEffect(() => {
    if (!tenantSlug) return undefined;

    const refetch = () => {
      const token = ++requestSeqRef.current;
      const requestedSlug = tenantSlug;
      void resolveTenant(requestedSlug).then((fresh) => {
        if (token !== requestSeqRef.current) return;
        if (!fresh) return;
        // Defence-in-depth: also drop responses whose payload doesn't
        // match the tenant we asked for. The counter-bump on slug change
        // already invalidates stale responses, but the explicit compare
        // guards against an upstream that returned data for a different
        // tenant than requested. Compare the SUBDOMAIN: the URL segment
        // is the subdomain, while the payload slug column can
        // legitimately differ from it (#1975).
        if (fresh.subdomain !== requestedSlug) return;
        setTenant(fresh);
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
      // Invalidate any in-flight resolve for the slug we're leaving so
      // its response can't overwrite the next slug's context. Without
      // this, navigating A → B while an A-resolve is pending leaves
      // token === counter and requestedSlug === fresh.slug both true,
      // and A's payload wins the last write on B's tenant context.
      // Direct ref mutation in cleanup is intentional here — the ref
      // is a counter, not a DOM node, so the lint rule's concern
      // (node identity changing between effect and cleanup) does not
      // apply.
      // eslint-disable-next-line react-hooks/exhaustive-deps
      requestSeqRef.current++;
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

  const value = useMemo(
    () => ({ tenantSlug, tenant, routingMode }),
    [tenantSlug, tenant, routingMode],
  );

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

export function useTenantRoutingModeSafe(): TenantRoutingMode {
  const ctx = useContext(TenantContext);
  return ctx?.routingMode ?? "path";
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

/**
 * Returns whether the current tenant uses NFC attendance devices.
 *
 * Defaults to false when tenant metadata is unavailable. That matches the
 * registry default and keeps NFC-only navigation hidden until the tenant
 * explicitly opts into NFC.
 */
export function useNFCEnabled(): boolean {
  const ctx = useContext(TenantContext);
  return ctx?.tenant?.nfcEnabled === true;
}

/**
 * Returns whether the Info-Point Dashboard feature is enabled for the
 * current tenant (display.enabled).
 *
 * Defaults to false when tenant metadata is unavailable — the feature is
 * opt-in, so the sidebar entry and admin page must stay hidden until a
 * school explicitly turns it on.
 */
export function useDisplayEnabled(): boolean {
  const ctx = useContext(TenantContext);
  return ctx?.tenant?.displayEnabled === true;
}

/**
 * Returns whether approved-child care offerings may be corrected.
 *
 * Missing tenant metadata stays enabled for compatibility with older backend
 * responses. A settings-resolution error is distinct: the backend publishes
 * an explicit false value, which keeps the mutation UI hidden.
 */
export function useCareOfferingsEnabled(): boolean {
  const ctx = useContext(TenantContext);
  return ctx?.tenant?.careOfferingsEnabled !== false;
}

export function useAttendanceWebEnabled(): boolean {
  const ctx = useContext(TenantContext);
  return ctx?.tenant?.attendanceWebEnabled === true;
}

export function useOpenCareGroupMode(): boolean {
  const ctx = useContext(TenantContext);
  return ctx?.tenant?.groupMode === "open_care";
}

export function useShowTimetableCounts(): boolean {
  const ctx = useContext(TenantContext);
  return ctx?.tenant?.showTimetableCounts !== false;
}

export function useWaitlistEnabled(): boolean {
  const ctx = useContext(TenantContext);
  return ctx?.tenant?.waitlistEnabled !== false;
}
