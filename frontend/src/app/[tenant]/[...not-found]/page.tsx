"use client";

import { useEffect } from "react";
import { useTenantRouter } from "~/lib/tenant-router";

export default function UnknownTenantRoute() {
  const router = useTenantRouter();

  useEffect(() => {
    router.replace("/");
  }, [router]);

  return null;
}
