"use client";

/**
 * VertretungDayList — die Störungsliste des Vertretungsbereichs
 * (docs/planung-redesign/docs/07-vertretung.md Abschnitt 2.2).
 *
 * Reine Präsentationskomponente: kein Datenabruf, kein URL-Zugriff. Welcher
 * Tag angezeigt wird und der Umschalter "Nur Störungen"/"Ganzer Tag" leben im
 * Parent (vertretung-view.tsx); der Filterzustand kommt hier nur als Prop an.
 *
 * Störungsdefinition (Abschnitt 2.2, Bedingungen 1-4): eine Position gilt als
 * gestört, wenn sie in `gaps` steht (offene Lücke), in `acknowledged` steht
 * (quittiert), `status === "cancelled"` ist (abgesagt), oder mindestens eine
 * geplante (nicht substituierende) Person `isAbsent === true` hat — auch mit
 * bereits gesetztem Ersatz bleibt die Störung sichtbar. Bei gleicher
 * Startzeit ordnet ein Schwere-Sekundärschlüssel: offene Lücke vor
 * Abwesenheit-mit-Ersatz vor quittiert vor abgesagt.
 */

import { Button } from "~/components/ui/button";
import {
  CoverageIndicator,
  type CoverageState,
} from "~/components/ui/coverage-indicator";
import type {
  EnrichedInstance,
  GapInstance,
  InstanceStaffSummary,
} from "~/lib/timetable-types";

import { timetableSurface } from "./timetable-style";

export type VertretungDayListMode = "stoerungen" | "ganzer-tag";

export interface StaffTriple {
  readonly planned: number;
  readonly present: number;
  readonly absent: number;
}

export interface VertretungDayListProps {
  /** Alle Instanzen des angezeigten Tages. */
  instances: EnrichedInstance[];
  /** Offene Lücken des Tages (GapsResponse.gaps, bereits tagesgefiltert). */
  gaps: GapInstance[];
  /** Quittierte Lücken des Tages (GapsResponse.acknowledged, tagesgefiltert). */
  acknowledged: GapInstance[];
  /**
   * Anzeigenamen fürs Personal. InstanceStaffSummary trägt nur eine staffId,
   * keinen Namen — die Zeilen "Person abwesend" / "Ersatz: Name" brauchen
   * daher eine Namensauflösung, dasselbe Muster wie in
   * substitution-slide-over.tsx.
   */
  staffNames: Map<string, string>;
  mode: VertretungDayListMode;
  canManage: boolean;
  onEdit: (instanceId: string) => void;
  className?: string;
}

/**
 * Pure Soll/Ist/Abwesend-Berechnung aus instance.staff — für Zeilen, die nur
 * über Bedingung 3 (abgesagt) oder 4 (Abwesenheit) in die Liste kommen und
 * daher keine GapInstance-Zahlen für diese Instanz haben. Spiegelt die
 * Backend-Semantik von presentStaffCount (docs/07-vertretung.md 2.2):
 * geplante Positionen = Zeilen ohne isSubstitute; anwesend = nicht-abwesende
 * geplante Zeilen plus deckende (nicht-abwesende) Substitut-Zeilen; abwesend
 * = alle isAbsent-Zeilen.
 */
export function computeStaffTriple(instance: EnrichedInstance): StaffTriple {
  const plannedRows = instance.staff.filter((row) => !row.isSubstitute);
  const substituteRows = instance.staff.filter((row) => row.isSubstitute);
  const presentPlanned = plannedRows.filter((row) => !row.isAbsent).length;
  const coveringSubstitutes = substituteRows.filter(
    (row) => !row.isAbsent,
  ).length;
  return {
    planned: plannedRows.length,
    present: presentPlanned + coveringSubstitutes,
    absent: instance.staff.filter((row) => row.isAbsent).length,
  };
}

interface ClassifiedRow {
  instance: EnrichedInstance;
  ackMatch?: GapInstance;
  isCancelled: boolean;
  hasAbsence: boolean;
  absentPlannedRows: InstanceStaffSummary[];
  isDisturbed: boolean;
  severityRank: number;
  coverageState: CoverageState;
  triple: StaffTriple;
}

function classify(
  instance: EnrichedInstance,
  gaps: GapInstance[],
  acknowledged: GapInstance[],
): ClassifiedRow {
  const gapMatch = gaps.find((g) => g.instanceId === instance.id);
  const ackMatch = acknowledged.find((g) => g.instanceId === instance.id);
  const isCancelled = instance.status === "cancelled";
  const absentPlannedRows = instance.staff.filter(
    (row) => !row.isSubstitute && row.isAbsent,
  );
  const hasAbsence = absentPlannedRows.length > 0;
  const isGap = gapMatch !== undefined;
  const isAcknowledged = ackMatch !== undefined;
  const isDisturbed = isGap || isAcknowledged || isCancelled || hasAbsence;

  // Schwere-Reihenfolge aus docs/07-vertretung.md 2.2: offene Lücke vor
  // Abwesenheit-mit-Ersatz (Bedingung 4 ohne offene Lücke) vor quittiert vor
  // abgesagt. Ungestörte Zeilen (nur im Ganztags-Fallback sichtbar) tragen
  // den niedrigsten Rang, damit sie bei einer Zeitgleichheit hinter jeder
  // gestörten Zeile einsortieren.
  let severityRank = 4;
  if (isGap) severityRank = 0;
  else if (hasAbsence) severityRank = 1;
  else if (isAcknowledged) severityRank = 2;
  else if (isCancelled) severityRank = 3;

  const coverageState: CoverageState = isGap
    ? "gap"
    : isAcknowledged
      ? "acknowledged"
      : "covered";

  const gapSource = gapMatch ?? ackMatch;
  const triple: StaffTriple =
    gapSource?.plannedStaffCount !== undefined &&
    gapSource.presentStaffCount !== undefined
      ? {
          planned: gapSource.plannedStaffCount,
          present: gapSource.presentStaffCount,
          absent: gapSource.absentStaffCount,
        }
      : computeStaffTriple(instance);

  return {
    instance,
    ackMatch,
    isCancelled,
    hasAbsence,
    absentPlannedRows,
    isDisturbed,
    severityRank,
    coverageState,
    triple,
  };
}

/**
 * Best-effort Zuordnung abwesend -> deckender Ersatz. InstanceStaffSummary
 * verknüpft eine Ersatzperson nicht explizit mit der Position, die sie
 * deckt (nur isSubstitute/isAbsent-Flags auf einer flachen Liste) — der
 * Server kann dieselbe Ersatzperson sogar für zwei abwesende Positionen zu
 * einer einzigen deckenden Zeile zusammenfassen. Diese Funktion paart
 * abwesende geplante Zeilen positionsweise mit den aktiven (nicht
 * abwesenden) Substitut-Zeilen; reicht der Vorrat nicht, bleibt die
 * Position "??" (WebUntis-Konvention).
 */
function substituteForAbsent(
  instance: EnrichedInstance,
): Map<string, string | null> {
  const plannedAbsent = instance.staff.filter(
    (row) => !row.isSubstitute && row.isAbsent,
  );
  const activeSubstitutes = instance.staff.filter(
    (row) => row.isSubstitute && !row.isAbsent,
  );
  const result = new Map<string, string | null>();
  plannedAbsent.forEach((row, index) => {
    result.set(row.staffId, activeSubstitutes[index]?.staffId ?? null);
  });
  return result;
}

function compareRows(a: ClassifiedRow, b: ClassifiedRow): number {
  if (a.instance.startTime !== b.instance.startTime) {
    return a.instance.startTime < b.instance.startTime ? -1 : 1;
  }
  return a.severityRank - b.severityRank;
}

export function VertretungDayList({
  instances,
  gaps,
  acknowledged,
  staffNames,
  mode,
  canManage,
  onEdit,
  className,
}: VertretungDayListProps) {
  const containerClassName = className
    ? `${timetableSurface} ${className}`
    : timetableSurface;

  if (instances.length === 0) {
    return (
      <div className={containerClassName}>
        <p className="p-4 text-sm text-gray-500">
          Keine Termine an diesem Tag.
        </p>
      </div>
    );
  }

  const classified = instances
    .map((instance) => classify(instance, gaps, acknowledged))
    .sort(compareRows);
  const disturbed = classified.filter((row) => row.isDisturbed);
  // Ein Tag ohne jede Störung fällt unabhängig vom angeforderten Filter auf
  // die Ganztagsdarstellung zurück (docs/07-vertretung.md 2.2).
  const showAll = mode === "ganzer-tag" || disturbed.length === 0;
  const visibleRows = showAll ? classified : disturbed;

  return (
    <div className={containerClassName}>
      {disturbed.length === 0 && (
        <p className="border-b border-gray-200 p-3 text-xs font-medium text-gray-500">
          Keine Störungen an diesem Tag
        </p>
      )}
      <ul className="divide-y divide-gray-200">
        {visibleRows.map((row) => (
          <VertretungDayListRow
            key={row.instance.id}
            row={row}
            staffNames={staffNames}
            canManage={canManage}
            onEdit={onEdit}
          />
        ))}
      </ul>
    </div>
  );
}

function VertretungDayListRow({
  row,
  staffNames,
  canManage,
  onEdit,
}: {
  row: ClassifiedRow;
  staffNames: Map<string, string>;
  canManage: boolean;
  onEdit: (instanceId: string) => void;
}) {
  const {
    instance,
    isDisturbed,
    hasAbsence,
    isCancelled,
    ackMatch,
    absentPlannedRows,
    triple,
    coverageState,
  } = row;
  // "Ganzer Tag" blendet ungestörte Positionen gedämpft grau ein, ohne
  // Störungsvermerke (docs/07-vertretung.md 2.2) — die Vermerke selbst bleiben
  // ohnehin aus, weil ihre Bedingungen für eine ungestörte Zeile nie zutreffen.
  const titleClass = isDisturbed ? "text-gray-900" : "text-gray-400";
  const timeClass = isDisturbed ? "text-gray-500" : "text-gray-400";
  const substituteMap = hasAbsence ? substituteForAbsent(instance) : null;

  return (
    <li
      data-testid={`vertretung-day-list-row-${instance.id}`}
      className="flex flex-col gap-1.5 p-3"
    >
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="flex flex-col gap-0.5">
          <span className={`text-[11px] font-medium tabular-nums ${timeClass}`}>
            {instance.startTime}–{instance.endTime}
          </span>
          <span className={`text-xs font-semibold ${titleClass}`}>
            {instance.title}
          </span>
        </div>
        {canManage && (
          <Button
            type="button"
            variant="ghost"
            size="compact"
            onClick={() => onEdit(instance.id)}
          >
            Bearbeiten
          </Button>
        )}
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <CoverageIndicator
          state={coverageState}
          current={triple.present}
          total={triple.planned}
          note={
            coverageState === "acknowledged" ? "bewusst unbesetzt" : undefined
          }
        />
        <span className="text-[11px] text-gray-500 tabular-nums">
          Soll {triple.planned} · Ist {triple.present} · Abwesend{" "}
          {triple.absent}
        </span>
      </div>

      {hasAbsence && (
        <ul className="flex flex-col gap-1">
          {absentPlannedRows.map((staffRow) => {
            const name = staffNames.get(staffRow.staffId) ?? staffRow.staffId;
            const substituteId = substituteMap?.get(staffRow.staffId) ?? null;
            const substituteName = substituteId
              ? (staffNames.get(substituteId) ?? substituteId)
              : null;
            return (
              <li key={staffRow.staffId} className="text-[11px] text-gray-600">
                <span className="font-medium text-gray-900">{name}</span>
                {" abwesend"}
                {staffRow.absenceReason ? ` (${staffRow.absenceReason})` : ""}
                {" · "}
                {substituteName ? `Ersatz: ${substituteName}` : "Ersatz: ??"}
              </li>
            );
          })}
        </ul>
      )}

      {ackMatch?.understaffedNote && (
        <p className="text-[11px] text-gray-500">
          Grund: {ackMatch.understaffedNote}
        </p>
      )}

      {isCancelled && (
        <p className="text-[11px] text-gray-600">
          abgesagt{instance.cancelReason ? `: ${instance.cancelReason}` : ""}
        </p>
      )}
    </li>
  );
}
