"use client";

import { useEffect } from "react";
import { redirect } from "next/navigation";
import { useSession } from "next-auth/react";
import { useSupervision } from "~/lib/supervision-context";
import { useSmartRedirectPath } from "~/lib/redirect-utils";
import { useOpenCareGroupMode, usePresenceMode } from "~/lib/tenant-context";
import { useTenantAwarePath } from "~/lib/tenant-path";

interface SmartRedirectProps {
  readonly onRedirect?: (path: string) => void;
}

/**
 * Component that automatically redirects authenticated users to the most appropriate page
 * based on their permissions and supervision state
 */
export function SmartRedirect({ onRedirect }: SmartRedirectProps) {
  const tenantPath = useTenantAwarePath();
  const { data: session, status } = useSession();
  const presenceMode = usePresenceMode();
  const openCareGroupMode = useOpenCareGroupMode();
  const { hasGroups, isLoadingGroups, isSupervising, isLoadingSupervision } =
    useSupervision();

  const { redirectPath, isReady } = useSmartRedirectPath(
    session,
    {
      hasGroups,
      isLoadingGroups,
      isSupervising,
      isLoadingSupervision,
    },
    presenceMode,
    openCareGroupMode,
  );

  useEffect(() => {
    // Only redirect if user is authenticated and supervision data is ready
    if (status === "authenticated" && session?.user?.token && isReady) {
      onRedirect?.(redirectPath);
    }
  }, [status, session?.user?.token, isReady, redirectPath, onRedirect]);

  if (
    !onRedirect &&
    status === "authenticated" &&
    session?.user?.token &&
    isReady
  ) {
    redirect(tenantPath(redirectPath));
  }

  // This component doesn't render anything
  return null;
}
