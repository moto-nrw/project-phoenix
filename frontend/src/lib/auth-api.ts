// lib/auth-api.ts
import type { ApiError } from "./api-error";
export { handleAuthFailure, refreshToken } from "./auth-failure";
import { createLogger } from "~/lib/logger";

export type { ApiError } from "./api-error";

const logger = createLogger({ component: "AuthAPI" });

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

export async function buildApiError(
  response: Response,
  fallbackMessage: string,
): Promise<ApiError> {
  let message = fallbackMessage;
  let code: string | undefined;
  let details: Record<string, unknown> | undefined;

  try {
    const contentType = response.headers.get("Content-Type") ?? "";
    if (contentType.includes("application/json")) {
      const body = (await response.json()) as {
        error?: string;
        message?: string;
        code?: string;
        details?: Record<string, unknown>;
      };
      message = body?.error ?? body?.message ?? fallbackMessage;
      code = body?.code;
      details = body?.details;
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
  if (code) {
    apiError.code = code;
  }
  if (details) {
    apiError.details = details;
  }

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
async function requestPasswordResetAt(
  endpoint: string,
  email: string,
): Promise<{ message: string }> {
  try {
    const response = await fetch(endpoint, {
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

export async function requestPasswordReset(
  email: string,
): Promise<{ message: string }> {
  return requestPasswordResetAt("/api/auth/password-reset", email);
}

export async function requestParentPasswordReset(
  email: string,
): Promise<{ message: string }> {
  return requestPasswordResetAt("/api/parent/auth/password-reset", email);
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
    const { authService } = await import("./auth-service");
    return await authService.resetPassword({
      token,
      newPassword: password,
      confirmPassword,
    });
  } catch (error) {
    logger.error("password reset confirmation error", { error: String(error) });
    throw error;
  }
}

export async function confirmParentPasswordReset(
  token: string,
  password: string,
  confirmPassword: string,
): Promise<{ message: string }> {
  try {
    const response = await fetch("/api/parent/auth/password-reset/confirm", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        token,
        new_password: password,
        confirm_password: confirmPassword,
      }),
    });

    if (!response.ok) {
      throw await buildApiError(
        response,
        "Fehler beim Zurücksetzen des Passworts",
      );
    }

    return (await response.json()) as { message: string };
  } catch (error) {
    logger.error("parent password reset confirmation error", {
      error: String(error),
    });
    throw error;
  }
}
