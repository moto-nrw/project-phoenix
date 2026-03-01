// [tenant]/auth/tenant-callback/page.tsx — Token transfer landing page
//
// When switching tenants, the TenantSwitcher navigates here with JWT tokens
// in the URL hash fragment (#t=<access>&r=<refresh>). This page establishes
// a NextAuth session on the NEW subdomain, then redirects to the dashboard.
//
// Hash fragments are never sent to the server (secure by design).
"use client";

import { useEffect, useRef } from "react";
import { signIn } from "next-auth/react";
import { mutate } from "~/lib/swr";
import { clearSessionCache } from "~/lib/session-cache";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "TenantCallback" });

export default function TenantCallbackPage() {
  const processed = useRef(false);

  useEffect(() => {
    // Strict mode double-mount guard
    if (processed.current) return;
    processed.current = true;

    const hash = window.location.hash.slice(1); // Remove leading #
    const params = new URLSearchParams(hash);
    const token = params.get("t");
    const refreshToken = params.get("r");

    // Clear hash immediately — tokens should not linger in the URL
    window.history.replaceState(null, "", window.location.pathname);

    if (!token || !refreshToken) {
      logger.warn("tenant_callback_missing_tokens");
      window.location.replace("/");
      return;
    }

    void (async () => {
      try {
        // Establish session on this subdomain with the transferred tokens
        const result = await signIn("credentials", {
          redirect: false,
          internalRefresh: "true",
          token,
          refreshToken,
        });

        if (result?.error) {
          logger.error("tenant_callback_signin_failed", {
            error: result.error,
          });
          window.location.replace("/");
          return;
        }

        // Clear stale caches before entering the dashboard
        await mutate(() => true, undefined, { revalidate: false });
        clearSessionCache();

        logger.info("tenant_callback_session_established");

        // Replace so the callback URL doesn't stay in browser history
        window.location.replace("/dashboard");
      } catch (err) {
        logger.error("tenant_callback_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        window.location.replace("/");
      }
    })();
  }, []);

  return (
    <div className="flex min-h-screen items-center justify-center">
      <div className="text-sm text-gray-500">Mandant wird gewechselt...</div>
    </div>
  );
}
