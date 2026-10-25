"use client";

import { SessionProvider } from "next-auth/react";
import { TraegerAnalyticsSession } from "~/components/analytics/traeger-analytics-session";
import { SentrySessionContext } from "~/components/auth/sentry-session-context";

/**
 * Träger-scoped providers ("moto träger", #4649).
 *
 * SessionProvider reads the Träger cookie ("traeger.session-token") via
 * basePath="/api/traeger/auth". The cookie is host-only, so tenant,
 * operator, parents, and school hosts never see it.
 */
export function TraegerProviders({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <SessionProvider
      basePath="/api/traeger/auth"
      refetchInterval={4 * 60}
      refetchOnWindowFocus={false}
    >
      <TraegerAnalyticsSession />
      <SentrySessionContext portal="traeger" />
      {children}
    </SessionProvider>
  );
}
