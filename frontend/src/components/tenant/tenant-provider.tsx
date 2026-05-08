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
 *
 * Cross-tab settings sync runs on three signals — each closes a gap the
 * others can't cover:
 *
 *  1. BroadcastChannel via `subscribeSettingsChanged` — covers other tabs
 *     on the SAME origin (e.g. an admin saves a setting in a tenant tab,
 *     and other tenant tabs of the same school pick it up).
 *
 *  2. `phoenix:tenant-settings-stale` window event — dispatched by the
 *     global SSE handler (use-global-sse.ts) when the backend fires
 *     `tenant_settings_changed`. This is the CROSS-ORIGIN path: an
 *     operator at operator.<domain> flipping student_photos_enabled never
 *     reaches tenant tabs via BroadcastChannel (different origin), but the
 *     SSE connection sees it because the event is broadcast to all clients
 *     regardless of subscription.
 *
 *  3. `visibilitychange` (visible) — last-resort fallback for the case
 *     where SSE is disconnected at the moment the toggle commits (auth
 *     timeout, network blip, kiosk waking from sleep). When the user
 *     returns to the tab we re-resolve once, so the staleness window is
 *     bounded by the time between the toggle and the next focus.
 *
 * Why centralised: hooks like useStudentPhotosEnabled used to subscribe per
 * mounted instance, so a single broadcast on a page rendering N student
 * cards would fan out into N identical /auth/tenant/resolve requests. The
 * value is tenant-global, so one subscription per tab is enough.
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
  // Mirror the server-rendered prop into local state so cross-tab broadcasts
  // can replace it without rerouting through layout.tsx. The useEffect below
  // re-syncs whenever the prop itself changes (router.refresh, navigation).
  const [tenant, setTenant] = useState<TenantInfo | null>(serverTenant);
  useEffect(() => {
    setTenant(serverTenant);
  }, [serverTenant]);

  // requestSeq guards against out-of-order responses when refetch fires
  // multiple times in close succession (visibilitychange + SSE +
  // BroadcastChannel can all land in the same tick). Without sequencing,
  // an older request that finishes last would clobber the newer
  // student_photos_enabled state and leave the tab on the wrong feature
  // flag until the next signal. We bump the counter on every
  // refetch and only apply a response whose token still matches the latest
  // outstanding request — older responses are silently dropped.
  const requestSeqRef = useRef(0);

  useEffect(() => {
    if (!tenantSlug) return undefined;
    // Reset sequencing whenever tenantSlug changes — a stale response from
    // a previous slug must not apply to the new slug's state.
    requestSeqRef.current = 0;

    // Single shared refetch — all three triggers below funnel through it so
    // a burst (e.g. SSE event arrives the same tick the user refocuses the
    // tab) doesn't fire two parallel /auth/tenant/resolve calls clobbering
    // each other in last-writer-wins order.
    const refetch = () => {
      const token = ++requestSeqRef.current;
      // resolveTenant is unauthenticated, so it works for every user
      // (admin, Betreuer, ...) without a session round-trip.
      void resolveTenant(tenantSlug).then((fresh) => {
        // Drop stale responses: only apply if no newer refetch has been
        // started since this one. Equality (not `<=`) so a refetch fired
        // during this then-callback also wins the race.
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
