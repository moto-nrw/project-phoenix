"use client";

import { useEffect } from "react";
import { useTenantRouter } from "~/lib/tenant-router";

export default function InvitationsPage() {
  const router = useTenantRouter();

  useEffect(() => {
    router.replace("/database/personal");
  }, [router]);

  return null;
}
