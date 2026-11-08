import { ApiError } from "~/lib/api-error";
import type {
  AdminExistingChildCandidate,
  AdminRequestChild,
  ExistingChildChoice,
} from "~/lib/enrollment-admin-api";
import { formatCalendarDate } from "~/lib/localized-date-format";
import { schoolClassLabel } from "~/lib/school-class-label";

// A child that may already be enrolled asks the office before the approval
// (#6053): renew an enrolled child or create a new one. An existing_students
// phase offers no new child. The manual enrollment asks the same question
// when its approval is refused with the candidates (#6148).

/** What the choice dialog needs to know about the child. */
type ExistingChildChoiceSubject = Pick<
  AdminRequestChild,
  | "first_name"
  | "last_name"
  | "possible_existing_children"
  | "existing_child_only"
>;

/** Whether approving the child needs the office's choice first. */
export function needsExistingChildChoice(child: AdminRequestChild): boolean {
  return (
    child.existing_child_choice_required === true ||
    (child.possible_existing_children?.length ?? 0) > 0
  );
}

/**
 * The parent's own relationship pinned the child at submission, and the
 * school removed it since: the pin no longer authorizes the family.
 */
function parentMayNoLongerEnroll(child: AdminRequestChild): boolean {
  return (
    child.existing_child_choice_required === true &&
    Boolean(child.matched_student_id)
  );
}

export function existingChildHintMessage(
  child: AdminRequestChild,
): string | null {
  if (parentMayNoLongerEnroll(child)) {
    return "Dieser Elternteil darf das Kind nicht mehr anmelden. Prüfen Sie bitte die Angaben.";
  }
  if ((child.possible_existing_children?.length ?? 0) > 0) {
    return "Ein Kind mit diesem Namen und Geburtsdatum ist schon angemeldet. Beim Bestätigen wählen Sie, ob es dasselbe Kind ist.";
  }
  return null;
}

export function existingChildChoiceDescription(
  child: ExistingChildChoiceSubject,
): string {
  const hasCandidates = (child.possible_existing_children?.length ?? 0) > 0;
  const existingOnly = child.existing_child_only === true;
  if (!hasCandidates) {
    return existingOnly
      ? "Zu diesen Angaben wurde kein angemeldetes Kind gefunden. Diese Anmeldung ist nur für Kinder, die schon angemeldet sind. Prüfen Sie Name und Geburtsdatum. Sonst lehnen Sie die Anmeldung ab."
      : "Zu diesen Angaben wurde kein angemeldetes Kind gefunden. Prüfen Sie Name und Geburtsdatum. Ist das Kind schon vorhanden, wählen Sie „Abbrechen“.";
  }
  const base = `Bitte wählen Sie für ${child.first_name} ${child.last_name}. Beim bestehenden Kind wird nur die Anmeldung verlängert. Die Eltern aus dieser Anmeldung bekommen dadurch keinen Zugang zum Kind.`;
  return existingOnly
    ? `${base} Ist es ein anderes Kind, lehnen Sie die Anmeldung ab.`
    : base;
}

export function existingChildChoiceOptions(child: ExistingChildChoiceSubject) {
  const candidates = (child.possible_existing_children ?? []).map(
    (candidate) => ({
      value: candidate.student_id,
      label: "Bestehendes Kind verwenden",
      description: [
        `${candidate.first_name} ${candidate.last_name}`,
        candidate.school_class ? schoolClassLabel(candidate.school_class) : "",
        candidate.date_of_birth
          ? `geboren am ${formatCalendarDate(candidate.date_of_birth, "de-DE")}`
          : "",
      ]
        .filter(Boolean)
        .join(" · "),
    }),
  );
  const newChild =
    child.existing_child_only === true
      ? []
      : [
          {
            value: "new",
            label: "Als neues Kind anlegen",
            description: "Wählen Sie dies nur, wenn es ein anderes Kind ist.",
          },
        ];
  return [...candidates, ...newChild];
}

function isCandidate(value: unknown): value is AdminExistingChildCandidate {
  if (!value || typeof value !== "object") return false;
  const candidate = value as Record<string, unknown>;
  return (
    typeof candidate.student_id === "string" &&
    typeof candidate.first_name === "string" &&
    typeof candidate.last_name === "string"
  );
}

/**
 * The enrolled children a refused approval names (#6148), or null when the
 * error is no such refusal or carries no usable candidate.
 */
export function possiblyEnrolledCandidates(
  error: unknown,
): AdminExistingChildCandidate[] | null {
  if (
    !(error instanceof ApiError) ||
    error.code !== "enrollment.child_possibly_enrolled"
  ) {
    return null;
  }
  const raw = error.details?.candidates;
  if (!Array.isArray(raw) || raw.length === 0 || !raw.every(isCandidate)) {
    return null;
  }
  return raw;
}

/**
 * Sends an approval that may meet an already enrolled child: on the refusal
 * with candidates it asks the office and sends once more with the answer.
 * Null means the office cancelled and nothing was saved.
 */
export async function sendWithExistingChildChoice<T>(
  send: (choice?: ExistingChildChoice) => Promise<T>,
  ask: (
    candidates: AdminExistingChildCandidate[],
  ) => Promise<ExistingChildChoice | null>,
): Promise<T | null> {
  try {
    return await send(undefined);
  } catch (error) {
    const candidates = possiblyEnrolledCandidates(error);
    if (!candidates) throw error;
    const choice = await ask(candidates);
    if (!choice) return null;
    return send(choice);
  }
}
