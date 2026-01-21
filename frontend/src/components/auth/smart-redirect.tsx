"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useSession } from "~/lib/auth-client";
import { useSupervision } from "~/lib/supervision-context";
import {
  useSmartRedirectPath,
  checkSaasAdminStatus,
} from "~/lib/redirect-utils";

interface SmartRedirectProps {
  readonly onRedirect?: (path: string) => void;
}

/**
 * Component that automatically redirects authenticated users to the most appropriate page
 * based on their permissions, supervision state, and SaaS admin status
 */
export function SmartRedirect({ onRedirect }: SmartRedirectProps) {
  const router = useRouter();
  // BetterAuth: cookies handle auth, isPending replaces status
  const { data: session, isPending } = useSession();
  const { hasGroups, isLoadingGroups, isSupervising, isLoadingSupervision } =
    useSupervision();

  // Check SaaS admin status
  const [isSaasAdmin, setIsSaasAdmin] = useState(false);
  const [isSaasAdminLoading, setIsSaasAdminLoading] = useState(true);

  useEffect(() => {
    // Only check SaaS admin status if user is authenticated
    if (session?.user) {
      checkSaasAdminStatus()
        .then((isAdmin) => {
          setIsSaasAdmin(isAdmin);
          setIsSaasAdminLoading(false);
        })
        .catch(() => {
          setIsSaasAdmin(false);
          setIsSaasAdminLoading(false);
        });
    } else if (!isPending) {
      setIsSaasAdminLoading(false);
    }
  }, [session?.user, isPending]);

  const { redirectPath, isReady } = useSmartRedirectPath(
    session,
    {
      hasGroups,
      isLoadingGroups,
      isSupervising,
      isLoadingSupervision,
    },
    {
      isSaasAdmin,
      isLoading: isSaasAdminLoading,
    },
  );

  useEffect(() => {
    // Only redirect if user is authenticated and all data is ready
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
