/**
 * Tenant Auth Wrapper
 *
 * Runs teacher-specific hooks: user context pre-warming, global SSE, PostHog.
 * Only active for tenant sessions — never runs for operator sessions.
 * Must run inside both SessionProvider and TenantProvider.
 * TenantProviders owns this composition so routing context is always present.
 */

"use client";

import { useEffect, useRef } from "react";
import { useSession } from "next-auth/react";
import { useUserContext } from "~/lib/hooks/use-user-context";
import { useGlobalSSE } from "~/lib/hooks/use-global-sse";
import { createLogger } from "~/lib/logger";
import {
  clearPortalSession,
  ogsAnalyticsRole,
  registerPortalSession,
} from "~/lib/analytics";
import { useTenant } from "~/lib/tenant-context";

const logger = createLogger({ component: "TenantAuthWrapper" });

function TeacherSpecificHooks() {
  const { data: session, status } = useSession();
  const { tenant } = useTenant();
  const schoolId = session?.user?.tenantId?.toString() ?? null;
  const urlSchoolId = tenant?.tenantId?.toString() ?? null;
  const tenantMatchesSession =
    schoolId !== null && urlSchoolId !== null && schoolId === urlSchoolId;
  const role = session?.user ? ogsAnalyticsRole(session.user) : null;
  const registeredSchoolIdRef = useRef<string | null>(null);

  const { isReady: contextReady } = useUserContext();
  const { status: sseStatus } = useGlobalSSE();

  useEffect(() => {
    if (
      status === "authenticated" &&
      tenantMatchesSession &&
      schoolId &&
      role
    ) {
      // School and role only. Never send an account/person identifier. Page
      // views come from the SDK; the analytics filter rewrites their URL to
      // the route template (analytics-policy.ts).
      registerPortalSession(
        { surface: "ogs", schoolId, role },
        registeredSchoolIdRef.current !== null &&
          registeredSchoolIdRef.current !== schoolId,
      );
      registeredSchoolIdRef.current = schoolId;
    } else if (
      status === "unauthenticated" ||
      (status === "authenticated" && !tenantMatchesSession)
    ) {
      clearPortalSession();
      registeredSchoolIdRef.current = null;
    }
  }, [status, schoolId, tenantMatchesSession, role]);

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
