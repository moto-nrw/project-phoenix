"use client";

/**
 * Lückenzähler plus Sprungliste des Betreuungsplan-Kopfzeilen-Kontexts. Der Chip
 * zeigt die Zahl der offenen Personal-Lücken; ein Klick öffnet eine schmale
 * Sprungliste. Jede Zeile trägt Startzeit, Titel und das Soll/Ist-Paar; ein
 * Klick springt zum Block (der Aufrufer setzt `d` und `block`).
 *
 * Reine Präsentation: keine Datenabfrage, kein URL-Zugriff. Die offenen Lücken
 * kommen als Prop; das Springen ist ein Callback.
 */

import { useMemo, useState } from "react";

import { AnchoredPopover } from "~/components/ui/anchored-popover";
import { Button } from "~/components/ui/button";
import { CoverageIndicator } from "~/components/ui/planning/coverage-indicator";
import type { GapInstance } from "~/lib/timetable-types";

import { formatDate } from "~/lib/date-helpers";
import { StatusDot } from "~/components/ui/status-dot";
import { StatusBadge } from "~/components/ui/status-badge";

interface GapJumpListProps {
  /** Offene (nicht quittierte) Lücken des sichtbaren Zeitraums. */
  gaps: GapInstance[];
  /** Springt zum Block der Lücke — der Aufrufer setzt `d` und `block`. */
  onJump: (gap: GapInstance) => void;
  /**
   * Solange die Lücken-Abfrage läuft, zeigt der Chip einen neutralen
   * Prüf-Hinweis statt der Entwarnung "Keine Lücken" — ein leeres Array vor
   * Datenankunft ist keine bestätigte Aussage. Den Fehlerfall blendet der
   * Aufrufer aus; er zeigt den Ladefehler vor Ort an (#2516).
   */
  state?: "ready" | "loading";
}

export function GapJumpList({
  gaps,
  onJump,
  state = "ready",
}: GapJumpListProps) {
  const [open, setOpen] = useState(false);

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
      <StatusBadge
        label="Lücken werden geprüft …"
        title="Personal-Lücken werden geprüft"
        tone="gray"
        compact
      />
    );
  }

  // Ohne offene Lücken bleibt der Chip als ruhige "Keine Lücken"-Anzeige
  // stehen (graue Pille), aber ohne Popover — es gäbe nichts anzuspringen.
  if (count === 0) {
    return (
      <StatusBadge
        label="Keine Lücken"
        title="Keine offenen Lücken im sichtbaren Zeitraum"
        tone="gray"
        compact
      />
    );
  }

  return (
    <AnchoredPopover
      open={open}
      onOpenChange={setOpen}
      ariaLabel="Offene Lücken"
      preferredWidth={352}
      focusPanelOnOpen
      header={
        <p className="text-sm font-semibold text-gray-900">Offene Lücken</p>
      }
      renderTrigger={({ ref, toggle, panelId }) => (
        <Button
          ref={ref}
          type="button"
          variant="ghost"
          size="compact"
          onClick={toggle}
          aria-expanded={open}
          aria-haspopup="dialog"
          aria-controls={open ? panelId : undefined}
          title={`${label} — Sprungliste öffnen`}
        >
          <StatusDot className="bg-moto-orange" decorative />
          {label}
        </Button>
      )}
    >
      {({ close }) => (
        <ul className="divide-y divide-gray-100">
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
                    close();
                    onJump(gap);
                  }}
                  className="flex w-full flex-col gap-1 px-3 py-2 text-left transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none focus-visible:ring-inset"
                >
                  <div className="flex items-baseline justify-between gap-2">
                    <span className="min-w-0 flex-1 truncate text-xs font-semibold text-gray-900">
                      {gap.title}
                    </span>
                    <span className="shrink-0 text-xs text-gray-500 tabular-nums">
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
                    <span className="text-xs text-gray-400 tabular-nums">
                      {formatDate(gap.date)}
                    </span>
                  </div>
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </AnchoredPopover>
  );
}
