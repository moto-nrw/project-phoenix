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
import { useUserContext } from "~/lib/hooks/use-user-context";
import { useGlobalSSE } from "~/lib/hooks/use-global-sse";

interface AuthWrapperProps {
  children: React.ReactNode;
}

export function AuthWrapper({ children }: Readonly<AuthWrapperProps>) {
  const { status } = useSession();

  // Pre-warm user context cache (only when authenticated)
  // This fetches once on mount and caches for instant access on all pages
  const { isReady: contextReady } = useUserContext();

  // Establish single global SSE connection
  // This replaces per-page SSE connections with one shared connection
  const { status: sseStatus } = useGlobalSSE();

  // Debug logging (only in development)
  useEffect(() => {
    if (process.env.NODE_ENV === "development" && status === "authenticated") {
      console.log("🔌 [AuthWrapper] Global SSE status:", sseStatus);
      console.log(
        "📦 [AuthWrapper] User context ready:",
        contextReady ? "Yes" : "Loading...",
      );
    }
  }, [sseStatus, contextReady, status]);

  return <>{children}</>;
}
