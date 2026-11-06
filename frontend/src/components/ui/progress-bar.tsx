const TONE_CLASS = {
  green: "bg-moto-green",
  orange: "bg-moto-orange",
  gray: "bg-gray-400",
  dark: "bg-gray-900",
} as const;

const SIZE_CLASS = {
  xs: "h-1 bg-gray-100",
  sm: "h-1.5 bg-gray-100",
  md: "h-2 bg-gray-200",
} as const;

interface ProgressBarProps {
  /** Erledigte Teile. */
  readonly value: number;
  readonly max: number;
  /** Vorgelesen statt der Zahl, zum Beispiel „3 von 6 Schritten erledigt“. */
  readonly label: string;
  readonly tone?: keyof typeof TONE_CLASS;
  /** Farbe aus Daten als Hex; hat Vorrang vor `tone`. */
  readonly color?: string;
  readonly size?: keyof typeof SIZE_CLASS;
  readonly className?: string;
}

/**
 * Ein durchgehender Fortschrittsbalken, etwa für eine Checkliste. Für
 * nummerierte Schritte eines Ablaufs gibt es `WizardStepper`.
 */
export function ProgressBar({
  value,
  max,
  label,
  tone = "green",
  color,
  size = "md",
  className = "",
}: ProgressBarProps) {
  const bounded = Math.min(Math.max(value, 0), max);
  const percent = max > 0 ? (bounded / max) * 100 : 0;
  return (
    <div
      role="progressbar"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={max}
      aria-valuenow={bounded}
      className={`w-full overflow-hidden rounded-full ${SIZE_CLASS[size]} ${className}`}
    >
      <div
        className={`h-full rounded-full motion-safe:transition-[width] motion-safe:duration-300 ${color ? "" : TONE_CLASS[tone]}`}
        style={{
          width: `${percent}%`,
          ...(color ? { backgroundColor: color } : {}),
        }}
      />
    </div>
  );
}
