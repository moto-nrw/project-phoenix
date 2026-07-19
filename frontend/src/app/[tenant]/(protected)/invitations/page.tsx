"use client";

import { redirect } from "next/navigation";
import { useTenantAwarePath } from "~/lib/tenant-path";

export default function InvitationsRedirectPage() {
  const tenantPath = useTenantAwarePath();
  return redirect(tenantPath("/database/personal"));
}
