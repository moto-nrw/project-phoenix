import type { ReactNode } from "react";
import { cn } from "~/lib/utils";

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
   * schmale Kacheln (Bereichskarten) dürfen auf `18rem` herunter.
   */
  minTileWidth?: "18rem" | "20rem";
  /** `output` für Ladegitter, die sich selbst ankündigen. */
  as?: "div" | "output";
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
        minTileWidth === "18rem"
          ? "grid-cols-[repeat(auto-fit,minmax(min(100%,18rem),1fr))]"
          : "grid-cols-[repeat(auto-fit,minmax(min(100%,20rem),1fr))]",
        className,
      )}
    >
      {children}
    </Element>
  );
}
