import { signOut } from "next-auth/react";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AuthAPI" });

// Singleton to manage token refresh and prevent concurrent refreshes
class TokenRefreshManager {
  private refreshPromise: Promise<{
    access_token: string;
    refresh_token: string;
  } | null> | null = null;

  async refreshToken(): Promise<{
    access_token: string;
    refresh_token: string;
  } | null> {
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
      const result = await this.refreshPromise;
      return result;
    } finally {
      // Clear the promise after it completes (success or failure)
      this.refreshPromise = null;
    }
  }

  private async doRefresh(): Promise<{
    access_token: string;
    refresh_token: string;
  } | null> {
    try {
      // Check if we're in a browser context
      if (globalThis.window === undefined) {
        logger.error("token refresh attempted from server context");
        return null;
      }

      const response = await fetch("/api/auth/token", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include", // Important to include cookies
      });

      if (!response.ok) {
        logger.error("token refresh failed", { status: response.status });
        return null;
      }

      const data = (await response.json()) as {
        access_token: string;
        refresh_token: string;
      };
      return data;
    } catch (error) {
      logger.error("error refreshing token", { error: String(error) });
      return null;
    }
  }
}

// Create a singleton instance
const tokenRefreshManager = new TokenRefreshManager();

/**
 * Function to refresh the authentication token
 * @returns Promise with the new tokens or null if refresh failed
 */
export async function refreshToken(): Promise<{
  access_token: string;
  refresh_token: string;
} | null> {
  return tokenRefreshManager.refreshToken();
}

/**
 * Handle a failed authentication by attempting to refresh the token
 * or signing out if that fails
 */
export async function handleAuthFailure(): Promise<boolean> {
  // Check if we're in a server context
  if (globalThis.window === undefined) {
    try {
      const { refreshSessionTokensOnServer } =
        await import("~/server/auth/token-refresh");
      const refreshed = await refreshSessionTokensOnServer();
      return Boolean(refreshed?.accessToken);
    } catch (serverError) {
      logger.error("auth failure in server context, refresh attempt failed", {
        error: String(serverError),
      });
      return false;
    }
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
    const newTokens = await refreshToken();

    if (newTokens) {
      // Token refresh successful. The /api/auth/token route called auth()
      // which refreshed tokens via the JWT callback and cached them in
      // refreshCache (module-level, 5-min TTL). The cookie is NOT updated
      // here — callers must trigger getSession() (e.g. Axios interceptor,
      // SessionProvider refetchInterval) to persist via Set-Cookie.
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
