/**
 * Tenant Auth Wrapper
 *
 * Runs teacher-specific hooks: user context pre-warming, global SSE, PostHog.
 * Only active for tenant sessions — never runs for operator sessions.
 * Must run inside both SessionProvider and TenantProvider.
 * TenantProviders owns this composition so routing context is always present.
 */

"use client";

import { useEffect, useRef, useState } from "react";
import { useSession } from "next-auth/react";
import { useUserContext } from "~/lib/hooks/use-user-context";
import { useGlobalSSE } from "~/lib/hooks/use-global-sse";
import { createLogger } from "~/lib/logger";
import {
  clearPortalSession,
  ogsAnalyticsRole,
  registerPortalSession,
} from "~/lib/analytics";
import { analyticsPseudonym } from "~/lib/analytics-pseudonym";
import { useTenant } from "~/lib/tenant-context";

const logger = createLogger({ component: "TenantAuthWrapper" });

/**
 * The pseudonymous ID of the signed-in account while its school has the
 * Analyse-Freigabe (#3603); null otherwise, and for the read-only staff
 * preview, where an admin looks at someone else's view.
 */
function useAnalyticsPerson(
  freigabe: boolean,
  schoolId: string | null,
  accountId: string | null,
): string | null {
  const [person, setPerson] = useState<string | null>(null);
  useEffect(() => {
    if (!freigabe || !schoolId || !accountId) {
      setPerson(null);
      return undefined;
    }
    let cancelled = false;
    void analyticsPseudonym(schoolId, accountId).then((pseudonym) => {
      if (!cancelled) setPerson(pseudonym);
    });
    return () => {
      cancelled = true;
    };
  }, [freigabe, schoolId, accountId]);
  return person;
}

function TeacherSpecificHooks() {
  const { data: session, status } = useSession();
  const { tenant } = useTenant();
  const schoolId = session?.user?.tenantId?.toString() ?? null;
  const urlSchoolId = tenant?.tenantId?.toString() ?? null;
  const tenantMatchesSession =
    schoolId !== null && urlSchoolId !== null && schoolId === urlSchoolId;
  const role = session?.user ? ogsAnalyticsRole(session.user) : null;
  const registeredSchoolIdRef = useRef<string | null>(null);
  // The school context carries the Freigabe; it is re-read on every page load
  // and when the moto team changes it, so a revocation applies at once.
  const freigabe = tenantMatchesSession && tenant?.analyticsFreigabe === true;
  const samplePercent = tenant?.analyticsRecordingSamplePercent ?? 0;
  const person = useAnalyticsPerson(
    freigabe,
    schoolId,
    session?.user?.isPreview === true ? null : (session?.user?.id ?? null),
  );

  const { isReady: contextReady } = useUserContext();
  const { status: sseStatus } = useGlobalSSE();

  useEffect(() => {
    if (
      status === "authenticated" &&
      tenantMatchesSession &&
      schoolId &&
      role
    ) {
      // School and role; an account only as its pseudonymous ID, and only
      // with the school's Analyse-Freigabe. Page views come from the SDK;
      // the analytics filter rewrites their URL to the route template
      // (analytics-policy.ts).
      registerPortalSession(
        {
          surface: "ogs",
          schoolId,
          role,
          ...(freigabe
            ? {
                analyseFreigabe: true,
                recordingSamplePercent: samplePercent,
                person,
              }
            : {}),
        },
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
  }, [
    status,
    schoolId,
    tenantMatchesSession,
    role,
    freigabe,
    samplePercent,
    person,
  ]);

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
