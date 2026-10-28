"use client";

import type { Session } from "next-auth";
import { SessionProvider } from "next-auth/react";
import { ParentPortalAnalyticsSession } from "~/components/analytics/parent-portal-analytics-session";
import { PersonalStorageGuard } from "~/components/auth/personal-storage-guard";
import { SWRScopeBoundary } from "~/lib/swr/scope-boundary";
import { parentPath } from "~/lib/parent-url";
import { SentrySessionContext } from "~/components/auth/sentry-session-context";
import { PushSubscriptionSync } from "~/components/notifications/service-worker-registrar";
import { ModulePortalProvider } from "~/lib/module-portal-context";
import { ParentLocaleProvider } from "~/lib/parent-locale-context";

// Where a page lands that still showed another account (#5000).
const parentsHome = () => parentPath("/parents");

/**
 * Parent-scoped providers.
 *
 * SessionProvider reads the parent cookie ("parent.session-token")
 * via basePath="/api/parent/auth". This cookie is host-only (no domain),
 * so it's invisible to tenant + operator subdomains.
 *
 * No ProfileProvider or SupervisionProvider — those are tenant-only.
 *
 * ParentLocaleProvider is mounted once here, inside SessionProvider. It derives
 * its authenticated state from the session itself, so the same instance handles
 * anonymous pages (login: cookie-only locale) and authenticated ones (syncs +
 * persists portal_locale) without the auth guard mounting a second, shadowing
 * provider.
 *
 * `session` is the layout's read-only cookie snapshot (#4990). Without one the
 * provider receives undefined and fetches the session as before; null would
 * mean "signed out" and skip that fetch.
 */
export function ParentProviders({
  children,
  session,
}: Readonly<{ children: React.ReactNode; session?: Session | null }>) {
  return (
    <SessionProvider
      basePath="/api/parent/auth"
      session={session ?? undefined}
      refetchInterval={4 * 60}
      refetchOnWindowFocus={false}
    >
      <PersonalStorageGuard homeHref={parentsHome} />
      <SWRScopeBoundary />
      <PushSubscriptionSync portal="parent" />
      <ParentPortalAnalyticsSession />
      <SentrySessionContext portal="parent" />
      <ParentLocaleProvider>
        <ModulePortalProvider portal="parent">{children}</ModulePortalProvider>
      </ParentLocaleProvider>
    </SessionProvider>
  );
}
