/**
 * Auth Wrapper Component
 *
 * This component wraps authenticated app content to:
 * 1. Pre-warm the user context cache on mount (instant navigation)
 * 2. Establish a single global SSE connection (shared across all pages)
 *
 * BetterAuth uses cookies for session management, so no SessionProvider is needed.
 * Session state is managed via the auth-client module.
 *
 * @example
 * ```tsx
 * // Used in providers.tsx
 * <AuthWrapper>
 *   {children}
 * </AuthWrapper>
 * ```
 */

"use client";

import { useEffect } from "react";
import { useSession } from "~/lib/auth-client";
import { useUserContext } from "~/lib/hooks/use-user-context";
import { useGlobalSSE } from "~/lib/hooks/use-global-sse";

interface AuthWrapperProps {
  children: React.ReactNode;
}

export function AuthWrapper({ children }: Readonly<AuthWrapperProps>) {
  // BetterAuth useSession returns { data, isPending, error }
  const { data: session, isPending } = useSession();

  // Pre-warm user context cache (only when authenticated)
  // This fetches once on mount and caches for instant access on all pages
  const { isReady: contextReady } = useUserContext();

  // Establish single global SSE connection
  // This replaces per-page SSE connections with one shared connection
  const { status: sseStatus } = useGlobalSSE();

  // Debug logging (only in development)
  useEffect(() => {
    if (process.env.NODE_ENV === "development" && session?.user && !isPending) {
      console.log("🔌 [AuthWrapper] Global SSE status:", sseStatus);
      console.log(
        "📦 [AuthWrapper] User context ready:",
        contextReady ? "Yes" : "Loading...",
      );
    }
  }, [sseStatus, contextReady, session, isPending]);

  return <>{children}</>;
}
