"use client";

import type { Session } from "next-auth";
import { SessionProvider } from "next-auth/react";
import { useSessionPollInterval } from "~/lib/use-session-poll-interval";
import { PersonalStorageGuard } from "~/components/auth/personal-storage-guard";
import { SessionAccountBinding } from "~/components/auth/session-account-binding";
import { SWRScopeBoundary } from "~/lib/swr/scope-boundary";
import { operatorPath } from "~/lib/operator-url";
import { SentrySessionContext } from "~/components/auth/sentry-session-context";

// Where a page lands that still showed another account (#5000).
const operatorHome = () => operatorPath("/operator/organizations");

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
  const pollSeconds = useSessionPollInterval(4 * 60);
  return (
    <SessionProvider
      basePath="/api/operator/auth"
      session={session ?? undefined}
      refetchInterval={pollSeconds}
      refetchOnWindowFocus={false}
    >
      <SessionAccountBinding />
      <PersonalStorageGuard homeHref={operatorHome} />
      <SWRScopeBoundary />
      <SentrySessionContext portal="operator" />
      {children}
    </SessionProvider>
  );
}
