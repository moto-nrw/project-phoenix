"use client";

import { useEffect } from "react";
import { useTenantRouter } from "~/lib/tenant-router";

// Legacy deep-link target. Activity management now happens from the canonical
// /activities list/modal flow, so bookmarks to /activities/{id} land there.
export default function ActivityDetailRedirect() {
  const router = useTenantRouter();

  useEffect(() => {
    router.replace("/activities");
  }, [router]);

  return null;
}
