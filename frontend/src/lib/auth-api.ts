// lib/auth-api.ts
import { signOut } from "next-auth/react";
// Import with alias for internal use
import { authService as internalAuthService } from "./auth-service";
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
          logger.info("session updated with new tokens");
        } else {
          logger.error("failed to update session with new tokens", {
            error: result?.error,
          });
        }
      } catch (sessionError) {
        logger.error("error updating session", { error: String(sessionError) });
      }

      // Return true to retry the original request regardless of session update
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

async function buildApiError(
  response: Response,
  fallbackMessage: string,
): Promise<ApiError> {
  let message = fallbackMessage;

  try {
    const contentType = response.headers.get("Content-Type") ?? "";
    if (contentType.includes("application/json")) {
      const body = (await response.json()) as {
        error?: string;
        message?: string;
      };
      message = body?.error ?? body?.message ?? fallbackMessage;
    } else {
      const text = (await response.text()).trim();
      if (text) {
        message = text;
      }
    }
  } catch (parseError) {
    logger.warn("failed to parse error response", {
      error: String(parseError),
    });
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
export async function requestPasswordReset(
  email: string,
): Promise<{ message: string }> {
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
        "Fehler beim Senden der Passwort-Zurücksetzen-E-Mail",
      );
    }

    return (await response.json()) as { message: string };
  } catch (error) {
    logger.error("password reset request error", { error: String(error) });
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
  password: string,
  confirmPassword: string,
): Promise<{ message: string }> {
  try {
    return await internalAuthService.resetPassword({
      token,
      newPassword: password,
      confirmPassword,
    });
  } catch (error) {
    logger.error("password reset confirmation error", { error: String(error) });
    throw error;
  }
}

/**
 * Re-export the auth service for use throughout the application
 */
export { authService } from "./auth-service";
