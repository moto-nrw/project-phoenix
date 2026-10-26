"use client";

import type { Session } from "next-auth";
import { SessionProvider } from "next-auth/react";
import { PersonalStorageGuard } from "~/components/auth/personal-storage-guard";
import { SentrySessionContext } from "~/components/auth/sentry-session-context";

/**
 * Operator-scoped providers.
 *
 * SessionProvider reads the operator cookie ("operator.session-token")
 * via basePath="/api/operator/auth". This cookie is host-only (no domain),
 * so it's invisible to tenant subdomains.
 *
 * No ProfileProvider or SupervisionProvider — those are tenant-only.
 *
 * `session` is the layout's read-only cookie snapshot (#4991). Without one
 * the provider receives undefined and fetches the session as before; null
 * would mean "signed out" and skip that fetch.
 */
export function OperatorProviders({
  children,
  session,
}: Readonly<{ children: React.ReactNode; session?: Session | null }>) {
  return (
    <SessionProvider
      basePath="/api/operator/auth"
      session={session ?? undefined}
      refetchInterval={4 * 60}
      refetchOnWindowFocus={false}
    >
      <PersonalStorageGuard />
      <SentrySessionContext portal="operator" />
      {children}
    </SessionProvider>
  );
}
