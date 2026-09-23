"use client";

import { ChoiceModal } from "~/components/ui/choice-modal";

/**
 * Frage vor dem Speichern einer Serie, die Schließtage trifft (#3594). Ohne
 * Zustimmung plant moto an Schließtagen keine Termine dieser Serie; mit
 * „Auch an Schließtagen planen“ läuft sie dort weiter, etwa als
 * Ferienbetreuung. Feiertage bleiben immer frei.
 */
export function SeriesClosingDaysChoiceModal({
  closingDayCount,
  onCancel,
  onChoose,
  isBusy = false,
}: {
  /** Wie viele Termine der Serie auf Schließtage fallen. */
  readonly closingDayCount: number;
  readonly onCancel: () => void;
  /** `true` = auch an Schließtagen planen. */
  readonly onChoose: (includeClosingDays: boolean) => void;
  readonly isBusy?: boolean;
}) {
  const days =
    closingDayCount === 1 ? "1 Schließtag" : `${closingDayCount} Schließtage`;
  return (
    <ChoiceModal
      isOpen
      onClose={onCancel}
      title="Schließtage in dieser Serie"
      description={`Diese Serie trifft ${days}. Normalerweise werden sie ausgelassen.`}
      options={[
        {
          value: "skip",
          label: "Schließtage auslassen",
          description: "An Schließtagen gibt es keinen Termin.",
        },
        {
          value: "include",
          label: "Auch an Schließtagen planen",
          description: "Zum Beispiel für die Ferienbetreuung.",
        },
      ]}
      onSelect={(value) => onChoose(value === "include")}
      isBusy={isBusy}
    />
  );
}
