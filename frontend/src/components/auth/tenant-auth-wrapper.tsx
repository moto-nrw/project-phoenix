/**
 * Tenant Auth Wrapper
 *
 * Runs teacher-specific hooks: user context pre-warming, global SSE, PostHog.
 * Only active for tenant sessions — never runs for operator sessions.
 *
 * @example
 * ```tsx
 * <SessionProvider>
 *   <TenantAuthWrapper>
 *     {children}
 *   </TenantAuthWrapper>
 * </SessionProvider>
 * ```
 */

"use client";

import { useEffect } from "react";
import { useSession } from "next-auth/react";
import posthog from "posthog-js";
import { useUserContext } from "~/lib/hooks/use-user-context";
import { useGlobalSSE } from "~/lib/hooks/use-global-sse";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "TenantAuthWrapper" });

function TeacherSpecificHooks() {
  const { data: session, status } = useSession();

  const { isReady: contextReady } = useUserContext();
  const { status: sseStatus } = useGlobalSSE();

  useEffect(() => {
    if (status === "authenticated" && session?.user?.id) {
      // GDPR: identify with the pseudonymous account ID only — never attach
      // person properties like email.
      posthog.identify(session.user.id);
      if (session.user.tenantId != null) {
        posthog.group("school", String(session.user.tenantId));
        posthog.register({ school_id: session.user.tenantId });
      }
    } else if (status === "unauthenticated") {
      posthog.reset();
    }
  }, [status, session]);

  useEffect(() => {
    if (process.env.NODE_ENV === "development" && status === "authenticated") {
      logger.debug("tenant auth wrapper state", {
        sse_status: sseStatus,
        context_ready: contextReady,
      });
    }
  }, [sseStatus, contextReady, status]);

  return null;
}

export function TenantAuthWrapper({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <>
      <TeacherSpecificHooks />
      {children}
    </>
  );
}
