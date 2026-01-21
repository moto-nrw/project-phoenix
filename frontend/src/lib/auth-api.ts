/**
 * Auth API Module for Project Phoenix
 *
 * BetterAuth Migration Notes:
 * - Session management is now handled via cookies automatically
 * - No manual token refresh needed - BetterAuth handles this
 * - On auth failure, we sign out and redirect to login
 */

import { authClient } from "./auth-client";
// Import with alias for internal use
import { authService as internalAuthService } from "./auth-service";

/**
 * Handle a failed authentication by signing out and redirecting to login
 * BetterAuth handles session refresh automatically via cookies,
 * so if we get a 401, the session is truly expired.
 */
export async function handleAuthFailure(): Promise<boolean> {
  // Check if we're in a server context
  if (globalThis.window === undefined) {
    // In server context, we can't sign out - just return false
    console.error("Auth failure in server context");
    return false;
  }

  try {
    // With BetterAuth, if we receive a 401, the session is expired
    // Sign out and redirect to login
    console.log("Session expired, signing out");
    await authClient.signOut();

    // Redirect to home page (login)
    globalThis.window.location.href = "/";

    return false;
  } catch (error) {
    console.error("Auth failure handling error:", error);
    // Still redirect on error
    if (globalThis.window !== undefined) {
      globalThis.window.location.href = "/";
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
    console.error("Password reset confirmation error:", error);
    throw error;
  }
}

/**
 * Re-export the auth service for use throughout the application
 */
export { authService } from "./auth-service";
