"use client";

/**
 * Lückenzähler plus Sprungliste des Betreuungsplan-Kopfzeilen-Kontexts
 * (docs/planung-redesign/docs/06-betreuungsplan.md Abschnitt 5.2). Der Chip
 * zeigt die Zahl der offenen Personal-Lücken; ein Klick öffnet eine schmale
 * Sprungliste. Jede Zeile trägt Startzeit, Titel und das Soll/Ist-Paar; ein
 * Klick springt zum Block (der Aufrufer setzt `d` und `block`).
 *
 * Reine Präsentation: keine Datenabfrage, kein URL-Zugriff. Die offenen Lücken
 * kommen als Prop; das Springen ist ein Callback.
 */

import { useCallback, useMemo, useRef, useState } from "react";

import { Button } from "~/components/ui/button";
import { CoverageIndicator } from "~/components/ui/coverage-indicator";
import { useClickOutside } from "~/lib/hooks/use-click-outside";
import type { GapInstance } from "~/lib/timetable-types";

import { formatDate } from "~/lib/date-helpers";
import { timetablePopoverSurface } from "./timetable-style";

interface GapJumpListProps {
  /** Offene (nicht quittierte) Lücken des sichtbaren Zeitraums. */
  gaps: GapInstance[];
  /** Springt zum Block der Lücke — der Aufrufer setzt `d` und `block`. */
  onJump: (gap: GapInstance) => void;
  /**
   * Solange die Lücken-Abfrage läuft, zeigt der Chip einen neutralen
   * Prüf-Hinweis statt der Entwarnung "Keine Lücken" — ein leeres Array vor
   * Datenankunft ist keine bestätigte Aussage. Den Fehlerfall blendet der
   * Aufrufer aus (der Fehler wird dort getoastet).
   */
  state?: "ready" | "loading";
}

export function GapJumpList({
  gaps,
  onJump,
  state = "ready",
}: GapJumpListProps) {
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const closeMenu = useCallback(() => setOpen(false), []);
  useClickOutside(containerRef, closeMenu, open);

  const sorted = useMemo(
    () =>
      [...gaps].sort((a, b) =>
        a.date !== b.date
          ? a.date.localeCompare(b.date)
          : a.startTime.localeCompare(b.startTime),
      ),
    [gaps],
  );

  const count = sorted.length;
  const label = count === 1 ? "1 Lücke" : `${count} Lücken`;

  // Während die Abfrage läuft, keine Entwarnung vortäuschen — neutraler
  // Prüf-Hinweis in derselben ruhigen Optik wie der "Keine Lücken"-Zustand.
  if (state === "loading") {
    return (
      <span
        className="inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-xs text-gray-500"
        title="Personal-Lücken werden geprüft"
      >
        <span aria-hidden className="h-1.5 w-1.5 rounded-full bg-gray-400" />
        Lücken werden geprüft …
      </span>
    );
  }

  // Ohne offene Lücken bleibt der Chip als ruhige "Keine Lücken"-Anzeige
  // stehen (grauer Punkt), aber ohne Popover — es gäbe nichts anzuspringen.
  if (count === 0) {
    return (
      <span
        className="inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-xs text-gray-500"
        title="Keine offenen Lücken im sichtbaren Zeitraum"
      >
        <span aria-hidden className="h-1.5 w-1.5 rounded-full bg-gray-400" />
        Keine Lücken
      </span>
    );
  }

  return (
    <div className="relative" ref={containerRef}>
      <Button
        type="button"
        variant="ghost"
        size="compact"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        title={`${label} — Sprungliste öffnen`}
      >
        <span aria-hidden className="bg-moto-orange h-1.5 w-1.5 rounded-full" />
        {label}
      </Button>

      {open && (
        <div
          role="region"
          aria-label="Offene Lücken"
          className={`absolute left-0 z-30 mt-2 w-[min(22rem,calc(100vw-2rem))] ${timetablePopoverSurface}`}
        >
          <p className="border-b border-gray-100 px-3 py-2 text-[10px] font-semibold tracking-wider text-gray-400 uppercase">
            Offene Lücken
          </p>
          <ul className="max-h-72 divide-y divide-gray-100 overflow-y-auto">
            {sorted.map((gap) => {
              const hasCoveragePair =
                gap.plannedStaffCount !== undefined &&
                gap.plannedStaffCount > 0 &&
                gap.presentStaffCount !== undefined;
              return (
                <li key={gap.instanceId}>
                  <button
                    type="button"
                    onClick={() => {
                      setOpen(false);
                      onJump(gap);
                    }}
                    className="flex w-full flex-col gap-1 px-3 py-2 text-left transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none focus-visible:ring-inset"
                  >
                    <div className="flex items-baseline justify-between gap-2">
                      <span className="min-w-0 flex-1 truncate text-xs font-semibold text-gray-900">
                        {gap.title}
                      </span>
                      <span className="shrink-0 text-[11px] text-gray-500 tabular-nums">
                        {gap.startTime}
                      </span>
                    </div>
                    <div className="flex items-center gap-2">
                      <CoverageIndicator
                        size="sm"
                        state="gap"
                        current={
                          hasCoveragePair ? gap.presentStaffCount : undefined
                        }
                        total={
                          hasCoveragePair ? gap.plannedStaffCount : undefined
                        }
                      />
                      <span className="text-[11px] text-gray-400 tabular-nums">
                        {formatDate(gap.date)}
                      </span>
                    </div>
                  </button>
                </li>
              );
            })}
          </ul>
        </div>
      )}
    </div>
  );
}
