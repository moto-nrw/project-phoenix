"use client";

import { SessionProvider } from "next-auth/react";
import { PushSubscriptionSync } from "~/components/notifications/service-worker-registrar";

/**
 * School-scoped providers ("moto schule", #2207).
 *
 * SessionProvider reads the school cookie ("school.session-token") via
 * basePath="/api/school/auth". This cookie is host-only (no domain), so
 * it's invisible to tenant, operator, and parents hosts.
 *
 * No TenantProvider — the school portal has exactly one surface (the
 * Klassenansicht) and its tenant binding lives in the JWT, not in the URL.
 */
export function SchoolProviders({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <SessionProvider
      basePath="/api/school/auth"
      refetchInterval={4 * 60}
      refetchOnWindowFocus={false}
    >
      {/* Rebinds an existing browser push subscription to the current school
          session (#2208) — the tenant and parents portals do the same. */}
      <PushSubscriptionSync portal="school" />
      {children}
    </SessionProvider>
  );
}
