"use client";

import { ToggleChip } from "~/components/ui/toggle-chip";
import { cn } from "~/lib/utils";

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
  className,
}: Readonly<{
  /** Name der Gruppe für Screenreader, z. B. „Wochentage“. */
  ariaLabel: string;
  days: ReadonlyArray<WeekdayToggleOption<T>>;
  value: readonly T[];
  onToggle: (day: T) => void;
  disabled?: boolean;
  className?: string;
}>) {
  return (
    <div
      role="group"
      aria-label={ariaLabel}
      className={cn("flex w-full flex-wrap gap-1.5", className)}
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
    </div>
  );
}
