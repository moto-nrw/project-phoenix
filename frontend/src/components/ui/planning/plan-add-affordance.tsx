"use client";

import { Plus } from "lucide-react";

/**
 * PlanAddAffordance ist DIE Anlege-Geste aller Planungsraster (#2031).
 * Betreuungsplan (Stundenraster), Dienstplan (gefüllte Zellen) und jede leere
 * ResourceGrid-Zelle rendern exakt dieses Bild, damit die Geste überall gleich
 * aussieht — leer wie gefüllt. Vorher hatte jedes Raster seine eigene Variante
 * (Textpille "+ Termin", gestrichelter Plus-Button, flächiger Plus-Button).
 *
 * Es ist ein <span>, kein <button>: jedes Raster besitzt bereits sein eigenes
 * interaktives Element (der Leerzellen-Button der ResourceGrid, der
 * Hinzufügen-Button in der gefüllten Dienstplan-Zelle, der absolut
 * positionierte Stunden-Slot im Wochenraster), und ein Button im Button ist
 * ungültiges HTML. Das interaktive Element des Aufrufers MUSS die Klasse
 * `group` tragen, weil die Einblendung daran hängt.
 *
 * Sichtbarkeit: auf Zeigegeräten mit echtem Hover erscheint die Fläche erst
 * bei Hover oder Tastaturfokus, damit ein dichtes Raster nicht zum Plus-Teppich
 * wird. Auf Touch (kein Hover) ist sie dauerhaft sichtbar, weil es dort keinen
 * Hover-Zustand gibt, über den man sie sonst entdecken könnte.
 */

type PlanAddAffordanceSize = "fill" | "inline";

const SIZE_CLASS: Record<PlanAddAffordanceSize, string> = {
  /**
   * Füllt den Platz, den der Aufrufer vorgibt (leere Rasterzelle,
   * Stunden-Slot). BEWUSST ohne eigene Mindesthöhe: die Zeilenhöhe des
   * ResourceGrid kommt aus dessen Zellen-Wrapper (Issue #2026), eine zweite
   * Mindesthöhe hier würde leere Zeilen wieder höher machen als gefüllte.
   */
  // self-stretch statt h-full: die Fläche ist ein Flex-Kind des Aufrufers, und
  // eine Prozenthöhe greift dort nicht zuverlässig (in der Rasterzelle blieb
  // sie 18px hoch statt 64px). Über die Querachse zu strecken ist der Weg, der
  // in beiden Aufrufern funktioniert — auch im Stunden-Slot mit items-center.
  fill: "self-stretch",
  /** Schmale Zeile unter vorhandenem Zelleninhalt. */
  inline: "h-7",
};

// Der Hover-Zweig hängt komplett an `(hover: hover) and (pointer: fine)`:
// Standard ist sichtbar, ausgeblendet wird nur dort, wo es Hover wirklich gibt.
const REVEAL_CLASS = [
  "opacity-100",
  "[@media(hover:hover)_and_(pointer:fine)]:opacity-0",
  "[@media(hover:hover)_and_(pointer:fine)]:group-hover:opacity-100",
  "[@media(hover:hover)_and_(pointer:fine)]:group-focus-visible:opacity-100",
].join(" ");

interface PlanAddAffordanceProps {
  readonly size?: PlanAddAffordanceSize;
  readonly className?: string;
}

export function PlanAddAffordance({
  size = "fill",
  className,
}: PlanAddAffordanceProps) {
  return (
    <span
      aria-hidden="true"
      className={[
        // pointer-events-none: das umschließende Element des Aufrufers trägt
        // den Klick, die Fläche darf ihn nie abfangen.
        "pointer-events-none flex w-full items-center justify-center rounded-md border border-dashed border-gray-200 text-gray-400 transition group-hover:border-gray-300 group-hover:text-gray-600",
        SIZE_CLASS[size],
        REVEAL_CLASS,
        className ?? "",
      ]
        .filter(Boolean)
        .join(" ")}
    >
      <Plus className="h-4 w-4" />
    </span>
  );
}
