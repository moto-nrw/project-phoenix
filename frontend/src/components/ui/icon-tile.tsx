import type { HTMLAttributes, ReactNode } from "react";

import { cn } from "~/lib/utils";

export type IconTileSize = "xs" | "sm" | "section" | "page" | "display";
export type IconTileShape = "square" | "round";
export type IconTileTone =
  "neutral" | "inverse" | "blue" | "green" | "greenFill" | "orange" | "red";

// Kachelmaß, Ecke der quadratischen Form und Symbolgröße je Stufe. Das
// Symbol wird über die Kachel gesetzt (`[&>svg]`), damit eine Stufe überall
// dieselbe Symbolgröße zeigt, egal welche Größe der Aufrufer dem Symbol gibt.
const SIZES: Record<
  IconTileSize,
  { readonly box: string; readonly square: string; readonly icon: string }
> = {
  // 32 px: Zeilen in dichten Listen und Formular-Editoren.
  xs: { box: "h-8 w-8", square: "rounded-lg", icon: "[&>svg]:size-4" },
  // 36 px: Köpfe kleiner Blöcke innerhalb einer Karte.
  sm: { box: "h-9 w-9", square: "rounded-xl", icon: "[&>svg]:size-5" },
  // 36 bis 40 px: Kartenköpfe (SectionCard, InfoCard, StatCard).
  section: {
    box: "h-9 w-9 sm:h-10 sm:w-10",
    square: "rounded-xl",
    icon: "[&>svg]:size-5.5",
  },
  // 44 bis 48 px: Seitenköpfe, leere Zustände, Bestätigungen.
  page: {
    box: "h-11 w-11 sm:h-12 sm:w-12",
    square: "rounded-xl",
    icon: "[&>svg]:size-6.5",
  },
  // 56 px: Info-Displays und große Statusanzeigen.
  display: { box: "h-14 w-14", square: "rounded-xl", icon: "[&>svg]:size-7.5" },
};

// Ein Grauton für die neutrale Kachel (#5582). Getönte Kacheln behalten die
// Farbe ihrer Aussage; Farben nur über moto-Tokens.
const TONES: Record<IconTileTone, string> = {
  neutral: "bg-gray-100 text-gray-600",
  inverse: "bg-gray-900 text-white",
  blue: "bg-moto-blue-soft text-moto-blue-strong",
  green: "bg-moto-green/15 text-moto-green-strong",
  greenFill: "bg-moto-green-fill text-white",
  orange: "bg-moto-orange/10 text-moto-orange-strong",
  red: "bg-moto-red/10 text-moto-red",
};

interface IconTileProps extends Omit<
  HTMLAttributes<HTMLSpanElement>,
  "children" | "className"
> {
  /** Das Symbol, meist ein Phosphor-Icon oder `MotoDuotoneIcon`. */
  readonly icon: ReactNode;
  readonly size?: IconTileSize;
  readonly shape?: IconTileShape;
  readonly tone?: IconTileTone;
  /**
   * Nur Lage und Abstand (`mx-auto`, `mb-4`, `mt-0.5`), nie Maß oder Farbe.
   * Ausnahme: `print:`-Maße für gedruckte Seiten (NFC-Anleitung).
   */
  readonly className?: string;
}

/**
 * Die eine Icon-Kachel des Kits: ein Symbol mittig auf einer grauen oder
 * getönten Fläche. Für ein fachliches Konzept `ConceptIconTile` nehmen.
 * Handgebaute Kacheln sperrt `ui-kit/no-hand-rolled-icon-tile`.
 */
export function IconTile({
  icon,
  size = "section",
  shape = "square",
  tone = "neutral",
  className,
  ...rest
}: IconTileProps) {
  const { box, square, icon: iconSize } = SIZES[size];

  return (
    <span
      {...rest}
      className={cn(
        "flex shrink-0 items-center justify-center",
        box,
        shape === "round" ? "rounded-full" : square,
        TONES[tone],
        iconSize,
        className,
      )}
    >
      {icon}
    </span>
  );
}
