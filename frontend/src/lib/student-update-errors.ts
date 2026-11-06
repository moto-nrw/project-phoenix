// The refusals of the student update that the forms answer differently from a
// plain failure: the companion-plan question, the stale "läuft mit" list, the
// stranded linked child, and the half-saved privacy consent. Each arrives as
// the ApiError of updateStudent (~/lib/student-api). Keyed off the code, never
// the status: the student PUT answers 409 and 400 for several reasons.

import { ApiError, enrichApiError, problemDetail } from "./api-error";
import {
  ALL_COMPANION_WEEKDAYS,
  type CompanionExtensionConfirmation,
  type CompanionWeekday,
} from "./student-companion-api";

function parsedBody(body: string): unknown {
  try {
    return JSON.parse(body) as unknown;
  } catch {
    return undefined;
  }
}

/**
 * The conflict list of a 409 body in the confirmation shape. Anything
 * unparsable yields an empty list, which makes the retry ask again rather than
 * confirm something we could not read.
 */
function conflictExtensions(body: unknown): CompanionExtensionConfirmation[] {
  const conflicts =
    body && typeof body === "object"
      ? (
          body as {
            conflicts?: { student_id?: number | string; weekdays?: string[] }[];
          }
        ).conflicts
      : undefined;
  return (Array.isArray(conflicts) ? conflicts : [])
    .filter((conflict) => conflict.student_id !== undefined)
    .map((conflict) => ({
      companion_student_id: String(conflict.student_id),
      weekdays: (conflict.weekdays ?? []).filter(
        (day): day is CompanionWeekday =>
          ALL_COMPANION_WEEKDAYS.includes(day as CompanionWeekday),
      ),
    }));
}

/**
 * Raised when a student update was refused because a linked child's own
 * departure plan does not yet allow leaving with another child on the
 * requested days. Nothing was written; re-send with extend_companion_plans
 * after the user confirms.
 */
export class CompanionPlanConflictError extends ApiError {
  /**
   * The conflicting children and weekdays exactly as the backend reported them.
   * The confirmation has to name them again on the retry, so the backend can
   * tell "the user agreed to this" apart from "this appeared afterwards".
   */
  readonly conflicts: CompanionExtensionConfirmation[];

  constructor(body: string) {
    const parsed = parsedBody(body);
    super(problemDetail(parsed)?.trim() ?? "companion plan conflict", 409);
    // Keeps code and details, including the half-saved consent marker.
    enrichApiError(this, parsed, 409);
    this.name = "CompanionPlanConflictError";
    this.conflicts = conflictExtensions(parsed);
  }
}

/**
 * Whether a complete 409 body is the companion-plan question. Strict on
 * purpose: the student PUT answers 409 for several unrelated reasons
 * (students.companion_lock_busy, students.sick_excused_conflict, …), and typing
 * one of those as the question would replace its real contract with an empty
 * confirmation the user cannot answer.
 */
export function isCompanionPlanConflictResponse(body: string): boolean {
  return conflictExtensions(parsedBody(body)).length > 0;
}

/**
 * The submitted "läuft mit" list was built on a snapshot someone else has
 * since replaced. Re-sending the SAME list is exactly what the refusal
 * prevents, so the form reloads first and lets the user redo the edit.
 */
export function isCompanionsChanged(err: unknown): boolean {
  return err instanceof ApiError && err.code === "students.companions_changed";
}

/**
 * Removing a link would leave the linked child with an accompanied departure
 * plan and no note and no other link. Nothing to confirm: the user has to fix
 * that child's Heimweg first.
 */
export function isCompanionDepartureRefusal(err: unknown): boolean {
  return (
    err instanceof ApiError &&
    err.code === "students.companion_would_lose_departure"
  );
}

/** The detail key of a student PUT that failed after its consent write. */
export const PRIVACY_CONSENT_SAVED_DETAIL = "privacy_consent_saved";

/**
 * The student PUT writes the privacy consent first, against another backend
 * endpoint and transaction. When the student write then fails, the consent is
 * stored and the rest is not; the route marks that error so the form can say
 * which half landed instead of inviting the user to abandon the retry.
 */
export function isPrivacyConsentSaved(err: unknown): boolean {
  return (
    err instanceof ApiError &&
    err.details?.[PRIVACY_CONSENT_SAVED_DETAIL] === true
  );
}
