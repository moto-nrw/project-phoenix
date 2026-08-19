"use client";

import { SessionProvider } from "next-auth/react";
import { PushSubscriptionSync } from "~/components/notifications/service-worker-registrar";
import { PwaUsageReporter } from "~/components/pwa/pwa-usage-reporter";
import { ParentLocaleProvider } from "~/lib/parent-locale-context";

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
      <PushSubscriptionSync portal="parent" />
      <PwaUsageReporter portal="parent" />
      <ParentLocaleProvider>{children}</ParentLocaleProvider>
    </SessionProvider>
  );
}
