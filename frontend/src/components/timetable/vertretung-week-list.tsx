"use client";

/**
 * VertretungWeekList — die Störungsliste der Wochenansicht (#2030).
 *
 * Dieselbe Liste wie die Tagesansicht, nur nach Wochentagen gruppiert: pro Tag
 * eine Kopfzeile mit Störungszähler, darunter die Zeilen aus
 * `VertretungRowList`. Klassifikation, Sortierung und Zeilendarstellung kommen
 * unverändert aus vertretung-day-list.tsx — es gibt genau eine
 * Störungsdefinition, nicht eine je Ansicht.
 *
 * Reine Präsentationskomponente: kein Datenabruf, kein URL-Zugriff. Welche
 * Woche gezeigt wird, ob Lückendaten vorliegen und was ein Tagesklick auslöst,
 * entscheidet der Parent (vertretung-view.tsx).
 *
 * Ein Unterschied zur Tagesansicht ist beabsichtigt: dort fällt ein
 * störungsfreier Tag auf die Ganztagsdarstellung zurück, damit die Fläche nicht
 * leer bleibt. Über fünf Tage hinweg wäre dieser Fallback eine Wand aus
 * ungestörten Zeilen und würde die Störungen genau verdecken, die die Ansicht
 * zeigen soll. Hier bleibt ein störungsfreier Tag daher eine Zeile Text; wer
 * alle Termine sehen will, schaltet auf "Ganze Woche".
 */

import {
  CoverageIndicator,
  type CoverageState,
} from "~/components/ui/coverage-indicator";
import { Alert } from "~/components/ui/alert";
import { EmptyState } from "~/components/ui/empty-state";
import {
  classifyInstances,
  VertretungRowList,
  type VertretungDayListMode,
} from "~/components/timetable/vertretung-day-list";
import { formatDayHeader, toISODate } from "~/lib/timetable-helpers";
import type { EnrichedInstance, GapInstance } from "~/lib/timetable-types";

import { timetableSurface } from "./timetable-style";
import { VertretungListFilter } from "./vertretung-list-filter";

export interface VertretungWeekListProps {
  /** Die angezeigten Wochentage (Mo–Fr). */
  weekDays: Date[];
  /** Alle Instanzen der geladenen Woche (Filterung nach Tag passiert hier). */
  instances: EnrichedInstance[];
  /** Offene Lücken der Woche (GapsResponse.gaps, ungefiltert). */
  gaps: GapInstance[];
  /** Quittierte Lücken der Woche (GapsResponse.acknowledged, ungefiltert). */
  acknowledged: GapInstance[];
  /**
   * Erster Tag (ISO), für den Lückendaten vorliegen — der Endpunkt ist
   * vorwärtsgerichtet, in der laufenden Woche fehlen die vergangenen Tage.
   * `null` heißt: für keinen Tag der Woche liegen Lückendaten vor.
   */
  gapsAvailableFrom: string | null;
  staffNames: Map<string, string>;
  mode: VertretungDayListMode;
  /** Setzt den Filter. Ohne Callback wird die Filterzeile nicht gerendert. */
  onModeChange?: (mode: VertretungDayListMode) => void;
  canManage: boolean;
  onEdit: (instanceId: string) => void;
  /** Klick auf eine Tageskopfzeile — springt in die Tagesansicht des Tages. */
  onSelectDay: (dateISO: string) => void;
  todayISO: string;
  className?: string;
}

export function VertretungWeekList({
  weekDays,
  instances,
  gaps,
  acknowledged,
  gapsAvailableFrom,
  staffNames,
  mode,
  onModeChange,
  canManage,
  onEdit,
  onSelectDay,
  todayISO,
  className,
}: VertretungWeekListProps) {
  // flex-col: die Filterzeile bleibt stehen, nur die Liste darunter scrollt.
  const containerClassName = className
    ? `${timetableSurface} flex flex-col ${className}`
    : `${timetableSurface} flex flex-col`;

  return (
    <div className={containerClassName} data-testid="vertretung-week-list">
      {onModeChange && (
        <VertretungListFilter
          mode={mode}
          onModeChange={onModeChange}
          allLabel="Ganze Woche"
        />
      )}
      {/* Der Scroll-Container sitzt INNERHALB der Karte (die Karte selbst
          bekommt vom Aufrufer nur die Höhe und overflow-hidden): so bleibt die
          Scrollleiste innerhalb des Rahmens und die abgerundeten Ecken bleiben
          sauber, statt dass die Leiste auf dem Rand klebt. Die sticky
          Tageskopfzeilen kleben an diesem Container. */}
      <div className="min-h-0 flex-1 overflow-y-auto rounded-2xl">
        {weekDays.map((day) => {
          const iso = toISODate(day);
          const gapsAvailable =
            gapsAvailableFrom !== null && iso >= gapsAvailableFrom;
          const dayInstances = instances.filter((inst) => inst.date === iso);
          const dayGaps = gaps.filter((gap) => gap.date === iso);
          const dayAcknowledged = acknowledged.filter(
            (gap) => gap.date === iso,
          );
          const classified = classifyInstances(
            dayInstances,
            dayGaps,
            dayAcknowledged,
            gapsAvailable,
          );
          const disturbed = classified.filter((row) => row.isDisturbed);
          const visibleRows = mode === "ganzer-tag" ? classified : disturbed;

          // Ohne geladene Lückendaten ist der Zähler unvollständig — dann einen
          // Strich zeigen, nie eine erfundene 0 (Muster der Tagesansicht).
          const coverageState: CoverageState = !gapsAvailable
            ? "covered"
            : dayGaps.length > 0
              ? "gap"
              : dayAcknowledged.length > 0
                ? "acknowledged"
                : "covered";
          const countLabel = !gapsAvailable
            ? "–"
            : disturbed.length === 1
              ? "1 Störung"
              : `${disturbed.length} Störungen`;

          return (
            <section
              key={iso}
              data-testid={`vertretung-week-list-day-${iso}`}
              className="border-b border-gray-200 last:border-b-0"
            >
              <button
                type="button"
                onClick={() => onSelectDay(iso)}
                aria-label={`${formatDayHeader(day)} als Tagesansicht öffnen`}
                // Sticky, weil der Aufrufer die Liste in einen scrollenden
                // Container mit der Höhe des Rasters setzt: beim Scrollen muss
                // sichtbar bleiben, zu welchem Tag die Zeilen gehören. Deckender
                // Hintergrund (kein /70), sonst scheinen die Zeilen durch.
                className="sticky top-0 z-10 flex w-full items-center justify-between gap-2 border-b border-gray-200 bg-gray-50 px-3 py-2 text-left transition-colors hover:bg-gray-100 focus-visible:ring-2 focus-visible:ring-gray-900 focus-visible:outline-none focus-visible:ring-inset"
              >
                <span className="flex items-center gap-2">
                  <span className="text-xs font-semibold text-gray-900">
                    {formatDayHeader(day)}
                  </span>
                  {iso === todayISO && (
                    <span className="rounded-full bg-gray-900 px-1.5 py-0.5 text-[10px] font-medium text-white">
                      Heute
                    </span>
                  )}
                </span>
                <CoverageIndicator
                  size="sm"
                  state={coverageState}
                  label={countLabel}
                  title={
                    gapsAvailable
                      ? `${disturbed.length} Störung(en) an diesem Tag`
                      : "Störungslage für diesen Tag nicht verfügbar"
                  }
                />
              </button>

              {dayInstances.length === 0 ? (
                <div className="px-3 py-1">
                  <EmptyState
                    variant="compact"
                    title="Keine Termine an diesem Tag"
                  />
                </div>
              ) : visibleRows.length === 0 ? (
                <div className="px-3 py-1">
                  <EmptyState
                    variant="compact"
                    title={
                      gapsAvailable
                        ? "Keine Störungen an diesem Tag"
                        : "Störungslage konnte nicht geprüft werden"
                    }
                  />
                </div>
              ) : (
                <>
                  {!gapsAvailable && (
                    <div className="px-3 pt-2">
                      <Alert
                        type="warning"
                        message="Störungslage konnte nicht vollständig geprüft werden"
                      />
                    </div>
                  )}
                  <VertretungRowList
                    rows={visibleRows}
                    staffNames={staffNames}
                    canManage={canManage}
                    onEdit={onEdit}
                  />
                </>
              )}
            </section>
          );
        })}
      </div>
    </div>
  );
}
