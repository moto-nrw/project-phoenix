"use client";

import type { Session } from "next-auth";
import { SessionProvider } from "next-auth/react";
import { PortalAnalyticsSession } from "~/components/analytics/portal-analytics-session";
import { SCHOOL_ANALYTICS_CONSENT_COPY } from "~/components/analytics/analytics-consent-copy";
import { PersonalStorageGuard } from "~/components/auth/personal-storage-guard";
import {
  SessionAccountBinding,
  useSessionPollInterval,
} from "~/components/auth/session-account-binding";
import { SessionEndBarrier } from "~/components/auth/session-end-barrier";
import { SWRScopeBoundary } from "~/lib/swr/scope-boundary";
import { schoolPath } from "~/lib/school-url";
import { SentrySessionContext } from "~/components/auth/sentry-session-context";
import { PushSubscriptionSync } from "~/components/notifications/service-worker-registrar";
import { ModulePortalProvider } from "~/lib/module-portal-context";

// Where a page lands that still showed another account (#5000).
const schoolHome = () => schoolPath("/school");

/**
 * School-scoped providers ("moto schule", #2207).
 *
 * SessionProvider reads the school cookie ("school.session-token") via
 * basePath="/api/school/auth". This cookie is host-only (no domain), so
 * it's invisible to tenant, operator, and parents hosts.
 *
 * No TenantProvider — the school portal has exactly one surface (the
 * Klassenansicht) and its tenant binding lives in the JWT, not in the URL.
 *
 * `session` is the layout's read-only cookie snapshot (#4991). Without one
 * the provider receives undefined and fetches the session as before; null
 * would mean "signed out" and skip that fetch.
 */
export function SchoolProviders({
  children,
  session,
}: Readonly<{ children: React.ReactNode; session?: Session | null }>) {
  const pollSeconds = useSessionPollInterval(4 * 60);
  return (
    <SessionProvider
      basePath="/api/school/auth"
      session={session ?? undefined}
      refetchInterval={pollSeconds}
      refetchOnWindowFocus={false}
    >
      <SessionAccountBinding />
      <SessionEndBarrier />
      <PersonalStorageGuard homeHref={schoolHome} />
      <SWRScopeBoundary />
      {/* Rebinds an existing browser push subscription to the current school
          session (#2208) — the tenant and parents portals do the same. */}
      <PushSubscriptionSync portal="school" />
      <PortalAnalyticsSession
        surface="school"
        consentCopy={SCHOOL_ANALYTICS_CONSENT_COPY}
      />
      <SentrySessionContext portal="school" />
      <ModulePortalProvider portal="school">{children}</ModulePortalProvider>
    </SessionProvider>
  );
}
