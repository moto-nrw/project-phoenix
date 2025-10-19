// lib/auth-api.ts
import { signOut } from "next-auth/react";
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
    // The JWT callback in NextAuth handles automatic token refresh
    // If we're here with a 401, it likely means:
    // 1. The JWT callback's refresh already failed, OR
    // 2. We're in a race condition where client and server both tried to refresh
    
    // Let's check if we recently had a successful refresh
    const lastRefresh = sessionStorage.getItem("lastSuccessfulRefresh");
    if (lastRefresh) {
      const lastRefreshTime = parseInt(lastRefresh, 10);
      const timeSinceRefresh = Date.now() - lastRefreshTime;
      
      // If we refreshed less than 5 seconds ago, just retry the request
      if (timeSinceRefresh < 5000) {
        console.log("Recently refreshed tokens, retrying request...");
        return true;
      }
    }
    
    // Try to refresh the token one more time
    const newTokens = await refreshToken();

    if (newTokens) {
      // Token refresh successful
      
      // Mark the time of successful refresh
      sessionStorage.setItem("lastSuccessfulRefresh", Date.now().toString());
      
      // IMPORTANT: Update the NextAuth session with new tokens
      try {
        // Use signIn with internalRefresh to update the session
        const { signIn } = await import("next-auth/react");
        
        const result = await signIn("credentials", {
          internalRefresh: "true",
          token: newTokens.access_token,
          refreshToken: newTokens.refresh_token,
          redirect: false,
        });
        
        if (result?.ok) {
          console.log("Session updated with new tokens");
        } else {
          console.error("Failed to update session with new tokens:", result?.error);
        }
      } catch (sessionError) {
        console.error("Error updating session:", sessionError);
      }
      
      // Return true to retry the original request regardless of session update
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

export type ApiError = Error & { status?: number; retryAfterSeconds?: number };

function parseRetryAfter(value: string | null): number | null {
  if (!value) {
    return null;
  }

  const numeric = Number(value);
  if (!Number.isNaN(numeric)) {
    return Math.max(0, Math.round(numeric));
  }

  const date = Date.parse(value);
  if (!Number.isNaN(date)) {
    const diffMs = date - Date.now();
    return diffMs > 0 ? Math.ceil(diffMs / 1000) : 0;
  }

  return null;
}

async function buildApiError(response: Response, fallbackMessage: string): Promise<ApiError> {
  let message = fallbackMessage;

  try {
    const contentType = response.headers.get("Content-Type") ?? "";
    if (contentType.includes("application/json")) {
      const body = await response.json() as { error?: string; message?: string };
      message = body?.error ?? body?.message ?? fallbackMessage;
    } else {
      const text = (await response.text()).trim();
      if (text) {
        message = text;
      }
    }
  } catch (parseError) {
    console.warn("Failed to parse error response", parseError);
  }

  const apiError = new Error(message) as ApiError;
  apiError.status = response.status;

  const retryAfter = parseRetryAfter(response.headers.get("Retry-After"));
  if (retryAfter !== null) {
    apiError.retryAfterSeconds = retryAfter;
  }

  return apiError;
}

/**
 * Request a password reset email for the given email address
 * @param email - The email address to send the reset link to
 * @returns Promise with success message or error
 */
export async function requestPasswordReset(email: string): Promise<{ message: string }> {
  try {
    const response = await fetch("/api/auth/password-reset", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ email }),
    });

    if (!response.ok) {
      throw await buildApiError(
        response,
        "Fehler beim Senden der Passwort-Zurücksetzen-E-Mail"
      );
    }

    return await response.json() as { message: string };
  } catch (error) {
    console.error("Password reset request error:", error);
    throw error;
  }
}

/**
 * Confirm password reset with token and new password
 * @param token - The reset token from the email link
 * @param password - The new password
 * @returns Promise with success message or error
 */
export async function confirmPasswordReset(
  token: string,
  password: string
): Promise<{ message: string }> {
  try {
    const response = await fetch("/api/auth/password-reset/confirm", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ token, password }),
    });

    if (!response.ok) {
      throw await buildApiError(
        response,
        "Fehler beim Zurücksetzen des Passworts"
      );
    }

    return await response.json() as { message: string };
  } catch (error) {
    console.error("Password reset confirmation error:", error);
    throw error;
  }
}

/**
 * Export the auth service for use throughout the application
 */
export { authService };
