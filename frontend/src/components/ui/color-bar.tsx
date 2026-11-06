const SIZE_CLASS = {
  xs: "h-3 w-0.5",
  sm: "h-3 w-1",
  md: "h-3.5 w-1",
  lg: "h-9 w-1",
} as const;

/** Neutrale Kante für Einträge ohne Farbe (UNTYPED_EDGE_COLOR). */
export const COLOR_BAR_FALLBACK = "#D1D5DB";

/**
 * Senkrechter Farbbalken als Typkennzeichen einer Zeile, etwa die Schichtart.
 * Rein schmückend: der Text daneben nennt den Typ.
 */
export function ColorBar({
  color,
  size = "sm",
  className = "",
}: Readonly<{
  color?: string | null;
  size?: keyof typeof SIZE_CLASS;
  className?: string;
}>) {
  return (
    <span
      aria-hidden="true"
      className={`inline-block shrink-0 rounded-full ${SIZE_CLASS[size]} ${className}`}
      style={{ backgroundColor: color ? color : COLOR_BAR_FALLBACK }}
    />
  );
}
