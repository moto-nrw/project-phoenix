"use client";

import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";

/**
 * Die eine Kennzahl-Kachel des Anmeldungs-Moduls. Vorher gab es drei Stile
 * nebeneinander (Phasenübersicht, Phasendetail, Auswertung); jede Kachel im
 * Modul nutzt jetzt diese Komponente, damit Radius, Fläche und Typografie
 * überall gleich sind. Das Symbol ist optional, weil die dichten
 * Vierer-Raster ohne Symbol auskommen.
 */
export function EnrollmentStatTile({
  label,
  value,
  icon: Icon,
  leading,
}: Readonly<{
  label: string;
  value: ReactNode;
  icon?: LucideIcon;
  /** Fertiges Symbol für Icon-Systeme ohne Lucide (MotoConceptIcon). */
  leading?: ReactNode;
}>) {
  return (
    <div className="moto-content-surface rounded-2xl border px-4 py-3 shadow-sm">
      <div className="flex items-center gap-3">
        {Icon || leading ? (
          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-gray-50 text-gray-500 shadow-sm">
            {Icon ? <Icon className="h-4 w-4" aria-hidden="true" /> : leading}
          </span>
        ) : null}
        <span className="min-w-0">
          <span className="block text-lg leading-none font-semibold text-gray-900">
            {value}
          </span>
          <span className="mt-1 block truncate text-xs font-medium text-gray-500">
            {label}
          </span>
        </span>
      </div>
    </div>
  );
}
