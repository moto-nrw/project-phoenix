"use client";

import {
  useState,
  useEffect,
  useLayoutEffect,
  useRef,
  useCallback,
} from "react";
import {
  OverflowMenu,
  type OverflowMenuEntry,
} from "~/components/ui/page-header/OverflowMenu";
import { useSession } from "next-auth/react";
import { mutate } from "~/lib/swr";
import {
  listAvailableTenants,
  performTenantSwitch,
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
import { credentialError } from "~/components/auth/credential-error";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import {
  BrandLink,
  BrandLogo,
  BrandTraegerLine,
  brandLabelClass,
} from "~/components/dashboard/header/brand-link";

const logger = createLogger({ component: "BrandTenantSwitcher" });

const SWITCH_REFUSALS = [
  "identity.tenant_access_denied",
  "identity.mfa_session_required",
] as const;

interface BrandTenantSwitcherProps {
  readonly href?: string;
  readonly label?: string | null;
  /** Unterhalb dieser Breite nur das Logo zeigen, siehe `BrandLink`. */
  readonly hideLabelBelow?: "md" | "lg";
  /** Träger der Schule als zweite Zeile, siehe `BrandTraegerLine`. */
  readonly traegerName?: string | null;
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
 * 2. The BFF replaces the NextAuth session cookie
 * 3. Clear SWR cache to prevent stale cross-tenant data
 * 4. Clear session cache for fresh token resolution
 * 5. Hard-navigate to new tenant URL
 */
export function BrandTenantSwitcher({
  href = "/home",
  label,
  hideLabelBelow,
  traegerName,
}: BrandTenantSwitcherProps) {
  // The tenant layout preloads the list on the server (#2973); without a
  // seed the list is fetched once the session is authenticated.
  const seededTenants = useShellSeed()?.accountTenants;
  const [tenants, setTenants] = useState<TenantSummary[]>(
    () => seededTenants ?? [],
  );
  const [isSwitching, setIsSwitching] = useState(false);
  // Lehrkraft-Konto: kein Fehler, sondern der Weg zum richtigen Portal.
  const [schoolPortalHint, setSchoolPortalHint] = useState<{
    url: string | null;
  } | null>(null);
  // Ein gescheiterter Wechsel ist eine Aktion ohne Formular: Toast mit
  // Katalogtext und Wiederholen (#2517).
  const { show: showSwitchError } = useApiErrorDisplay();
  const latestSwitchRef = useRef<(target: TenantSummary) => void>(
    () => undefined,
  );
  const currentSlug = useTenantSlugSafe();
  const { status, data: session, update } = useSession();

  // Fetch available tenants once authenticated
  useEffect(() => {
    if (status !== "authenticated" || seededTenants) return;
    listAvailableTenants()
      .then(setTenants)
      .catch((err: unknown) => {
        // Bewusst still: ohne Liste bleibt der Markenbereich ein normaler
        // Link zur Startseite, die Kopfzeile funktioniert weiter. Ein
        // Fehler hier ist kein Schritt, den die Person angestoßen hat.
        logger.error("list_tenants_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
  }, [status, seededTenants]);

  const handleSwitch = useCallback(
    async (targetTenant: TenantSummary) => {
      if (isSwitching) return;
      setIsSwitching(true);
      setSchoolPortalHint(null);

      try {
        // Ein Schulwechsel beendet die Mitarbeiter-Vorschau (#2893): erst
        // die Admin-Sitzung wiederherstellen, dann mit deren Token wechseln
        // — das Vorschau-Token selbst darf den Wechsel nicht ausführen.
        if (session?.user?.isPreview) {
          await performEndStaffPreview(update, mutate);
        }
        // The backend resolves the switch target by SUBDOMAIN (same as
        // login), so pass targetTenant.subdomain — the slug column can
        // legitimately differ from it (#1975).
        await performTenantSwitch(targetTenant.subdomain, mutate);

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
        window.location.href = `${protocol}//${targetTenant.subdomain}.${tenantDomain}${port}/home`;
      } catch (err) {
        logger.error("tenant_switch_failed", {
          error: err instanceof Error ? err.message : String(err),
          target_slug: targetTenant.subdomain,
        });
        if (
          err instanceof ApiError &&
          err.code === "identity.use_school_portal"
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
          setSchoolPortalHint({ url: portalUrl });
        } else {
          const openTargetLogin = () => {
            void import("./tenant-relogin")
              .then(({ reloginAtTenant }) =>
                reloginAtTenant(targetTenant.subdomain),
              )
              .catch((error: unknown) => {
                void showSwitchError(error, {
                  object: "die Abmeldung",
                  retry: openTargetLogin,
                });
              });
          };
          // Kein Zugang zur Schule kommt als 401: ohne den Code-Abgleich
          // spränge der Fehlerweg wortlos zur Anmeldung (#2517).
          void showSwitchError(credentialError(err, SWITCH_REFUSALS), {
            object:
              err instanceof ApiError &&
              err.code === "identity.mfa_session_required"
                ? targetTenant.name
                : `das Wechseln zu ${targetTenant.name}`,
            ...(err instanceof ApiError &&
            err.code === "identity.mfa_session_required"
              ? {
                  action: {
                    label: "Zur Schulanmeldung",
                    onClick: openTargetLogin,
                  },
                }
              : { retry: () => latestSwitchRef.current(targetTenant) }),
          });
        }
        setIsSwitching(false);
      }
    },
    [isSwitching, currentSlug, session, update, showSwitchError],
  );

  useLayoutEffect(() => {
    latestSwitchRef.current = (target) => void handleSwitch(target);
  });

  // Single (or unknown) tenant: plain brand link, no dropdown
  if (tenants.length <= 1) {
    return (
      <BrandLink
        href={href}
        label={label}
        hideLabelBelow={hideLabelBelow}
        traegerName={traegerName}
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

  const menuItems: OverflowMenuEntry[] = [];
  if (currentTenant) {
    menuItems.push({
      label: currentTenant.name,
      href,
      selected: true,
      onClick: () => undefined,
    });
    if (grouped.size === 1) menuItems.push({ kind: "separator" });
  }
  for (const [orgName, orgTenants] of grouped) {
    if (grouped.size > 1) menuItems.push({ kind: "header", label: orgName });
    for (const t of orgTenants) {
      menuItems.push({
        label: t.name,
        disabled: isSwitching,
        onClick: () => void handleSwitch(t),
      });
    }
  }

  return (
    <div className="relative min-w-0">
      {/* Brand as trigger */}
      <OverflowMenu
        ariaLabel="Einrichtung wechseln"
        items={menuItems}
        disabled={isSwitching}
        onOpen={() => setSchoolPortalHint(null)}
        wrapperClassName="min-w-0 block"
        triggerClassName="-mx-1.5 max-w-[200px] min-w-0 gap-2.5 rounded-lg px-1.5 py-1 hover:bg-gray-100 disabled:opacity-50 sm:max-w-[260px] lg:max-w-[300px]"
        triggerContent={
          <>
            <BrandLogo />
            <span
              className={`min-w-0 flex-col text-left ${
                hideLabelBelow === "md"
                  ? "hidden md:flex"
                  : hideLabelBelow === "lg"
                    ? "hidden lg:flex"
                    : "flex"
              }`}
            >
              <span className={brandLabelClass(true)}>{displayLabel}</span>
              <BrandTraegerLine traegerName={traegerName} />
            </span>
            <svg
              className="h-4 w-4 shrink-0 text-gray-400"
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
          </>
        }
      />

      {/* Lehrkraft-Konto: Hinweis mit dem Weg zu moto schule. Andere
          Fehler beim Wechsel kommen als Toast (#2517); ein stilles Nichts
          hat #1975 unbemerkt gelassen. */}
      {schoolPortalHint && (
        <div className="absolute left-0 z-50 mt-1 w-72">
          <Alert
            type="info"
            message="Dieses Konto gehört zu moto schule. Bitte melden Sie sich dort an."
            action={
              schoolPortalHint.url ? (
                <a
                  href={schoolPortalHint.url}
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
