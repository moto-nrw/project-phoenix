"use client";

import { useState } from "react";

import { ChoiceTile } from "~/components/ui/choice-tile";
import { ConfirmationModal } from "~/components/ui/modal";
import { Radio } from "~/components/ui/radio";
import { formatDate } from "~/lib/date-helpers";
import type {
  AdminExistingChildCandidate,
  ExistingChildChoice,
} from "~/lib/enrollment-admin-api";
import { schoolClassLabel } from "~/lib/school-class-label";

const NEW_CHILD = "new";

/**
 * Asks the office, before an approval, whether an enrolled child with the
 * same name and birthday is the child of this enrollment (#6053). Renewing
 * it links none of the family's guardians; creating a new child keeps the
 * usual approval.
 */
export function ExistingChildChoiceModal({
  isOpen,
  childName,
  candidates,
  onClose,
  onConfirm,
}: Readonly<{
  isOpen: boolean;
  childName: string;
  candidates: readonly AdminExistingChildCandidate[];
  onClose: () => void;
  onConfirm: (choice: ExistingChildChoice) => void;
}>) {
  const [selected, setSelected] = useState<string | null>(null);
  const name = `existing-child-choice-${childName}`;

  const confirm = () => {
    if (selected === null) return;
    onConfirm(
      selected === NEW_CHILD
        ? { createNewStudent: true }
        : { existingStudentId: selected },
    );
  };

  return (
    <ConfirmationModal
      isOpen={isOpen}
      onClose={onClose}
      onConfirm={confirm}
      title="Ist das Kind schon angemeldet?"
      confirmText="Bestätigen"
      confirmVariant="success"
      isConfirmDisabled={selected === null}
    >
      <p className="text-sm text-gray-700">
        {candidates.length === 1
          ? `An der Schule ist schon ein Kind mit dem Namen ${childName} und diesem Geburtsdatum angemeldet.`
          : `An der Schule sind schon ${candidates.length} Kinder mit dem Namen ${childName} und diesem Geburtsdatum angemeldet.`}{" "}
        Bitte wählen Sie.
      </p>
      <fieldset className="mt-4 space-y-2">
        <legend className="sr-only">Kind für diese Anmeldung</legend>
        {candidates.map((candidate) => (
          <ChoiceTile
            key={candidate.student_id}
            selected={selected === candidate.student_id}
            tone="green"
            className="items-start p-3"
          >
            <Radio
              name={name}
              value={candidate.student_id}
              checked={selected === candidate.student_id}
              onChange={() => setSelected(candidate.student_id)}
            />
            <span>
              <span className="block text-gray-900">
                Bestehendes Kind verwenden
              </span>
              <span className="block text-xs font-normal text-gray-600">
                {candidateDetails(candidate)}
              </span>
            </span>
          </ChoiceTile>
        ))}
        <ChoiceTile
          selected={selected === NEW_CHILD}
          tone="green"
          className="items-start p-3"
        >
          <Radio
            name={name}
            value={NEW_CHILD}
            checked={selected === NEW_CHILD}
            onChange={() => setSelected(NEW_CHILD)}
          />
          <span>
            <span className="block text-gray-900">Als neues Kind anlegen</span>
            <span className="block text-xs font-normal text-gray-600">
              Nur wählen, wenn es ein anderes Kind ist.
            </span>
          </span>
        </ChoiceTile>
      </fieldset>
      <p className="mt-3 text-xs text-gray-600">
        Beim bestehenden Kind wird nur die Anmeldung verlängert. Die Eltern aus
        dieser Anmeldung bekommen dadurch keinen Zugang zum Kind.
      </p>
    </ConfirmationModal>
  );
}

function candidateDetails(candidate: AdminExistingChildCandidate): string {
  return [
    `${candidate.first_name} ${candidate.last_name}`,
    candidate.school_class ? schoolClassLabel(candidate.school_class) : "",
    candidate.date_of_birth
      ? `geboren am ${formatDate(candidate.date_of_birth)}`
      : "",
  ]
    .filter(Boolean)
    .join(" · ");
}
