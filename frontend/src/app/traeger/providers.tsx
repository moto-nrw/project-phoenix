"use client";

import type { Session } from "next-auth";
import { SessionProvider } from "next-auth/react";
import { useSessionPollInterval } from "~/lib/use-session-poll-interval";
import { TraegerAnalyticsSession } from "~/components/analytics/traeger-analytics-session";
import { PersonalStorageGuard } from "~/components/auth/personal-storage-guard";
import { SessionAccountBinding } from "~/components/auth/session-account-binding";
import { SessionEndBarrier } from "~/components/auth/session-end-barrier";
import { SWRScopeBoundary } from "~/lib/swr/scope-boundary";
import { traegerPath } from "~/lib/traeger-url";
import { SentrySessionContext } from "~/components/auth/sentry-session-context";

// Where a page lands that still showed another account (#5000).
const traegerHome = () => traegerPath("/traeger");

/**
 * Träger-scoped providers ("moto träger", #4649).
 *
 * SessionProvider reads the Träger cookie ("traeger.session-token") via
 * basePath="/api/traeger/auth". The cookie is host-only, so tenant,
 * operator, parents, and school hosts never see it.
 *
 * `session` is the layout's read-only cookie snapshot (#4991). Without one
 * the provider receives undefined and fetches the session as before; null
 * would mean "signed out" and skip that fetch.
 */
export function TraegerProviders({
  children,
  session,
}: Readonly<{ children: React.ReactNode; session?: Session | null }>) {
  const pollSeconds = useSessionPollInterval(4 * 60);
  return (
    <SessionProvider
      basePath="/api/traeger/auth"
      session={session ?? undefined}
      refetchInterval={pollSeconds}
      refetchOnWindowFocus={false}
    >
      <SessionAccountBinding />
      <SessionEndBarrier />
      <PersonalStorageGuard homeHref={traegerHome} />
      <SWRScopeBoundary />
      <TraegerAnalyticsSession />
      <SentrySessionContext portal="traeger" />
      {children}
    </SessionProvider>
  );
}
