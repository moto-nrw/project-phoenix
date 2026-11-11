"use client";

// Die Debitorennummer der Familie (#6692): Der Träger merkt sie sich an der
// Familie. Bei einer neuen Ferienanmeldung derselben Familie steht sie als
// Vorschlag am Feld. Das Büro übernimmt sie mit einem Klick; bestätigt wird
// erst mit „Bestätigen“. Weicht die eingetragene Nummer ab, fragt der Dialog,
// welche Nummer gilt. moto überschreibt nie still.

import { History } from "lucide-react";
import { Button } from "~/components/ui/button";
import { ChoiceModal } from "~/components/ui/choice-modal";
import { InsetPanel } from "~/components/ui/inset-panel";
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
      className="mt-3 flex flex-wrap items-start gap-3 text-sm sm:flex-nowrap sm:items-center"
    >
      <History aria-hidden="true" className="h-4 w-4 shrink-0 text-gray-500" />
      <p
        id={id}
        className="min-w-0 flex-1 basis-[calc(100%-2rem)] text-gray-700 sm:basis-0"
      >
        Bekannte Nummer dieser Familie:{" "}
        <span className={`wrap-anywhere ${DEBTOR_NUMBER_TEXT_CLASS}`}>
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
        className="ml-7 shrink-0 sm:ml-0"
      >
        Übernehmen
      </Button>
    </InsetPanel>
  );
}

/** Asks which number applies, naming every affected family before saving. */
export function FamilyDebtorNumberConflictDialog({
  conflict,
  running,
  onChoose,
  onClose,
}: Readonly<{
  conflict: FamilyDebtorNumberConflict | null;
  running: boolean;
  onChoose: (choice: FamilyDebtorNumberChoice) => void;
  onClose: () => void;
}>) {
  const multiple = (conflict?.families.length ?? 0) > 1;
  const description = multiple
    ? `Für diese Familien sind andere Nummern gemerkt: ${conflict?.families.map((family) => `${family.familyEmail}: ${family.familyNumber}`).join("; ")}. Bitte wählen Sie für alle genannten Familien.`
    : conflict?.familyEmail
      ? `Für die Familie ${conflict.familyEmail} ist schon eine andere Nummer gemerkt. Bitte wählen Sie.`
      : "Für diese Familie ist schon eine andere Nummer gemerkt. Bitte wählen Sie.";
  return (
    <ChoiceModal
      isOpen={conflict !== null}
      title="Welche Debitorennummer gilt?"
      description={description}
      isBusy={running}
      onClose={onClose}
      onSelect={(choice) => {
        if (choice === "keep" || choice === "replace") onChoose(choice);
      }}
      options={
        conflict
          ? [
              {
                value: "replace",
                label: multiple
                  ? "Neue Nummer für alle genannten Familien merken"
                  : "Neue Nummer für die Familie merken",
                description: multiple
                  ? `Diese Anmeldungen und spätere Anmeldungen aller genannten Familien bekommen ${conflict.enteredNumber}.`
                  : `Diese und spätere Anmeldungen der Familie bekommen ${conflict.enteredNumber}.`,
              },
              {
                value: "keep",
                label: multiple
                  ? "Gemerkte Nummern behalten"
                  : "Gemerkte Nummer behalten",
                description: multiple
                  ? `Diese Anmeldungen bekommen ${conflict.enteredNumber}. Jede genannte Familie behält ihre gemerkte Nummer.`
                  : `Nur diese Anmeldung bekommt ${conflict.enteredNumber}. Die Familie behält ${conflict.familyNumber}.`,
              },
            ]
          : []
      }
    />
  );
}
