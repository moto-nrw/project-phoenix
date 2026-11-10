"use client";

import { useId } from "react";

import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { ToggleChip } from "~/components/ui/toggle-chip";

export interface WeekdayToggleOption<T extends string | number> {
  readonly value: T;
  /** Sichtbares Kürzel: „Mo“. */
  readonly label: string;
  /** Eigener vorgelesener Name, wenn das Kürzel nicht reicht. */
  readonly ariaLabel?: string;
}

/**
 * Die eine Wochentagsauswahl: jeder Tag ist für sich an oder aus. Eine Reihe
 * passt auch mit sieben Tagen auf ein Telefon.
 *
 * Der Baustein kennt keinen Kalender. Welche Tage es gibt und in welchem Format
 * ihr Wert steckt, sagt die Stelle, die ihn nutzt.
 */
export function WeekdayToggleGroup<T extends string | number>({
  ariaLabel,
  days,
  value,
  onToggle,
  disabled = false,
  hint,
  name,
  error,
}: Readonly<{
  /** Name der Gruppe für Screenreader, z. B. „Wochentage“. */
  ariaLabel: string;
  days: ReadonlyArray<WeekdayToggleOption<T>>;
  value: readonly T[];
  onToggle: (day: T) => void;
  disabled?: boolean;
  /**
   * Eine Zeile unter den Tagen, die stehen bleibt und mit der Gruppe
   * vorgelesen wird. Aussehen und Platz wie der `hint` am Kit-`Input`.
   */
  hint?: string;
  name?: string;
  error?: string;
}>) {
  const hintId = `${useId()}-hint`;
  const errorId = `${useId()}-error`;
  const describedBy =
    [hint ? hintId : null, error ? errorId : null].filter(Boolean).join(" ") ||
    undefined;
  const group = (
    <fieldset
      aria-label={ariaLabel}
      aria-invalid={error ? true : undefined}
      aria-describedby={describedBy}
      className="flex w-full min-w-0 flex-wrap gap-1.5"
    >
      {days.map((day) => (
        <ToggleChip
          key={day.value}
          pressed={value.includes(day.value)}
          onPressedChange={() => onToggle(day.value)}
          disabled={disabled}
          ariaLabel={day.ariaLabel}
          className="max-w-14 min-w-10 flex-1 px-1"
        >
          {day.label}
        </ToggleChip>
      ))}
    </fieldset>
  );

  if (!hint && !error && !name) return group;

  return (
    <div>
      {name ? (
        <input type="hidden" name={name} value={value.join(",")} />
      ) : null}
      {group}
      {hint ? (
        <p id={hintId} className="mt-1 text-xs text-gray-600">
          {hint}
        </p>
      ) : null}
      <FieldErrorHint id={errorId} message={error} />
    </div>
  );
}
