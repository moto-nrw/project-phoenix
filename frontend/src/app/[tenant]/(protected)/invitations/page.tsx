"use client";

import { useEffect } from "react";
import { useTenantRouter } from "~/lib/tenant-router";

export default function InvitationsRedirectPage() {
  const router = useTenantRouter();

  useEffect(() => {
    router.replace("/database/personal");
  }, [router]);

  return null;
}
