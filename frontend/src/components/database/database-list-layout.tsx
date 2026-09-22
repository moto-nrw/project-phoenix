"use client";

import type { ReactNode } from "react";
import { useIsMobile } from "~/components/ui/hooks/useIsMobile";
import { cn } from "~/lib/utils";
import { useFillHeight } from "./use-fill-height";

interface DatabaseListLayoutProps {
  children: ReactNode;
  className?: string;
  /** Luft unter der Liste, damit sie nicht am Bildschirmrand klebt. */
  bottomOffset?: number;
}

/**
 * Die einspaltige Sammlung der Datenverwaltung (BAUARTEN-SPEC Bauart 1,
 * Regel 2): eine Liste, deren Zeilen auf die Objektroute führen. Kein Pane
 * daneben, weil der Objekttyp seine Ansicht unter einer eigenen Route hat
 * (Kind, Mitarbeitende, Raum, #3115).
 *
 * Dieselbe Karte und dieselbe Höhe wie das Master-Detail-Layout ohne
 * Auswahl, damit ein Register beim Wechsel der Bauart nicht springt.
 */
export function DatabaseListLayout({
  children,
  className,
  bottomOffset = 32,
}: DatabaseListLayoutProps) {
  const isMobile = useIsMobile();
  const { ref, height } = useFillHeight<HTMLDivElement>(bottomOffset);

  // Am Computer setzt `useFillHeight` die Höhe und die Liste scrollt in der
  // Karte; `moto-scroll-surface` nimmt sie aus der Wachstumsregel, sonst
  // wüchse sie über die feste Hülle hinaus so hoch wie alle Zeilen. Auf dem
  // Telefon scrollt die Seite: keine feste Höhe, die Karte wächst mit der
  // Liste, wie im Master-Detail-Layout (#3330).
  return (
    <div
      ref={ref}
      style={isMobile ? undefined : { height }}
      className={cn("flex w-full flex-col", className)}
      data-testid="database-list-layout"
    >
      <div
        className={cn(
          "moto-content-surface min-h-0 flex-1 overflow-hidden rounded-2xl border shadow-sm",
          !isMobile && "moto-scroll-surface",
        )}
      >
        <div className="flex h-full flex-col">{children}</div>
      </div>
    </div>
  );
}
