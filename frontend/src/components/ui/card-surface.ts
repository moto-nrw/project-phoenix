import { cn } from "~/lib/utils";

// Die Fläche jeder Karte des Kits, in ihren Teilen: Rahmen, Innenabstand,
// Hover-Anhebung und Fokusring. `SectionCard`, `TileCard`, `StatCard`,
// die gerahmte `ListRowLink` und die Kind-Kachel setzen sie hieraus zusammen,
// statt die Klassenkette je selbst zu schreiben (#6581). Ändert sich eine
// Karte, ändern sich alle.

/** Weiße Fläche mit Rahmen und Kartenradius (24 px). */
export const CARD_FRAME = "moto-content-surface rounded-2xl border";

// Das eine Kartenpolster (ADR 0149, Entscheidung 9): 20 px, auf dem Telefon
// und in der kompakten Ansicht 16 px.
export const CARD_SURFACE_PADDING = "compact:p-4 p-5 max-sm:p-4";

/** Hebt eine Karte beim Überfahren an (`globals.css`). */
export const CARD_HOVER = "moto-hover-elevated";

/**
 * Fokusring einer klickbaren Karte, in moto-blue für 3:1 Kontrast (#6565).
 * Mit dem Kartenradius, damit der Ring der Rundung folgt, auch wenn er auf
 * einem Link um die Fläche liegt statt auf der Fläche selbst.
 */
export const CARD_FOCUS_RING =
  "rounded-2xl focus-visible:outline-2 focus-visible:outline-moto-blue focus-visible:outline-offset-2";

/**
 * Die Fläche als eine Klassenkette. `hover` und `focusRing` für die Karte, die
 * selbst das klickbare Element ist; liegt ein Link um die Fläche, trägt er
 * den Fokusring (`CARD_FOCUS_RING`).
 */
export function cardSurfaceClasses({
  padded = false,
  hover = false,
  focusRing = false,
}: Readonly<{
  padded?: boolean;
  hover?: boolean;
  focusRing?: boolean;
}> = {}): string {
  return cn(
    CARD_FRAME,
    hover && CARD_HOVER,
    focusRing && CARD_FOCUS_RING,
    padded && CARD_SURFACE_PADDING,
  );
}
