"use client";

// Die Debitorennummer der Familie (#6692): Der Träger merkt sie sich an der
// Familie. Bei einer neuen Ferienanmeldung derselben Familie steht sie als
// Vorschlag am Feld. Das Büro übernimmt sie mit einem Klick; bestätigt wird
// erst mit „Bestätigen“. Weicht die eingetragene Nummer ab, fragt der Dialog,
// welche Nummer gilt. moto überschreibt nie still.

import { useEffect, useId, useState } from "react";
import { Button } from "~/components/ui/button";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { ConfirmationModal } from "~/components/ui/modal";
import { Radio } from "~/components/ui/radio";
import type {
  FamilyDebtorNumberChoice,
  FamilyDebtorNumberConflict,
} from "~/lib/traeger-contributions-api";

/**
 * The number the family already has, below the field (#6692). Hidden while
 * the field already holds it. „Übernehmen“ fills the field and confirms
 * nothing.
 */
export function FamilyDebtorNumberHint({
  id,
  familyNumber,
  value,
  onAdopt,
}: Readonly<{
  /** Referenced by the field's aria-describedby. */
  id: string;
  familyNumber: string;
  value: string;
  onAdopt: (number: string) => void;
}>) {
  if (!familyNumber || value.trim() === familyNumber) return null;
  return (
    <div className="mt-2 flex items-center justify-between gap-3 text-sm text-gray-700">
      <p id={id} className="min-w-0 flex-1">
        Bekannte Debitorennummer dieser Familie:{" "}
        <span className="font-mono font-semibold whitespace-nowrap text-gray-900 tabular-nums">
          {familyNumber}
        </span>
        {". Bitte prüfen."}
      </p>
      <Button
        type="button"
        variant="outline"
        size="compact"
        onClick={() => onAdopt(familyNumber)}
        aria-label={`Debitorennummer ${familyNumber} übernehmen`}
        className="shrink-0"
      >
        Übernehmen
      </Button>
    </div>
  );
}

/**
 * Asks which number applies when the number entered differs from the one the
 * family has (#6692). The action stays disabled until a choice is made.
 */
export function FamilyDebtorNumberConflictDialog({
  conflict,
  confirmText,
  loadingText,
  running,
  onChoose,
  onClose,
}: Readonly<{
  /** Both numbers; null keeps the dialog closed. */
  conflict: FamilyDebtorNumberConflict | null;
  /** „Bestätigen“ at the confirmation, „Speichern“ at a change. */
  confirmText: string;
  loadingText: string;
  running: boolean;
  onChoose: (choice: FamilyDebtorNumberChoice) => void;
  onClose: () => void;
}>) {
  const [choice, setChoice] = useState<FamilyDebtorNumberChoice | null>(null);
  const name = useId();
  useEffect(() => {
    if (conflict) setChoice(null);
  }, [conflict]);

  const options: {
    value: FamilyDebtorNumberChoice;
    label: string;
    number: string;
    description: string;
  }[] = conflict
    ? [
        {
          value: "replace",
          label: "Neue Nummer für die Familie merken",
          number: conflict.enteredNumber,
          description: `Diese und spätere Anmeldungen der Familie bekommen ${conflict.enteredNumber}.`,
        },
        {
          value: "keep",
          label: "Gemerkte Nummer behalten",
          number: conflict.familyNumber,
          description: `Nur diese Anmeldung bekommt ${conflict.enteredNumber}. Die Familie behält ${conflict.familyNumber}.`,
        },
      ]
    : [];

  return (
    <ConfirmationModal
      isOpen={conflict !== null}
      title="Welche Debitorennummer gilt?"
      confirmText={confirmText}
      loadingText={loadingText}
      isConfirmLoading={running}
      isConfirmDisabled={choice === null}
      onClose={onClose}
      onConfirm={() => {
        if (choice) onChoose(choice);
      }}
    >
      <fieldset className="space-y-3">
        <legend className="mb-3 text-sm text-gray-700">
          Für diese Familie ist schon eine andere Nummer gemerkt. Bitte wählen
          Sie.
        </legend>
        {options.map((option) => {
          const id = `${name}-${option.value}`;
          return (
            <ChoiceTile
              key={option.value}
              htmlFor={id}
              selected={choice === option.value}
              className="items-start p-4"
            >
              <Radio
                id={id}
                name={name}
                value={option.value}
                checked={choice === option.value}
                onChange={() => setChoice(option.value)}
                aria-describedby={`${id}-description`}
                className="mt-0.5"
              />
              <span className="min-w-0 flex-1">
                <span className="block text-sm font-medium text-gray-900">
                  {option.label}
                </span>
                <span className="mt-1 block font-mono text-lg font-semibold break-all text-gray-900 tabular-nums">
                  {option.number}
                </span>
                <span
                  id={`${id}-description`}
                  className="mt-1 block text-xs font-normal text-gray-600"
                >
                  {option.description}
                </span>
              </span>
            </ChoiceTile>
          );
        })}
      </fieldset>
    </ConfirmationModal>
  );
}
