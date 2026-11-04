"use client";

import { useEffect } from "react";
import { redirect } from "next/navigation";
import { useSession } from "next-auth/react";
import { useSmartRedirectPath } from "~/lib/redirect-utils";
import { hasSessionAccess } from "~/lib/session-access";
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

  const { redirectPath, isReady } = useSmartRedirectPath(session);
  const hasAccess = hasSessionAccess(session);

  useEffect(() => {
    // Only redirect if user is authenticated and supervision data is ready
    if (status === "authenticated" && hasAccess && isReady) {
      onRedirect?.(redirectPath);
    }
  }, [status, hasAccess, isReady, redirectPath, onRedirect]);

  if (!onRedirect && status === "authenticated" && hasAccess && isReady) {
    redirect(tenantPath(redirectPath));
  }

  // This component doesn't render anything
  return null;
}
