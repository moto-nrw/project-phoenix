/**
 * Helper functions for student privacy consent handling in API routes
 * Extracted to reduce cognitive complexity in route handlers
 */

/**
 * Privacy consent response structure
 */
interface PrivacyConsentData {
  accepted: boolean;
  data_retention_days: number;
}

/**
 * Wrapped privacy consent response from backend
 */
interface WrappedPrivacyConsentResponse {
  data?: PrivacyConsentData;
  accepted?: boolean;
  data_retention_days?: number;
}

/**
 * Default privacy consent values
 */
export const DEFAULT_PRIVACY_CONSENT = {
  privacy_consent_accepted: false,
  data_retention_days: 30,
} as const;

/**
 * Fetches privacy consent data for a student
 * Handles both wrapped and unwrapped response formats
 *
 * @param studentId - The student ID
 * @param apiGet - The API GET function
 * @param token - Authentication token
 * @returns Privacy consent data or defaults if not found/forbidden
 */
export async function fetchPrivacyConsent(
  studentId: string,
  apiGet: (endpoint: string, token: string) => Promise<unknown>,
  token: string,
): Promise<{ privacy_consent_accepted: boolean; data_retention_days: number }> {
  try {
    const consentResponse = await apiGet(
      `/api/students/${studentId}/privacy-consent`,
      token,
    );

    const consentData = extractConsentData(consentResponse);
    if (consentData) {
      return {
        privacy_consent_accepted: consentData.accepted,
        data_retention_days: consentData.data_retention_days,
      };
    }
  } catch (e) {
    // Differentiate 404 (no consent yet) and 403 (no permission) from other errors
    if (e instanceof Error) {
      const is404or403 =
        e.message.includes("(404)") || e.message.includes("(403)");
      if (!is404or403) {
        // Re-throw system/network errors
        throw e;
      }
    } else {
      throw e;
    }
    // For 404 or 403, fall through to defaults
  }

  return DEFAULT_PRIVACY_CONSENT;
}

/**
 * Extracts consent data from various response formats
 * Handles both wrapped ({data: {accepted, data_retention_days}}) and unwrapped formats
 *
 * @param response - The API response
 * @returns Extracted consent data or null if not found
 */
function extractConsentData(
  response: unknown,
): PrivacyConsentData | null {
  if (!response || typeof response !== "object") {
    return null;
  }

  const wrappedResponse = response as WrappedPrivacyConsentResponse;

  // Check if response is wrapped in { data: ... }
  if ("data" in wrappedResponse && wrappedResponse.data) {
    const wrappedData = wrappedResponse.data;
    if (
      typeof wrappedData === "object" &&
      "accepted" in wrappedData &&
      "data_retention_days" in wrappedData
    ) {
      const data = wrappedData as { accepted: unknown; data_retention_days: unknown };
      if (
        typeof data.accepted === "boolean" &&
        typeof data.data_retention_days === "number"
      ) {
        return {
          accepted: data.accepted,
          data_retention_days: data.data_retention_days,
        };
      }
    }
  }

  // Check if response has fields directly
  if (
    "accepted" in wrappedResponse &&
    "data_retention_days" in wrappedResponse &&
    typeof wrappedResponse.accepted === "boolean" &&
    typeof wrappedResponse.data_retention_days === "number"
  ) {
    return {
      accepted: wrappedResponse.accepted,
      data_retention_days: wrappedResponse.data_retention_days,
    };
  }

  return null;
}

/**
 * Determines if privacy consent should be created
 * Only create consent if user explicitly accepted OR specified custom retention days
 *
 * @param accepted - Privacy consent acceptance status
 * @param retentionDays - Data retention days
 * @returns True if consent should be created
 */
export function shouldCreatePrivacyConsent(
  accepted?: boolean,
  retentionDays?: number,
): boolean {
  return (
    accepted === true ||
    (retentionDays !== undefined && retentionDays !== 30 && retentionDays !== null)
  );
}

/**
 * Updates or creates privacy consent for a student
 * Logs the operation for debugging
 *
 * @param studentId - The student ID
 * @param apiPut - The API PUT function
 * @param token - Authentication token
 * @param accepted - Privacy consent acceptance status
 * @param retentionDays - Data retention days
 * @param operationName - Name of the operation (for logging)
 */
export async function updatePrivacyConsent(
  studentId: number | string,
  apiPut: (endpoint: string, token: string, data: unknown) => Promise<unknown>,
  token: string,
  accepted?: boolean,
  retentionDays?: number,
  operationName = "Operation",
): Promise<void> {
  console.log(
    `[${operationName}] Updating privacy consent for student ${studentId} - accepted:`,
    accepted,
    "retention:",
    retentionDays,
  );

  await apiPut(`/api/students/${studentId}/privacy-consent`, token, {
    policy_version: "1.0",
    accepted: accepted ?? false,
    data_retention_days: retentionDays ?? 30,
  });

  console.log(
    `[${operationName}] Privacy consent updated - accepted=${accepted}, retention=${retentionDays}`,
  );
}
