"use client";

import { useId } from "react";
import { RepeatIcon } from "@phosphor-icons/react/ssr";
import { SegmentedControl } from "~/components/ui/segmented-control";
import type { AnswerScope } from "~/lib/calendar-answers";

/**
 * Die Wahl, ob eine Zu- oder Absage nur für den geöffneten Termin einer Reihe
 * gilt oder für alle Termine (#6987). Darunter steht in einem Satz, was mit
 * den anderen Terminen passiert. Die Texte kommen vom Portal, weil das
 * Elternportal übersetzt ist.
 */
export function AnswerScopeChoice({
  value,
  onChange,
  legend,
  occurrenceLabel,
  seriesLabel,
  occurrenceHint,
  seriesHint,
}: Readonly<{
  value: AnswerScope;
  onChange: (next: AnswerScope) => void;
  /** „Ihre Antwort gilt für“ */
  legend: string;
  /** „Nur am Mi., 25.11.“ */
  occurrenceLabel: string;
  /** „Alle Termine“ */
  seriesLabel: string;
  /** Was die Antwort für nur diesen Termin mit den anderen Terminen macht. */
  occurrenceHint: string;
  /** Was die Antwort für alle Termine mit den anderen Terminen macht. */
  seriesHint: string;
}>) {
  const hintId = useId();
  return (
    <div className="space-y-2">
      <p className="text-xs leading-5 font-medium text-gray-600">{legend}</p>
      <SegmentedControl<AnswerScope>
        fullWidth
        ariaLabel={legend}
        ariaDescribedBy={hintId}
        value={value}
        onChange={onChange}
        items={[
          { value: "occurrence", label: occurrenceLabel },
          { value: "series", label: seriesLabel },
        ]}
      />
      <p
        id={hintId}
        aria-live="polite"
        className="flex items-start gap-1.5 text-xs leading-5 text-gray-500"
      >
        <RepeatIcon size={14} className="mt-0.5 shrink-0" aria-hidden="true" />
        {value === "occurrence" ? occurrenceHint : seriesHint}
      </p>
    </div>
  );
}
