"use client";

import { useEffect, useRef, useState } from "react";
import { signIn, signOut, useSession } from "next-auth/react";
import { mutate } from "~/lib/swr";
import { clearSessionCache } from "~/lib/session-cache";
import { TenantSwitchError, performTenantSwitch } from "~/lib/tenant-api";
import { performEndStaffPreview } from "~/lib/staff-preview-api";
import { useTenant } from "~/lib/tenant-context";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "TenantGuard" });

interface TenantGuardProps {
  readonly children: React.ReactNode;
  readonly redirect?: (url: string) => void;
}

function browserRedirect(url: string): void {
  window.location.assign(url);
}

/**
 * Client component that detects when the session's tenant differs from the
 * URL tenant and auto-switches the session to match the URL.
 *
 * Two-tab scenario:
 * 1. Tab A (school-a): session=school-a → no mismatch → renders normally
 * 2. Tab B (school-b): session=school-a, URL=school-b → mismatch → auto-switch
 * 3. Return to Tab A: SessionProvider refetches, session=school-b, URL=school-a → auto-switch back
 *
 * Operator isolation:
 * If a platform-scoped (operator) session leaks to a tenant subdomain via
 * shared cookies, the guard signs out immediately and redirects to the tenant
 * login page. This prevents operator sessions from accessing tenant-scoped UI.
 *
 * RLS provides defense-in-depth during any brief mismatch window.
 */
export function TenantGuard({
  children,
  redirect = browserRedirect,
}: TenantGuardProps) {
  const { data: session, status, update } = useSession();
  const { tenant } = useTenant();
  const switchAttempted = useRef(false);
  const [signingOutOperator, setSigningOutOperator] = useState(false);
  const [signingOutExpiredSession, setSigningOutExpiredSession] =
    useState(false);

  const sessionTenantId = session?.user?.tenantId;
  const sessionScope = session?.user?.scope;
  const sessionToken = session?.user?.token;
  const sessionError = session?.error;
  const urlTenantId = tenant?.tenantId;
  const urlSlug = tenant?.slug;
  // The backend resolves switch targets by subdomain, not by the slug
  // column — the two can differ (#1975).
  const urlSubdomain = tenant?.subdomain;

  // Auth.js can still report "authenticated" after the JWT callback has
  // stripped token/roles because refresh failed. Do not render tenant UI in
  // that limbo state; it makes real admins look like caregivers and produces
  // confusing forbidden screens.
  useEffect(() => {
    if (status !== "authenticated") return;
    if (sessionToken && !sessionError) return;

    logger.warn("tenant_session_invalid", {
      token_present: !!sessionToken,
      error: sessionError,
    });

    setSigningOutExpiredSession(true);

    void (async () => {
      try {
        await mutate(() => true, undefined, { revalidate: false });
        clearSessionCache();
      } catch (err) {
        logger.warn("invalid_session_cache_clear_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      }

      try {
        await signOut({ redirect: false });
      } catch (err) {
        logger.warn("invalid_session_signout_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      } finally {
        redirect("/?error=SessionExpired");
      }
    })();
  }, [status, sessionToken, sessionError, redirect]);

  // Operator session on tenant subdomain — sign out immediately
  useEffect(() => {
    if (status !== "authenticated" || !tenant) return;
    if (!sessionToken || sessionError) return;
    if (sessionScope !== "platform") return;

    logger.warn("operator_session_on_tenant", {
      url_slug: urlSlug,
      scope: sessionScope,
    });

    setSigningOutOperator(true);

    void (async () => {
      try {
        await mutate(() => true, undefined, { revalidate: false });
        clearSessionCache();
      } catch (err) {
        logger.warn("operator_signout_cache_clear_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      }

      try {
        await signOut({ redirect: false });
      } catch (err) {
        logger.warn("operator_signout_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      } finally {
        redirect("/");
      }
    })();
  }, [
    status,
    tenant,
    sessionScope,
    urlSlug,
    sessionToken,
    sessionError,
    redirect,
  ]);

  useEffect(() => {
    // Only check when authenticated and tenant context is resolved
    if (status !== "authenticated" || !tenant) return;
    if (!sessionToken || sessionError) return;

    // Operator sessions are handled by the effect above
    if (sessionScope === "platform") return;

    // Skip when session has no tenantId
    if (sessionTenantId === undefined) return;

    // No mismatch — reset guard for future switches
    if (sessionTenantId === urlTenantId) {
      switchAttempted.current = false;
      return;
    }

    // Mismatch detected — auto-switch (but only once per mismatch)
    if (switchAttempted.current) return;
    switchAttempted.current = true;

    logger.info("tenant_mismatch_detected", {
      session_tenant_id: sessionTenantId,
      url_tenant_id: urlTenantId,
      url_slug: urlSlug,
    });

    void (async () => {
      try {
        // Ein Schulwechsel beendet die Mitarbeiter-Vorschau (#2893): erst
        // die Admin-Sitzung wiederherstellen, dann mit deren Token wechseln
        // — das Vorschau-Token selbst darf den Wechsel nicht ausführen.
        if (session?.user?.isPreview) {
          await performEndStaffPreview(
            session.user.previewTargetAccountId?.toString(),
            update,
            mutate,
          );
        }
        await performTenantSwitch(urlSubdomain!, signIn, mutate);

        // Refetch session to trigger re-render with new tenant
        await update();

        logger.info("tenant_auto_switched", {
          from_tenant_id: sessionTenantId,
          to_slug: urlSubdomain,
        });
      } catch (err) {
        logger.error("tenant_auto_switch_failed", {
          error: err instanceof Error ? err.message : String(err),
          target_slug: urlSubdomain,
        });

        if (err instanceof TenantSwitchError && err.code === "access_denied") {
          await signOut({ callbackUrl: "/" });
        }
      }
    })();
  }, [
    status,
    tenant,
    session,
    sessionScope,
    sessionTenantId,
    urlTenantId,
    urlSlug,
    urlSubdomain,
    update,
    sessionToken,
    sessionError,
  ]);

  // While session is loading, render children transparently.
  // Individual pages handle their own loading states (e.g. login form
  // uses opacity fade). Blocking here causes visible flicker on every
  // page load because the guard's placeholder has a different layout
  // than the page content that replaces it.
  if (status === "loading") {
    return <>{children}</>;
  }

  // Block render for operator sessions — covers both before effect fires
  // and during sign-out (signingOutOperator state)
  const isOperatorOnTenant =
    signingOutOperator ||
    (status === "authenticated" && sessionScope === "platform" && !!tenant);

  if (
    signingOutExpiredSession ||
    (status === "authenticated" && (!sessionToken || sessionError))
  ) {
    return (
      <div className="flex min-h-[200px] items-center justify-center">
        <div className="text-sm text-gray-500">Sitzung wird erneuert...</div>
      </div>
    );
  }

  if (isOperatorOnTenant) {
    return (
      <div className="flex min-h-[200px] items-center justify-center">
        <div className="text-sm text-gray-500">
          Operator-Sitzung wird beendet...
        </div>
      </div>
    );
  }

  // Show switching state during mismatch auto-switch
  if (
    status === "authenticated" &&
    tenant &&
    sessionTenantId !== undefined &&
    sessionTenantId !== urlTenantId
  ) {
    return (
      <div className="flex min-h-[200px] items-center justify-center">
        <div className="text-sm text-gray-500">Mandant wird gewechselt...</div>
      </div>
    );
  }

  return <>{children}</>;
}
