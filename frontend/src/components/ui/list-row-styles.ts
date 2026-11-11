import { cn } from "~/lib/utils";

export type ListRowTone =
  "neutral" | "success" | "danger" | "muted" | "surface";
export type ListRowDensity = "default" | "compact";

const TONE_CLASSES: Record<ListRowTone, string> = {
  neutral: "bg-gray-50/50",
  success: "bg-moto-green/10",
  danger: "bg-moto-red/5",
  muted: "bg-gray-100",
  // Für Zeilen auf einer grauen Innenfläche, auf der `neutral` verschwände.
  surface: "bg-white shadow-sm",
};

const TONE_HOVER_CLASSES: Record<ListRowTone, string> = {
  neutral: "hover:bg-gray-100/50",
  success: "hover:bg-moto-green/15",
  danger: "hover:bg-moto-red/10",
  muted: "hover:bg-gray-200/60",
  surface: "hover:bg-gray-50",
};

// Dieselbe Hover-Farbe, wenn die Zeile in einem Link (`group`) liegt und
// dessen Überfahren sie färbt.
const TONE_GROUP_HOVER_CLASSES: Record<ListRowTone, string> = {
  neutral: "group-hover:bg-gray-100/50",
  success: "group-hover:bg-moto-green/15",
  danger: "group-hover:bg-moto-red/10",
  muted: "group-hover:bg-gray-200/60",
  surface: "group-hover:bg-gray-50",
};

const DENSITY_CLASSES: Record<ListRowDensity, string> = {
  default: "px-3 py-2",
  // Für Karten fester Höhe: ab `sm` passen so drei Zeilen ganz hinein.
  compact: "px-3 py-2 sm:py-1.5",
};

/** Fläche einer Listenzeile, geteilt von `ListRow` und `ListRowSkeleton`. */
export function listRowClasses({
  tone = "neutral",
  density = "default",
  interactive = false,
  hoverFromGroup = false,
}: Readonly<{
  tone?: ListRowTone;
  density?: ListRowDensity;
  interactive?: boolean;
  /** Färbt beim Überfahren des umschließenden Links (`group`). */
  hoverFromGroup?: boolean;
}> = {}): string {
  return cn(
    "flex items-center justify-between gap-3 rounded-xl",
    DENSITY_CLASSES[density],
    TONE_CLASSES[tone],
    interactive &&
      `w-full text-left transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none focus-visible:ring-inset ${TONE_HOVER_CLASSES[tone]}`,
    hoverFromGroup && `transition-colors ${TONE_GROUP_HOVER_CLASSES[tone]}`,
  );
}

/**
 * Hinterlegung einer `ListRowLink`-Zeile in einer Liste: die offene Auswahl
 * grün (wie in der Datenverwaltung), sonst nur beim Überfahren grau.
 */
export function listRowLinkStateClasses(selected: boolean): string {
  return selected
    ? "bg-moto-green-soft/60 hover:bg-moto-green-soft/70 active:bg-moto-green-soft/70"
    : "hover:bg-gray-50 active:bg-gray-100";
}
