// app/api/staff/pin/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPut } from "~/lib/api-helpers";
import { createGetHandler, createPutHandler } from "~/lib/route-wrapper";
import { validatePinOrThrow } from "~/lib/pin";

/**
 * Maps PIN update error messages to German user-friendly messages
 * Extracted to reduce cognitive complexity (S3776)
 */
function mapPinUpdateError(error: Error): string {
  const errorMessage = error.message.toLowerCase();

  if (errorMessage.includes("403") || errorMessage.includes("forbidden")) {
    return "Zugriff verweigert: Sie können Ihre PIN nicht verwalten";
  }
  if (
    errorMessage.includes("401") ||
    errorMessage.includes("current pin is incorrect")
  ) {
    return "Aktuelle PIN ist falsch";
  }
  if (
    errorMessage.includes("locked") ||
    errorMessage.includes("temporarily locked")
  ) {
    return "Konto ist vorübergehend gesperrt aufgrund fehlgeschlagener PIN-Versuche";
  }
  if (errorMessage.includes("current pin is required")) {
    return "Aktuelle PIN ist erforderlich zum Ändern der bestehenden PIN";
  }
  if (errorMessage.includes("pin must be exactly 4 digits")) {
    return "PIN muss aus genau 4 Ziffern bestehen";
  }
  if (errorMessage.includes("pin must contain only digits")) {
    return "PIN darf nur Ziffern enthalten";
  }
  if (errorMessage.includes("404") || errorMessage.includes("not found")) {
    return "Konto nicht gefunden";
  }
  return "Fehler beim Aktualisieren der PIN. Bitte versuchen Sie es erneut.";
}

/**
 * Type definition for PIN status response from backend
 */
interface BackendPINStatusResponse {
  status: string;
  data: {
    has_pin: boolean;
    last_changed?: string;
  };
  message: string;
}

/**
 * Type definition for PIN update request
 */
interface PINUpdateRequest {
  current_pin?: string | null; // null for first-time setup
  new_pin: string;
}

/**
 * Type definition for PIN update response from backend
 */
interface BackendPINUpdateResponse {
  status: string;
  data: {
    success: boolean;
    message: string;
  };
  message: string;
}

/**
 * Logs a message only in non-production environments
 */
function logInDevelopment(message: string, data?: unknown): void {
  if (process.env.NODE_ENV !== "production") {
    if (data) {
      console.log(message, JSON.stringify(data, null, 2));
    } else {
      console.log(message);
    }
  }
}

/**
 * Handles errors from PIN fetch operations
 */
function handlePINFetchError(error: unknown): Error {
  if (error instanceof Error && error.message.includes("404")) {
    return new Error("Konto nicht gefunden");
  }
  if (error instanceof Error && error.message.includes("403")) {
    return new Error("Permission denied: Unable to access PIN settings");
  }
  return error as Error;
}

/**
 * Validates PIN update request body
 */
function validatePINUpdateRequest(body: PINUpdateRequest): void {
  if (!body.new_pin || body.new_pin.trim() === "") {
    throw new Error("Neue PIN ist erforderlich");
  }
  validatePinOrThrow(body.new_pin);
}

/**
 * Performs PIN update and returns current PIN status
 */
async function performPINUpdate(
  token: string,
  body: PINUpdateRequest,
): Promise<BackendPINStatusResponse["data"]> {
  const response = await apiPut<BackendPINUpdateResponse>(
    "/api/staff/pin",
    token,
    body,
  );

  logInDevelopment("PIN update response:", response);

  const statusResponse = await apiGet<BackendPINStatusResponse>(
    "/api/staff/pin",
    token,
  );
  return statusResponse.data;
}

/**
 * Handler for GET /api/staff/pin
 * Returns the current user's PIN status
 */
export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    try {
      const response = await apiGet<BackendPINStatusResponse>(
        "/api/staff/pin",
        token,
      );

      logInDevelopment("PIN status response:", response);

      return response.data;
    } catch (error) {
      logInDevelopment("Error fetching PIN status:", error);
      throw handlePINFetchError(error);
    }
  },
);

/**
 * Handler for PUT /api/staff/pin
 * Updates the current user's PIN
 */
export const PUT = createPutHandler<
  BackendPINStatusResponse["data"],
  PINUpdateRequest
>(async (_request: NextRequest, body: PINUpdateRequest, token: string) => {
  validatePINUpdateRequest(body);

  try {
    return await performPINUpdate(token, body);
  } catch (error) {
    logInDevelopment("Error updating PIN:", error);
    throw new Error(
      error instanceof Error
        ? mapPinUpdateError(error)
        : "Fehler beim Aktualisieren der PIN. Bitte versuchen Sie es erneut.",
    );
  }
});
