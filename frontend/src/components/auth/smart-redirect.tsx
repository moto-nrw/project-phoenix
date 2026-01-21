"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useSession } from "~/lib/auth-client";
import { useSupervision } from "~/lib/supervision-context";
import { useSmartRedirectPath } from "~/lib/redirect-utils";

interface SmartRedirectProps {
  readonly onRedirect?: (path: string) => void;
}

/**
 * Component that automatically redirects authenticated users to the most appropriate page
 * based on their permissions and supervision state
 */
export function SmartRedirect({ onRedirect }: SmartRedirectProps) {
  const router = useRouter();
  // BetterAuth: cookies handle auth, isPending replaces status
  const { data: session, isPending } = useSession();
  const { hasGroups, isLoadingGroups, isSupervising, isLoadingSupervision } =
    useSupervision();

  const { redirectPath, isReady } = useSmartRedirectPath(session, {
    hasGroups,
    isLoadingGroups,
    isSupervising,
    isLoadingSupervision,
  });

  useEffect(() => {
    // Only redirect if user is authenticated and supervision data is ready
    if (!isPending && session?.user && isReady) {
      if (onRedirect) {
        onRedirect(redirectPath);
      } else {
        router.push(redirectPath);
        router.refresh();
      }
    }
  }, [isPending, session?.user, isReady, redirectPath, router, onRedirect]);

  // This component doesn't render anything
  return null;
}
