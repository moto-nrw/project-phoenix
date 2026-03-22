"use client";

import { SessionProvider } from "next-auth/react";

/**
 * Operator-scoped providers.
 *
 * SessionProvider reads the operator cookie ("operator.session-token")
 * via basePath="/api/operator/auth". This cookie is host-only (no domain),
 * so it's invisible to tenant subdomains.
 *
 * No ProfileProvider or SupervisionProvider — those are tenant-only.
 */
export function OperatorProviders({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <SessionProvider
      basePath="/api/operator/auth"
      refetchInterval={4 * 60}
      refetchOnWindowFocus={false}
    >
      {children}
    </SessionProvider>
  );
}
