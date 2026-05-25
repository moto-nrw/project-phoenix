"use client";

import { useEffect } from "react";
import { useOGSGroupsEnabled } from "~/components/tenant/tenant-provider";
import { useTenantRouter } from "~/lib/tenant-router";
import { Loading } from "~/components/ui/loading";

export default function DatabaseGroupsLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const router = useTenantRouter();
  const ogsGroupsEnabled = useOGSGroupsEnabled();

  useEffect(() => {
    if (!ogsGroupsEnabled) {
      router.replace("/database");
    }
  }, [ogsGroupsEnabled, router]);

  if (!ogsGroupsEnabled) {
    return <Loading fullPage={false} />;
  }

  return children;
}
