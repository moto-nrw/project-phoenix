"use client";

import { SessionProvider } from "next-auth/react";
import { PortalAnalyticsSession } from "~/components/analytics/portal-analytics-session";
import { SentrySessionContext } from "~/components/auth/sentry-session-context";

/**
 * Caterer-scoped providers (ADR 0075). SessionProvider reads the host-only
 * caterer cookie ("caterer.session-token") through basePath
 * "/api/caterer/auth".
 */
export function CatererProviders({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <SessionProvider
      basePath="/api/caterer/auth"
      refetchInterval={4 * 60}
      refetchOnWindowFocus={false}
    >
      <PortalAnalyticsSession surface="caterer" />
      <SentrySessionContext portal="caterer" />
      {children}
    </SessionProvider>
  );
}
