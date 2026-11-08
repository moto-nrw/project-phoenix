"use client";

import { ChoiceModal } from "~/components/ui/choice-modal";
import { Alert } from "~/components/ui/alert";
import type {
  AdminRequestChild,
  ExistingChildChoice,
} from "~/lib/enrollment-admin-api";
import { formatCalendarDate } from "~/lib/localized-date-format";
import { schoolClassLabel } from "~/lib/school-class-label";

// A child that may already be enrolled asks the office before the approval
// (#6053): renew an enrolled child or create a new one. An existing_students
// phase offers no new child.

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

export function ExistingChildHint({
  child,
}: Readonly<{ child: AdminRequestChild }>) {
  if (parentMayNoLongerEnroll(child)) {
    return (
      <Alert
        type="warning"
        announce="off"
        message="Dieser Elternteil darf das Kind nicht mehr anmelden. Prüfen Sie bitte die Angaben."
      />
    );
  }
  if ((child.possible_existing_children?.length ?? 0) > 0) {
    return (
      <Alert
        type="warning"
        announce="off"
        message="Ein Kind mit diesem Namen und Geburtsdatum ist schon angemeldet. Beim Bestätigen wählen Sie, ob es dasselbe Kind ist."
      />
    );
  }
  return null;
}

function choiceDescription(child: AdminRequestChild): string {
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

export function ExistingChildChoiceDialog({
  child,
  onClose,
  onChoose,
}: Readonly<{
  child: AdminRequestChild;
  onClose: () => void;
  onChoose: (choice: ExistingChildChoice) => void;
}>) {
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
  return (
    <ChoiceModal
      isOpen
      title="Ist das Kind schon angemeldet?"
      description={choiceDescription(child)}
      options={[...candidates, ...newChild]}
      onClose={onClose}
      onSelect={(value) =>
        onChoose(
          value === "new"
            ? { createNewStudent: true }
            : { existingStudentId: value },
        )
      }
    />
  );
}
