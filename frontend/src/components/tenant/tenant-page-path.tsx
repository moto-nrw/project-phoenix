"use client";

import type { ReactNode } from "react";

import { PagePathProvider } from "~/components/ui/page-path";
import { useTenantAwarePath } from "~/lib/tenant-path";

/**
 * Resolves the kit's app paths (the scaffold's back link) for the school of
 * the tenant area (#6577). Other portals mount no resolver and pass ready
 * paths.
 */
export function TenantPagePathProvider({
  children,
}: {
  readonly children: ReactNode;
}) {
  return (
    <PagePathProvider resolve={useTenantAwarePath()}>
      {children}
    </PagePathProvider>
  );
}
