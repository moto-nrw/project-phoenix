"use client";

import { redirect } from "next/navigation";
import { useOpenCareGroupMode } from "~/lib/tenant-context";
import { useTenantAwarePath } from "~/lib/tenant-path";

/**
 * OpenCareModeGuard — redirects to the Kindersuche when the tenant runs in
 * open-care group mode (operations.group_mode = "open_care").
 *
 * Open-care tenants have no concept of "meine Gruppe", so the group-based
 * staff entry page (/ogs-groups) is misleading there. Navigation already
 * hides the entry (#1544); this guard covers direct URL entry by sending the
 * user to the open-care staff entry point instead of a dead group page.
 *
 * Same component-not-hook rationale as BinaryModeGuard: the gate lives at
 * the route boundary and renders nothing before redirecting.
 */
export function OpenCareModeGuard({ children }: { children: React.ReactNode }) {
  const openCareGroupMode = useOpenCareGroupMode();
  const tenantPath = useTenantAwarePath();
  if (openCareGroupMode) {
    redirect(tenantPath("/students/search"));
  }
  return <>{children}</>;
}
