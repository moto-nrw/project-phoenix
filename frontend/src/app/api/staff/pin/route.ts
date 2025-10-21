// app/api/staff/pin/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPut } from "~/lib/api-helpers";
import { createGetHandler, createPutHandler } from "~/lib/route-wrapper";

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
 * Handler for GET /api/staff/pin
 * Returns the current user's PIN status
 */
export const GET = createGetHandler(async (request: NextRequest, token: string) => {
  try {
    // Fetch PIN status from backend API
    const response = await apiGet<BackendPINStatusResponse>("/api/staff/pin", token);
    
    console.log("PIN status response:", JSON.stringify(response, null, 2));
    
    // Extract data from the backend response - backend already returns the correct structure
    return response.data;
  } catch (error) {
    console.error("Error fetching PIN status:", error);
    
    // Check for specific error types
    if (error instanceof Error && error.message.includes("404")) {
      throw new Error("Account not found");
    }
    if (error instanceof Error && error.message.includes("403")) {
      throw new Error("Permission denied: Unable to access PIN settings");
    }
    
    // Re-throw other errors
    throw error;
  }
});

/**
 * Handler for PUT /api/staff/pin
 * Updates the current user's PIN
 */
export const PUT = createPutHandler<BackendPINUpdateResponse, PINUpdateRequest>(
  async (_request: NextRequest, body: PINUpdateRequest, token: string) => {
    // Validate required fields
    if (!body.new_pin || body.new_pin.trim() === '') {
      throw new Error('Neue PIN ist erforderlich');
    }
    
    // Validate PIN format (4 digits)
    if (!/^\d{4}$/.test(body.new_pin)) {
      throw new Error('PIN muss aus genau 4 Ziffern bestehen');
    }
    
    try {
      // Update PIN via backend API
      const response = await apiPut<BackendPINUpdateResponse>("/api/staff/pin", token, body);
      
      console.log("PIN update response:", JSON.stringify(response, null, 2));
      
      // Return the full backend response (route wrapper will extract data)
      return response;
    } catch (error) {
      console.error("Error updating PIN:", error);
      
      // Check for specific error types and provide German error messages
      if (error instanceof Error) {
        const errorMessage = error.message.toLowerCase();
        
        if (errorMessage.includes("403") || errorMessage.includes("forbidden")) {
          throw new Error("Zugriff verweigert: Sie können Ihre PIN nicht verwalten");
        }
        if (errorMessage.includes("401") || errorMessage.includes("current pin is incorrect")) {
          throw new Error("Aktuelle PIN ist falsch");
        }
        if (errorMessage.includes("locked") || errorMessage.includes("temporarily locked")) {
          throw new Error("Konto ist vorübergehend gesperrt aufgrund fehlgeschlagener PIN-Versuche");
        }
        if (errorMessage.includes("current pin is required")) {
          throw new Error("Aktuelle PIN ist erforderlich zum Ändern der bestehenden PIN");
        }
        if (errorMessage.includes("pin must be exactly 4 digits")) {
          throw new Error("PIN muss aus genau 4 Ziffern bestehen");
        }
        if (errorMessage.includes("pin must contain only digits")) {
          throw new Error("PIN darf nur Ziffern enthalten");
        }
        if (errorMessage.includes("404") || errorMessage.includes("not found")) {
          throw new Error("Mitarbeiter nicht gefunden");
        }
      }
      
      // Re-throw other errors with generic German message
      throw new Error("Fehler beim Aktualisieren der PIN. Bitte versuchen Sie es erneut.");
    }
  }
);