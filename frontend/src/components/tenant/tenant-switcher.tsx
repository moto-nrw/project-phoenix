"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import { signIn, useSession } from "next-auth/react";
import { mutate } from "~/lib/swr";
import {
  listAvailableTenants,
  performTenantSwitch,
  type TenantSummary,
} from "~/lib/tenant-api";
import { useTenantSlugSafe } from "~/lib/tenant-context";
import { createLogger } from "~/lib/logger";
import { trackEvent } from "~/lib/analytics";
import { env } from "~/env";
import { Alert } from "~/components/ui/alert";

const logger = createLogger({ component: "TenantSwitcher" });

/**
 * Dropdown component that lets users switch between tenants they have access to.
 * Only renders when the user has more than one available tenant.
 *
 * Switch flow (per spec 04-frontend.md):
 * 1. Call switchTenant(slug) to get new JWT tokens
 * 2. Update NextAuth session via signIn("credentials", { internalRefresh: true })
 * 3. Clear SWR cache to prevent stale cross-tenant data
 * 4. Clear session cache for fresh token resolution
 * 5. Hard-navigate to new tenant URL
 */
export function TenantSwitcher() {
  const [tenants, setTenants] = useState<TenantSummary[]>([]);
  const [isOpen, setIsOpen] = useState(false);
  const [isSwitching, setIsSwitching] = useState(false);
  const [switchError, setSwitchError] = useState("");
  const dropdownRef = useRef<HTMLDivElement>(null);
  const currentSlug = useTenantSlugSafe();
  const { status } = useSession();

  // Fetch available tenants once authenticated
  useEffect(() => {
    if (status !== "authenticated") return;
    listAvailableTenants()
      .then(setTenants)
      .catch((err: unknown) => {
        logger.error("list_tenants_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
  }, [status]);

  // Close dropdown on outside click
  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(e.target as Node)
      ) {
        setIsOpen(false);
      }
    }
    if (isOpen) {
      document.addEventListener("mousedown", handleClickOutside);
    }
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [isOpen]);

  const handleSwitch = useCallback(
    async (targetTenant: TenantSummary) => {
      if (isSwitching) return;
      setIsSwitching(true);
      setIsOpen(false);
      setSwitchError("");

      try {
        // The backend resolves the switch target by SUBDOMAIN (same as
        // login), so pass targetTenant.subdomain — the slug column can
        // legitimately differ from it (#1975).
        await performTenantSwitch(targetTenant.subdomain, signIn, mutate);

        const switchPayload = {
          from_slug: currentSlug ?? "unknown",
          to_slug: targetTenant.subdomain,
        };
        logger.info("tenant_switched", switchPayload);
        trackEvent("tenant_switched", switchPayload);

        // 5. Hard-navigate to the new tenant subdomain.
        // Always use subdomain routing — the proxy rewrites subdomains
        // to path segments, so navigating to a path directly on the old
        // subdomain creates a broken double-prefixed URL.
        const tenantDomain = env.NEXT_PUBLIC_TENANT_DOMAIN;
        const port = window.location.port ? `:${window.location.port}` : "";
        const protocol = window.location.protocol;
        window.location.href = `${protocol}//${targetTenant.subdomain}.${tenantDomain}${port}/dashboard`;
      } catch (err) {
        logger.error("tenant_switch_failed", {
          error: err instanceof Error ? err.message : String(err),
          target_slug: targetTenant.subdomain,
        });
        setSwitchError(
          `Wechsel zu ${targetTenant.name} fehlgeschlagen. Bitte erneut versuchen oder direkt über die Schul-URL anmelden.`,
        );
        setIsSwitching(false);
      }
    },
    [isSwitching, currentSlug],
  );

  // Only render when user has multiple tenants
  if (tenants.length <= 1) {
    return null;
  }

  // currentSlug comes from the URL, which is the tenant's subdomain — so
  // match against subdomain, not the independent slug column (#1975).
  const currentTenant = tenants.find((t) => t.subdomain === currentSlug);
  const otherTenants = tenants.filter((t) => t.subdomain !== currentSlug);

  // Group other tenants by organization
  const grouped = new Map<string, TenantSummary[]>();
  for (const t of otherTenants) {
    const orgName = t.organizationName || "Andere";
    const existing = grouped.get(orgName) ?? [];
    existing.push(t);
    grouped.set(orgName, existing);
  }

  return (
    <div ref={dropdownRef} className="relative">
      {/* Trigger button */}
      <button
        type="button"
        onClick={() => {
          setSwitchError("");
          setIsOpen(!isOpen);
        }}
        disabled={isSwitching}
        className="flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-100 disabled:opacity-50"
      >
        <span className="max-w-[140px] truncate">
          {currentTenant?.name ?? currentSlug}
        </span>
        <svg
          className={`h-4 w-4 shrink-0 text-gray-400 transition-transform ${isOpen ? "rotate-180" : ""}`}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M19 9l-7 7-7-7"
          />
        </svg>
      </button>

      {/* Dropdown menu */}
      {isOpen && (
        <div className="absolute right-0 z-50 mt-1 w-64 overflow-hidden rounded-lg border border-gray-200 bg-white py-1 shadow-lg">
          {[...grouped.entries()].map(([orgName, orgTenants]) => (
            <div key={orgName}>
              {grouped.size > 1 && (
                <div className="px-3 py-1.5 text-xs font-semibold tracking-wider text-gray-400 uppercase">
                  {orgName}
                </div>
              )}
              {orgTenants.map((t) => (
                <button
                  key={t.tenantId}
                  type="button"
                  onClick={() => void handleSwitch(t)}
                  disabled={isSwitching}
                  className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-gray-700 hover:bg-gray-50 disabled:opacity-50"
                >
                  <span className="truncate">{t.name}</span>
                </button>
              ))}
            </div>
          ))}
        </div>
      )}

      {/* Visible feedback when a switch fails — a silent no-op here caused
          #1975 to go unnoticed. */}
      {switchError && !isOpen && (
        <div className="absolute right-0 z-50 mt-1 w-72">
          <Alert type="error" message={switchError} />
        </div>
      )}
    </div>
  );
}
