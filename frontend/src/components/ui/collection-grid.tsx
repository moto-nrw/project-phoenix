import type { ReactNode } from "react";
import { cn } from "~/lib/utils";

// Ganze Klassen-Literale, damit Tailwind sie beim Scannen findet.
const GRID_COLUMNS = {
  "16rem": "grid-cols-[repeat(auto-fit,minmax(min(100%,16rem),1fr))]",
  "18rem": "grid-cols-[repeat(auto-fit,minmax(min(100%,18rem),1fr))]",
  "20rem": "grid-cols-[repeat(auto-fit,minmax(min(100%,20rem),1fr))]",
} as const;

/**
 * DAS Kachelgitter der Sammlungsseiten (Kinder, Mitarbeiter, Räume,
 * Aktivitäten, Gruppen, Bereichskacheln). Vorher deklarierte jede Seite ihr
 * eigenes `grid grid-cols-1 gap-6 md:grid-cols-2 …` — zwölf Kopien mit je
 * anderen Spaltengrenzen, und `gap-6` galt auch unter sm, gegen den
 * 12-px-Mobilrhythmus des Gerüsts.
 *
 * Die Spalten kommen aus `auto-fit` mit einer Mindest-Kachelbreite statt aus
 * Breakpoints (dasselbe Muster wie im Eltern-Portal): kein Fensterausschnitt
 * quetscht die Kacheln, und eine einzelne Kachel füllt die Zeile, statt eine
 * halbe leer zu lassen. Der Abstand folgt dem Rhythmus der Seite: 16 px,
 * unter sm 12 px.
 */
export function CollectionGrid({
  minTileWidth = "20rem",
  as = "div",
  role,
  ariaLabel,
  ariaBusy,
  testId,
  className,
  children,
}: Readonly<{
  /**
   * Mindestbreite einer Kachel. `20rem` trägt die Personen-Kacheln;
   * schmale Kacheln (Bereichskarten) dürfen auf `18rem` herunter, kurze
   * Einträge mit wenigen Zeilen (Einsätze „Heute geplant“) auf `16rem`, damit
   * vier davon am Computer in einer Zeile stehen.
   */
  minTileWidth?: keyof typeof GRID_COLUMNS;
  /**
   * `output` für Ladegitter, die sich selbst ankündigen; `ul` für ein
   * Gitter aus Listeneinträgen (`li`-Kinder).
   */
  as?: "div" | "output" | "ul";
  role?: "status";
  ariaLabel?: string;
  ariaBusy?: boolean;
  testId?: string;
  className?: string;
  children: ReactNode;
}>) {
  const Element = as;
  return (
    <Element
      role={role}
      aria-label={ariaLabel}
      aria-busy={ariaBusy}
      data-testid={testId}
      className={cn(
        "grid gap-4 max-sm:gap-3",
        GRID_COLUMNS[minTileWidth],
        className,
      )}
    >
      {children}
    </Element>
  );
}

/**
 * Ein Feld im `CollectionGrid`, wenn die Einträge keine eigene Karte sind:
 * kurze Hinweise, Übergaben, Einwilligungen in einer Karte, die sonst bis
 * zur Unterkante leer bliebe (#4365). Leicht abgesetzt statt weiß gerahmt,
 * damit es nicht als Karte in der Karte liest. Nur Anzeige: anklickbare
 * Kacheln nehmen `TileCard`.
 */
export function CollectionGridTile({
  as = "li",
  className,
  children,
}: Readonly<{
  as?: "li" | "div";
  className?: string;
  children: ReactNode;
}>) {
  const Element = as;
  return (
    <Element
      className={cn(
        "rounded-xl border border-gray-100 bg-gray-50/70 p-4",
        className,
      )}
    >
      {children}
    </Element>
  );
}
