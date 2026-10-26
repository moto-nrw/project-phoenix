"use client";

import type { Session } from "next-auth";
import { SessionProvider } from "next-auth/react";
import { PortalAnalyticsSession } from "~/components/analytics/portal-analytics-session";
import { PersonalStorageGuard } from "~/components/auth/personal-storage-guard";
import { catererPath } from "~/lib/caterer-url";
import { SentrySessionContext } from "~/components/auth/sentry-session-context";

// Where a page lands that still showed another account (#5000).
const catererHome = () => catererPath("/caterer");

/**
 * Caterer-scoped providers (ADR 0077). SessionProvider reads the host-only
 * caterer cookie ("caterer.session-token") through basePath
 * "/api/caterer/auth".
 *
 * `session` is the layout's read-only cookie snapshot (#4991). Without one
 * the provider receives undefined and fetches the session as before; null
 * would mean "signed out" and skip that fetch.
 */
export function CatererProviders({
  children,
  session,
}: Readonly<{ children: React.ReactNode; session?: Session | null }>) {
  return (
    <SessionProvider
      basePath="/api/caterer/auth"
      session={session ?? undefined}
      refetchInterval={4 * 60}
      refetchOnWindowFocus={false}
    >
      <PersonalStorageGuard homeHref={catererHome} />
      <PortalAnalyticsSession surface="caterer" />
      <SentrySessionContext portal="caterer" />
      {children}
    </SessionProvider>
  );
}
