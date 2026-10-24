"use client";

import type { ReactNode } from "react";

import { ModuleNotBookedState } from "~/components/ui/module-not-booked-state";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import type { ModuleKey } from "~/lib/modules";
import { useBookedModules, useTenantSafe } from "~/lib/tenant-context";

/**
 * Stops historic module pages before their own data hooks run. Most module
 * pages are enforced by the API; activities and devices retain this client
 * boundary until their API routes enforce NFC-Anwesenheit themselves.
 */
export function ModuleRouteGuard({
  module,
  title,
  children,
}: {
  readonly module: ModuleKey;
  readonly title: string;
  readonly children: ReactNode;
}) {
  const tenant = useTenantSafe()?.tenant;
  const booked = useBookedModules();

  // While the tenant data is loading, preserve the page. This mirrors the
  // page scaffold and avoids showing an explanation briefly on every route.
  if (!tenant || booked.includes(module)) {
    return <>{children}</>;
  }

  return (
    <TenantPage title={title}>
      <SectionCard>
        <ModuleNotBookedState module={module} />
      </SectionCard>
    </TenantPage>
  );
}
