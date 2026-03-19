/**
 * Auth Wrapper Component
 *
 * This component wraps authenticated app content to:
 * 1. Pre-warm the user context cache on mount (instant navigation)
 * 2. Establish a single global SSE connection (shared across all pages)
 *
 * By placing these hooks here (inside SessionProvider), we ensure:
 * - Single SSE connection for the entire app
 * - User context is cached before any page loads
 * - React Strict Mode safe (SWR handles deduplication)
 *
 * Teacher-specific hooks (user context, SSE, PostHog) are skipped for
 * operator sessions to avoid fetching teacher-only data with operator tokens.
 *
 * @example
 * ```tsx
 * // Used in providers.tsx
 * <SessionProvider>
 *   <AuthWrapper>
 *     {children}
 *   </AuthWrapper>
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

const logger = createLogger({ component: "AuthWrapper" });

interface AuthWrapperProps {
  children: React.ReactNode;
}

/**
 * Runs teacher-specific hooks: user context pre-warming, global SSE, PostHog.
 * Extracted into a separate component so hooks are not called conditionally.
 */
function TeacherSpecificHooks() {
  const { data: session, status } = useSession();

  // Pre-warm user context cache (only when authenticated)
  const { isReady: contextReady } = useUserContext();

  // Establish single global SSE connection
  const { status: sseStatus } = useGlobalSSE();

  // Identify user in PostHog after login, reset on logout
  useEffect(() => {
    if (status === "authenticated" && session?.user?.id) {
      posthog.identify(session.user.id, {
        email: session.user.email,
      });
    } else if (status === "unauthenticated") {
      posthog.reset();
    }
  }, [status, session]);

  // Debug logging (only in development)
  useEffect(() => {
    if (process.env.NODE_ENV === "development" && status === "authenticated") {
      logger.debug("auth wrapper state", {
        sse_status: sseStatus,
        context_ready: contextReady,
      });
    }
  }, [sseStatus, contextReady, status]);

  return null;
}

export function AuthWrapper({ children }: Readonly<AuthWrapperProps>) {
  const { data: session } = useSession();
  const isOperator = session?.user?.scope === "platform";

  return (
    <>
      {!isOperator && <TeacherSpecificHooks />}
      {children}
    </>
  );
}
