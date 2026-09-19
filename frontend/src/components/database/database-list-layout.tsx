"use client";

import type { ReactNode } from "react";
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
  const { ref, height } = useFillHeight<HTMLDivElement>(bottomOffset);

  return (
    <div
      ref={ref}
      style={{ height }}
      className={cn("flex w-full flex-col", className)}
      data-testid="database-list-layout"
    >
      <div className="moto-content-surface min-h-0 flex-1 overflow-hidden rounded-2xl border shadow-sm">
        <div className="flex h-full flex-col">{children}</div>
      </div>
    </div>
  );
}
