"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import Link from "next/link";
import { signIn, useSession } from "next-auth/react";
import { mutate } from "~/lib/swr";
import {
  listAvailableTenants,
  performTenantSwitch,
  TenantSwitchError,
  type TenantSummary,
} from "~/lib/tenant-api";
import { performEndStaffPreview } from "~/lib/staff-preview-api";
import { schoolPortalLoginUrl } from "~/lib/school-url";
import { useShellSeed } from "~/lib/shell-seed";
import { useTenantSlugSafe } from "~/lib/tenant-context";
import { createLogger } from "~/lib/logger";
import { trackTenantEvent } from "~/lib/analytics";
import { clientEnv } from "~/env.client";
import { Alert } from "~/components/ui/alert";
import {
  BrandLink,
  BrandLogo,
  brandLabelClass,
} from "~/components/dashboard/header/brand-link";

const logger = createLogger({ component: "BrandTenantSwitcher" });

interface BrandTenantSwitcherProps {
  readonly isScrolled?: boolean;
  readonly href?: string;
  readonly label?: string | null;
  /** Unterhalb dieser Breite nur das Logo zeigen, siehe `BrandLink`. */
  readonly hideLabelBelow?: "md" | "lg";
}

/**
 * Brand area (logo + facility name) that doubles as the tenant switcher.
 *
 * With one (or zero) available tenants it renders the plain BrandLink —
 * name shown, no dropdown. With multiple tenants the brand itself becomes
 * the dropdown trigger (#2011), replacing the separate switcher element
 * that used to overflow the header on mobile.
 *
 * Switch flow:
 * 1. Call switchTenant(slug) to get new JWT tokens
 * 2. Update NextAuth session via signIn("credentials", { internalRefresh: true })
 * 3. Clear SWR cache to prevent stale cross-tenant data
 * 4. Clear session cache for fresh token resolution
 * 5. Hard-navigate to new tenant URL
 */
export function BrandTenantSwitcher({
  isScrolled = false,
  href = "/dashboard",
  label,
  hideLabelBelow,
}: BrandTenantSwitcherProps) {
  // The tenant layout preloads the list on the server (#2973); without a
  // seed the list is fetched once the session is authenticated.
  const seededTenants = useShellSeed()?.accountTenants;
  const [tenants, setTenants] = useState<TenantSummary[]>(
    () => seededTenants ?? [],
  );
  const [isOpen, setIsOpen] = useState(false);
  const [isSwitching, setIsSwitching] = useState(false);
  const [switchError, setSwitchError] = useState("");
  const [schoolPortalUrl, setSchoolPortalUrl] = useState<string | null>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const currentSlug = useTenantSlugSafe();
  const { status, data: session, update } = useSession();

  // Fetch available tenants once authenticated
  useEffect(() => {
    if (status !== "authenticated" || seededTenants) return;
    listAvailableTenants()
      .then(setTenants)
      .catch((err: unknown) => {
        logger.error("list_tenants_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
  }, [status, seededTenants]);

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
      setSchoolPortalUrl(null);

      try {
        // Ein Schulwechsel beendet die Mitarbeiter-Vorschau (#2893): erst
        // die Admin-Sitzung wiederherstellen, dann mit deren Token wechseln
        // — das Vorschau-Token selbst darf den Wechsel nicht ausführen.
        if (session?.user?.isPreview) {
          await performEndStaffPreview(session.user.token, update, mutate);
        }
        // The backend resolves the switch target by SUBDOMAIN (same as
        // login), so pass targetTenant.subdomain — the slug column can
        // legitimately differ from it (#1975).
        await performTenantSwitch(targetTenant.subdomain, signIn, mutate);

        const switchPayload = {
          from_slug: currentSlug ?? "unknown",
          to_slug: targetTenant.subdomain,
        };
        logger.info("tenant_switched", switchPayload);
        // Slugs identify organizations and must stay out of product analytics.
        trackTenantEvent("tenant_switched", targetTenant.tenantId.toString());

        // 5. Hard-navigate to the new tenant subdomain.
        // Always use subdomain routing — the proxy rewrites subdomains
        // to path segments, so navigating to a path directly on the old
        // subdomain creates a broken double-prefixed URL.
        const tenantDomain = clientEnv.NEXT_PUBLIC_TENANT_DOMAIN;
        const port = window.location.port ? `:${window.location.port}` : "";
        const protocol = window.location.protocol;
        window.location.href = `${protocol}//${targetTenant.subdomain}.${tenantDomain}${port}/dashboard`;
      } catch (err) {
        logger.error("tenant_switch_failed", {
          error: err instanceof Error ? err.message : String(err),
          target_slug: targetTenant.subdomain,
        });
        if (
          err instanceof TenantSwitchError &&
          err.code === "use_school_portal"
        ) {
          let portalUrl: string | null = null;
          try {
            portalUrl = schoolPortalLoginUrl(
              `?from=staff&tenant=${encodeURIComponent(targetTenant.subdomain)}`,
            );
          } catch (urlErr) {
            logger.error("school_portal_url_unavailable", {
              error: urlErr instanceof Error ? urlErr.message : String(urlErr),
            });
          }
          setSchoolPortalUrl(portalUrl);
          setSwitchError(
            "Dieses Konto ist ein Lehrkraft-Konto. Bitte melden Sie sich bei moto schule an.",
          );
        } else {
          setSwitchError(
            `Wechsel zu ${targetTenant.name} fehlgeschlagen. Bitte erneut versuchen.`,
          );
        }
        setIsSwitching(false);
      }
    },
    [isSwitching, currentSlug, session, update],
  );

  // Single (or unknown) tenant: plain brand link, no dropdown
  if (tenants.length <= 1) {
    return (
      <BrandLink
        isScrolled={isScrolled}
        href={href}
        label={label}
        hideLabelBelow={hideLabelBelow}
      />
    );
  }

  // currentSlug comes from the URL, which is the tenant's subdomain — so
  // match against subdomain, not the independent slug column (#1975).
  const currentTenant = tenants.find((t) => t.subdomain === currentSlug);
  const otherTenants = tenants.filter((t) => t.subdomain !== currentSlug);
  const displayLabel = currentTenant?.name ?? label?.trim() ?? currentSlug;

  // Group other tenants by organization
  const grouped = new Map<string, TenantSummary[]>();
  for (const t of otherTenants) {
    const orgName = t.organizationName || "Andere";
    const existing = grouped.get(orgName) ?? [];
    existing.push(t);
    grouped.set(orgName, existing);
  }

  return (
    <div ref={dropdownRef} className="relative min-w-0">
      {/* Brand as trigger */}
      <button
        type="button"
        onClick={() => {
          setSwitchError("");
          setSchoolPortalUrl(null);
          setIsOpen(!isOpen);
        }}
        disabled={isSwitching}
        aria-haspopup="menu"
        aria-expanded={isOpen}
        aria-label="Einrichtung wechseln"
        className="-mx-1.5 flex max-w-[200px] min-w-0 items-center gap-2 rounded-lg px-1.5 py-1 transition-colors hover:bg-gray-100 disabled:opacity-50 sm:max-w-[260px] lg:max-w-[300px]"
      >
        <BrandLogo />
        <span
          className={`${brandLabelClass(isScrolled, true)} ${
            hideLabelBelow === "md"
              ? "hidden md:inline-block"
              : hideLabelBelow === "lg"
                ? "hidden lg:inline-block"
                : ""
          }`}
        >
          {displayLabel}
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
        <div className="absolute left-0 z-50 mt-1 w-64 overflow-hidden rounded-lg border border-gray-200 bg-white py-1 shadow-lg">
          {currentTenant && (
            <Link
              href={href}
              onClick={() => setIsOpen(false)}
              className="flex items-center gap-2 border-b border-gray-100 px-3 py-2 text-sm font-medium text-gray-900 hover:bg-gray-50"
            >
              <span className="truncate">{currentTenant.name}</span>
              <svg
                className="text-moto-green ml-auto h-4 w-4 shrink-0"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M5 13l4 4L19 7"
                />
              </svg>
            </Link>
          )}
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
        <div className="absolute left-0 z-50 mt-1 w-72">
          <Alert
            type="error"
            message={switchError}
            action={
              schoolPortalUrl ? (
                <a
                  href={schoolPortalUrl}
                  className="font-medium whitespace-nowrap underline underline-offset-2"
                >
                  Jetzt zu moto schule
                </a>
              ) : undefined
            }
          />
        </div>
      )}
    </div>
  );
}
