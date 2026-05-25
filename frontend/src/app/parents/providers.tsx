"use client";

import { SessionProvider } from "next-auth/react";

/**
 * Parent-scoped providers.
 *
 * SessionProvider reads the parent cookie ("parent.session-token")
 * via basePath="/api/parent/auth". This cookie is host-only (no domain),
 * so it's invisible to tenant + operator subdomains.
 *
 * No ProfileProvider or SupervisionProvider — those are tenant-only.
 */
export function ParentProviders({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <SessionProvider
      basePath="/api/parent/auth"
      refetchInterval={4 * 60}
      refetchOnWindowFocus={false}
    >
      {children}
    </SessionProvider>
  );
}
