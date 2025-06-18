// lib/auth-api.ts
import { signIn, signOut } from "next-auth/react";
import { authService } from "./auth-service";

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
      console.log("Token refresh already in progress, waiting for existing refresh");
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
      if (typeof window === "undefined") {
        console.error("Token refresh attempted from server context");
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
        console.error("Token refresh failed:", response.status);
        return null;
      }

      const data = (await response.json()) as {
        access_token: string;
        refresh_token: string;
      };
      return data;
    } catch (error) {
      console.error("Error refreshing token:", error);
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
  if (typeof window === "undefined") {
    console.error("Auth failure in server context - cannot refresh token or sign out");
    return false;
  }

  try {
    // Try to refresh the token
    const newTokens = await refreshToken();

    if (newTokens) {
      // Token refresh successful
      console.log("Token refreshed successfully");
      // Force a session reload to update the token in the session
      const result = await signIn("credentials", {
        redirect: false,
        internalRefresh: true, // Special flag to indicate this is just a token refresh
        token: newTokens.access_token,
        refreshToken: newTokens.refresh_token,
      });

      if (result?.error) {
        console.error(
            "Session update failed after token refresh:",
            result.error,
        );
        await signOut({ redirect: false });
        return false;
      }

      console.log("Session updated with new tokens");
      return true;
    }

    // If refresh failed, sign out
    console.log("Token refresh failed, signing out");
    await signOut({ redirect: false });

    // Redirect to home page (login)
    window.location.href = "/";

    return false;
  } catch (error) {
    console.error("Auth failure handling error:", error);
    if (typeof window !== "undefined") {
      await signOut({ redirect: false });
    }
    return false;
  }
}

/**
 * Export the auth service for use throughout the application
 */
export { authService };