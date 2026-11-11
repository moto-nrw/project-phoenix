"use client";

// Die Debitorennummer der Familie (#6692): Der Träger merkt sie sich an der
// Familie. Bei einer neuen Ferienanmeldung derselben Familie steht sie als
// Vorschlag am Feld. Das Büro übernimmt sie mit einem Klick; bestätigt wird
// erst mit „Bestätigen“. Weicht die eingetragene Nummer ab, fragt der Dialog,
// welche Nummer gilt. moto überschreibt nie still.

import { History } from "lucide-react";
import { type Ref, useEffect, useId, useState } from "react";
import { Button } from "~/components/ui/button";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { InsetPanel } from "~/components/ui/inset-panel";
import { Input } from "~/components/ui/input";
import { ConfirmationModal } from "~/components/ui/modal";
import { Radio } from "~/components/ui/radio";
import {
  DEBTOR_NUMBER_MAX_LENGTH,
  type DebtorNumberFormat,
  NO_DEBTOR_NUMBER_FORMAT,
  debtorNumberFormatHint,
} from "~/lib/debtor-number-format";
import type {
  FamilyDebtorNumberChoice,
  FamilyDebtorNumberConflict,
} from "~/lib/traeger-contributions-api";

/**
 * How a debtor number reads everywhere it is shown: the UI font with figures
 * of equal width, so numbers line up in a column, a dialog and a sentence.
 */
export const DEBTOR_NUMBER_TEXT_CLASS =
  "font-medium text-gray-900 tabular-nums";

/**
 * The field of a debtor number (#6675). With the Träger's format the helper
 * text names it first, and phones open the number keyboard; the format is
 * checked on save, by the form and then by the backend.
 */
export function DebtorNumberInput({
  id,
  inputRef,
  value,
  onChange,
  format = NO_DEBTOR_NUMBER_FORMAT,
  hint,
  required,
  error,
  describedBy,
}: Readonly<{
  id: string;
  inputRef?: Ref<HTMLInputElement>;
  value: string;
  onChange: (value: string) => void;
  format?: DebtorNumberFormat;
  /** What the number is for, after the format. */
  hint: string;
  required?: boolean;
  error?: string;
  /** Further descriptions, such as the family's known number. */
  describedBy?: string;
}>) {
  const formatHint = debtorNumberFormatHint(format);
  return (
    <Input
      ref={inputRef}
      id={id}
      name="debtor_number"
      label="Debitorennummer"
      required={required}
      autoComplete="off"
      controlSize="compact"
      inputMode={format.digitsOnly ? "numeric" : undefined}
      maxLength={DEBTOR_NUMBER_MAX_LENGTH}
      value={value}
      hint={formatHint ? `${formatHint} ${hint}` : hint}
      aria-describedby={describedBy}
      error={error}
      onChange={(event) => onChange(event.target.value)}
    />
  );
}

/**
 * The number the family already has, below the field (#6692): one calm row
 * with the number and „Übernehmen“. Hidden while the field already holds it.
 * „Übernehmen“ fills the field and confirms nothing.
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
    <InsetPanel
      padding="compact"
      className="mt-3 flex items-center gap-3 text-sm"
    >
      <History aria-hidden="true" className="h-4 w-4 shrink-0 text-gray-500" />
      <p id={id} className="min-w-0 flex-1 text-gray-700">
        Bekannte Nummer dieser Familie:{" "}
        <span className={`whitespace-nowrap ${DEBTOR_NUMBER_TEXT_CLASS}`}>
          {familyNumber}
        </span>
        {". Bitte prüfen."}
      </p>
      <Button
        type="button"
        variant="surface"
        size="compact"
        onClick={() => onAdopt(familyNumber)}
        aria-label={`Debitorennummer ${familyNumber} übernehmen`}
        className="shrink-0"
      >
        Übernehmen
      </Button>
    </InsetPanel>
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
          {conflict?.familyEmail ? (
            <>
              Für die Familie{" "}
              <span className="font-medium break-all text-gray-900">
                {conflict.familyEmail}
              </span>{" "}
              ist schon eine andere Nummer gemerkt. Bitte wählen Sie.
            </>
          ) : (
            "Für diese Familie ist schon eine andere Nummer gemerkt. Bitte wählen Sie."
          )}
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
                <span className="mt-1 block text-lg font-semibold break-all text-gray-900 tabular-nums">
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
