import { signOut } from "next-auth/react";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AuthAPI" });

// Singleton to manage session renewal and prevent concurrent renewals
class TokenRefreshManager {
  private refreshPromise: Promise<boolean> | null = null;

  async refreshToken(): Promise<boolean> {
    // If a refresh is already in progress, return the existing promise
    if (this.refreshPromise) {
      logger.debug(
        "token refresh already in progress, waiting for existing refresh",
      );
      return this.refreshPromise;
    }

    // Create a new refresh promise
    this.refreshPromise = this.doRefresh();

    try {
      return await this.refreshPromise;
    } finally {
      // Clear the promise after it completes (success or failure)
      this.refreshPromise = null;
    }
  }

  private async doRefresh(): Promise<boolean> {
    try {
      // Check if we're in a browser context
      if (globalThis.window === undefined) {
        logger.error("token refresh attempted from server context");
        return false;
      }

      // The route renews the session server-side and answers without a
      // token (#5005); its Set-Cookie carries the rotated session.
      const response = await fetch("/api/auth/token", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include", // Important to include cookies
      });

      if (!response.ok) {
        logger.error("token refresh failed", { status: response.status });
        return false;
      }

      return true;
    } catch (error) {
      logger.error("error refreshing token", { error: String(error) });
      return false;
    }
  }
}

// Create a singleton instance
const tokenRefreshManager = new TokenRefreshManager();

/**
 * Renew the session's tokens on the server.
 * @returns whether the session still holds a token afterwards
 */
export async function refreshToken(): Promise<boolean> {
  return tokenRefreshManager.refreshToken();
}

/**
 * Handle a failed authentication by attempting to refresh the token
 * or signing out if that fails
 */
export async function handleAuthFailure(): Promise<boolean> {
  // Check if we're in a server context
  if (globalThis.window === undefined) {
    logger.error("auth failure handling attempted from server context");
    return false;
  }

  try {
    // The JWT callback in NextAuth handles automatic token refresh
    // If we're here with a 401, it likely means:
    // 1. The JWT callback's refresh already failed, OR
    // 2. We're in a race condition where client and server both tried to refresh

    // Let's check if we recently had a successful refresh
    const lastRefresh = sessionStorage.getItem("lastSuccessfulRefresh");
    if (lastRefresh) {
      const lastRefreshTime = Number.parseInt(lastRefresh, 10);
      const timeSinceRefresh = Date.now() - lastRefreshTime;

      // If we refreshed less than 5 seconds ago, just retry the request
      if (timeSinceRefresh < 5000) {
        logger.debug("recently refreshed tokens, retrying request");
        return true;
      }
    }

    // Try to refresh the token one more time
    const refreshed = await refreshToken();

    if (refreshed) {
      // Token refresh successful. The /api/auth/token route called auth()
      // which refreshed tokens via the JWT callback and cached them in
      // refreshCache (module-level, 5-min TTL). Callers still trigger
      // getSession() (sessionFetch, SessionProvider refetchInterval) before
      // they retry, so the next request reads the current cookie.
      sessionStorage.setItem("lastSuccessfulRefresh", Date.now().toString());
      return true;
    }

    // If refresh failed, sign out
    logger.warn("token refresh failed, signing out");
    await signOut({ redirect: false });

    // Redirect to home page (login)
    globalThis.window.location.href = "/";

    return false;
  } catch (error) {
    logger.error("auth failure handling error", { error: String(error) });
    if (globalThis.window !== undefined) {
      await signOut({ redirect: false });
    }
    return false;
  }
}
