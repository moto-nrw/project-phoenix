"use client";

import { FieldShell, useFieldIds } from "~/components/ui/field-shell";
import { ToggleChip } from "~/components/ui/toggle-chip";
import { cn } from "~/lib/utils";

export interface WeekdayToggleOption<T extends string | number> {
  readonly value: T;
  /** Sichtbares Kürzel: „Mo“. */
  readonly label: string;
  /** Eigener vorgelesener Name, wenn das Kürzel nicht reicht. */
  readonly ariaLabel?: string;
  /**
   * Nur dieser Tag ist nicht wählbar, zum Beispiel ein Feiertag. Er bleibt
   * sichtbar und durchgestrichen, damit die Reihe nicht springt. Den Grund
   * nennen `ariaLabel` und der `hint` der Gruppe.
   */
  readonly disabled?: boolean;
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
  const ids = useFieldIds({ hint, error });
  const group = (
    <fieldset
      aria-label={ariaLabel}
      aria-invalid={error ? true : undefined}
      aria-describedby={ids.describedBy}
      className="flex w-full min-w-0 flex-wrap gap-1.5"
    >
      {days.map((day) => (
        <ToggleChip
          key={day.value}
          pressed={value.includes(day.value)}
          onPressedChange={() => onToggle(day.value)}
          disabled={disabled || day.disabled}
          ariaLabel={day.ariaLabel}
          className={cn(
            "max-w-14 min-w-10 flex-1 px-1",
            day.disabled && "bg-gray-50 line-through",
          )}
        >
          {day.label}
        </ToggleChip>
      ))}
    </fieldset>
  );

  // Die Gruppe trägt ihren Namen selbst (`aria-label`); eine Beschriftung
  // darüber hat sie nicht.
  return (
    <FieldShell ids={ids} keepWrapper={Boolean(name)} hint={hint} error={error}>
      {name ? (
        <input type="hidden" name={name} value={value.join(",")} />
      ) : null}
      {group}
    </FieldShell>
  );
}
