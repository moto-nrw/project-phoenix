import { LOCATION_COLORS } from "~/lib/location-helper";

interface ProgressBarProps {
  /** Erledigte Teile. */
  readonly value: number;
  readonly max: number;
  /** Vorgelesen statt der Zahl, zum Beispiel „3 von 6 Schritten erledigt“. */
  readonly label: string;
  readonly className?: string;
}

/**
 * Ein durchgehender Fortschrittsbalken in Markengrün, etwa für eine
 * Checkliste. Für nummerierte Schritte eines Ablaufs gibt es `WizardStepper`.
 */
export function ProgressBar({
  value,
  max,
  label,
  className = "",
}: ProgressBarProps) {
  const percent = max > 0 ? Math.min(Math.max(value / max, 0), 1) * 100 : 0;
  return (
    <div
      role="progressbar"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={max}
      aria-valuenow={value}
      className={`h-2 w-full overflow-hidden rounded-full bg-gray-200 ${className}`}
    >
      <div
        className="h-full rounded-full motion-safe:transition-[width] motion-safe:duration-300"
        style={{
          width: `${percent}%`,
          backgroundColor: LOCATION_COLORS.GROUP_ROOM,
        }}
      />
    </div>
  );
}
