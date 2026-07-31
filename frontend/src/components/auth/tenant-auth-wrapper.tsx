/**
 * Tenant Auth Wrapper
 *
 * Runs teacher-specific hooks: user context pre-warming, global SSE, PostHog.
 * Only active for tenant sessions — never runs for operator sessions.
 * Must run inside both SessionProvider and TenantProvider.
 * TenantProviders owns this composition so routing context is always present.
 */

"use client";

import { useEffect } from "react";
import { useSession } from "next-auth/react";
import { usePathname } from "next/navigation";
import posthog from "posthog-js";
import { env } from "~/env";
import { useUserContext } from "~/lib/hooks/use-user-context";
import { useGlobalSSE } from "~/lib/hooks/use-global-sse";
import { createLogger } from "~/lib/logger";
import { trackPageView } from "~/lib/analytics";
import { resolveAnalyticsViewId } from "~/lib/analytics-routes";
import { useTenant } from "~/lib/tenant-context";
import { normalizeTenantPathname } from "~/lib/tenant-path";

const logger = createLogger({ component: "TenantAuthWrapper" });

function TeacherSpecificHooks() {
  const { data: session, status } = useSession();
  const rawPathname = usePathname();
  const { tenantSlug, tenant, routingMode } = useTenant();
  const pathname = normalizeTenantPathname(
    rawPathname,
    tenantSlug,
    routingMode,
  );
  const viewId = resolveAnalyticsViewId(pathname);
  const schoolId = session?.user?.tenantId?.toString() ?? null;
  const urlSchoolId = tenant?.tenantId?.toString() ?? null;
  const tenantMatchesSession =
    schoolId !== null && urlSchoolId !== null && schoolId === urlSchoolId;

  const { isReady: contextReady } = useUserContext();
  const { status: sseStatus } = useGlobalSSE();

  useEffect(() => {
    if (status === "authenticated" && tenantMatchesSession && schoolId) {
      // School-level context only. Never send an account/person identifier.
      posthog.register({
        school_id: schoolId,
        $groups: { school: schoolId },
        deployment: env.NEXT_PUBLIC_TENANT_DOMAIN,
      });
    } else if (
      status === "unauthenticated" ||
      (status === "authenticated" && !tenantMatchesSession)
    ) {
      posthog.unregister("school_id");
      posthog.unregister("$groups");
      posthog.unregister("deployment");
      if (status === "unauthenticated") posthog.reset();
    }
  }, [status, schoolId, tenantMatchesSession]);

  useEffect(() => {
    if (
      status === "authenticated" &&
      tenantMatchesSession &&
      schoolId &&
      viewId
    ) {
      trackPageView(viewId, schoolId);
    }
    // `pathname` is intentionally a dependency: navigating between two
    // resources with the same template (for example /students/1 ->
    // /students/2) is a new page view, while only `viewId` leaves the browser.
  }, [status, schoolId, tenantMatchesSession, viewId, pathname]);

  useEffect(() => {
    if (process.env.NODE_ENV === "development" && status === "authenticated") {
      logger.debug("tenant auth wrapper state", {
        sse_status: sseStatus,
        context_ready: contextReady,
      });
    }
  }, [sseStatus, contextReady, status]);

  return null;
}

export function TenantAuthWrapper({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <>
      <TeacherSpecificHooks />
      {children}
    </>
  );
}
