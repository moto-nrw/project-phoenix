// The privacy consent of a student as the student BFF routes read and write
// it. Student Presence owns the consent; without a stored one it answers with
// the school's defaults (gdpr.privacy_consent_retention_days).

import { apiGet, apiPut } from "~/lib/api-helpers.server";
import { bffError, isApiResponseError } from "~/lib/api-response-error.server";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StudentPrivacyConsent" });

/** The two consent fields a student form edits. */
interface ConsentPair {
  privacy_consent_accepted: boolean;
  data_retention_days: number;
}

/** A request carries the pair only when the person changed it. */
export type ConsentPairInput = Partial<ConsentPair>;

// The policy version the backend states as its default
// (studentpresence defaultPrivacyPolicyVersion).
const POLICY_VERSION = "1.0";

interface BackendConsent {
  accepted: boolean;
  data_retention_days: number;
}

/**
 * The pair a request sent, or null when it sent neither field. The backend
 * write is a full upsert, so a lone field would silently reset the other.
 */
export function sentConsentPair(body: ConsentPairInput): ConsentPair | null {
  const { privacy_consent_accepted, data_retention_days } = body;
  if (
    privacy_consent_accepted === undefined &&
    data_retention_days === undefined
  )
    return null;
  if (
    privacy_consent_accepted === undefined ||
    data_retention_days === undefined
  )
    throw bffError(
      400,
      "privacy_consent_accepted and data_retention_days are sent together",
    );
  return { privacy_consent_accepted, data_retention_days };
}

export async function writeConsent(
  studentId: string | number,
  token: string,
  pair: ConsentPair,
): Promise<void> {
  await apiPut(`/api/students/${studentId}/privacy-consent`, token, {
    policy_version: POLICY_VERSION,
    accepted: pair.privacy_consent_accepted,
    data_retention_days: pair.data_retention_days,
  });
}

/**
 * The consent read back after a student write. Everything is committed by
 * then, so an unreadable consent must not turn the write into a failure: the
 * pair just written stands in, and with nothing written the keys stay out and
 * the form keeps the consent it holds.
 */
export async function readConsentAfterWrite(
  studentId: string | number,
  token: string,
  written: ConsentPair | null,
): Promise<ConsentPair | null> {
  try {
    return await readConsent(studentId, token);
  } catch (error) {
    logger.warn("privacy consent read-back failed after student write", {
      student_id: studentId,
      error: error instanceof Error ? error.message : String(error),
    });
    return written;
  }
}

/**
 * The stored consent, or the school's defaults when none is stored. Null when
 * the caller may not read it (403): the form then keeps what it holds.
 */
export async function readConsent(
  studentId: string | number,
  token: string,
): Promise<ConsentPair | null> {
  try {
    const response = await apiGet<{ data: BackendConsent }>(
      `/api/students/${studentId}/privacy-consent`,
      token,
    );
    return {
      privacy_consent_accepted: response.data.accepted,
      data_retention_days: response.data.data_retention_days,
    };
  } catch (error) {
    if (isApiResponseError(error, 403)) return null;
    throw error;
  }
}
