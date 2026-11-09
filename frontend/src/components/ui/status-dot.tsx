const SIZE_CLASS = {
  xs: "h-1 w-1",
  sm: "h-1.5 w-1.5",
  md: "h-2 w-2",
  lg: "h-2.5 w-2.5",
} as const;

type StatusDotProps = {
  /** Farbe aus Daten als Hex; eine feste Farbe kommt als `bg-…` über `className`. */
  readonly color?: string;
  readonly size?: keyof typeof SIZE_CLASS;
  readonly className?: string;
} & (
  | {
      /** Vorgelesen, wenn neben dem Punkt kein Text steht. */
      readonly label: string;
      readonly decorative?: never;
    }
  | {
      /** Sichtbarer Text daneben sagt schon dasselbe. */
      readonly decorative: true;
      readonly label?: never;
    }
);

/**
 * Farbpunkt für Legenden und Pläne, wo heute schon Punkte stehen (ADR 0149,
 * Entscheidung 5). Kein allgemeines Status-Mittel: für einen Status mit Text
 * gibt es `StatusBadge`; `ui-kit/no-status-dot-outside-plans` hält den Rahmen.
 * Der Punkt pulsiert nie: Bewegung ist kein Signal.
 */
export function StatusDot({
  color,
  size = "sm",
  label,
  decorative,
  className = "",
}: StatusDotProps) {
  return (
    <span
      aria-hidden={decorative ? true : undefined}
      className={`inline-block shrink-0 rounded-full ${SIZE_CLASS[size]} ${className}`}
      // `color` trägt zusätzlich den Ring einer Markierung (ring-* nimmt currentColor).
      style={color ? { backgroundColor: color, color } : undefined}
    >
      {label ? <span className="sr-only">{label}</span> : null}
    </span>
  );
}
