"use client";

/**
 * VertretungDayList — die Störungsliste des Vertretungsbereichs
 * (docs/planung-redesign/docs/07-vertretung.md Abschnitt 2.2).
 *
 * Reine Präsentationskomponente: kein Datenabruf, kein URL-Zugriff. Welcher
 * Tag angezeigt wird und der Umschalter "Nur Störungen"/"Ganzer Tag" leben im
 * Parent (vertretung-view.tsx); der Filterzustand kommt hier nur als Prop an.
 *
 * Die Klassifikation (`classifyInstances`) und die Zeilendarstellung
 * (`VertretungRowList`) sind exportiert, weil die Wochenansicht
 * (vertretung-week-list.tsx, #2030) dieselben Regeln pro Tag anwendet. Es gibt
 * genau eine Störungsdefinition, nicht zwei je Ansicht.
 *
 * Störungsdefinition (Abschnitt 2.2, Bedingungen 1-4): eine Position gilt als
 * gestört, wenn sie in `gaps` steht (offene Lücke), in `acknowledged` steht
 * (quittiert), `status === "cancelled"` ist (abgesagt), oder mindestens eine
 * geplante (nicht substituierende) Person `isAbsent === true` hat — auch mit
 * bereits gesetztem Ersatz bleibt die Störung sichtbar. Bei gleicher
 * Startzeit ordnet ein Schwere-Sekundärschlüssel: offene Lücke vor
 * Abwesenheit-mit-Ersatz vor quittiert vor abgesagt.
 */

import { Alert } from "~/components/ui/alert";
import { EmptyState } from "~/components/ui/empty-state";
import { Button } from "~/components/ui/button";
import {
  CoverageIndicator,
  type CoverageState,
} from "~/components/ui/coverage-indicator";
import { staffLabel } from "~/lib/timetable-helpers";
import type {
  EnrichedInstance,
  GapInstance,
  InstanceStaffSummary,
} from "~/lib/timetable-types";

import { timetableSurface } from "./timetable-style";
import {
  VertretungListFilter,
  type VertretungDayListMode as FilterMode,
} from "./vertretung-list-filter";

// Der Typ lebt beim Filter (siehe dort), wird hier aber unter dem gewohnten
// Namen weitergereicht, damit die bestehenden Aufrufer nichts ändern müssen.
export type VertretungDayListMode = FilterMode;

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
  /** Ob offene und quittierte Lücken für den angezeigten Tag geladen wurden. */
  gapsAvailable: boolean;
  /**
   * Anzeigenamen fürs Personal. InstanceStaffSummary trägt nur eine staffId,
   * keinen Namen — die Zeilen "Person abwesend" / "Ersatzkräfte: Namen" brauchen
   * daher eine Namensauflösung, dasselbe Muster wie in
   * substitution-slide-over.tsx.
   */
  staffNames: Map<string, string>;
  mode: VertretungDayListMode;
  /** Setzt den Filter. Ohne Callback wird die Filterzeile nicht gerendert. */
  onModeChange?: (mode: VertretungDayListMode) => void;
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

export interface ClassifiedRow {
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
  gapsAvailable: boolean,
): ClassifiedRow {
  const gapMatch = gapsAvailable
    ? gaps.find((g) => g.instanceId === instance.id)
    : undefined;
  const ackMatch = gapsAvailable
    ? acknowledged.find((g) => g.instanceId === instance.id)
    : undefined;
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

function compareRows(a: ClassifiedRow, b: ClassifiedRow): number {
  if (a.instance.startTime !== b.instance.startTime) {
    return a.instance.startTime < b.instance.startTime ? -1 : 1;
  }
  return a.severityRank - b.severityRank;
}

/**
 * Klassifiziert und sortiert die Instanzen eines Tages (Startzeit, bei
 * Gleichstand Schwere). Reine Funktion, von Tages- und Wochenansicht geteilt.
 */
export function classifyInstances(
  instances: EnrichedInstance[],
  gaps: GapInstance[],
  acknowledged: GapInstance[],
  gapsAvailable: boolean,
): ClassifiedRow[] {
  return instances
    .map((instance) => classify(instance, gaps, acknowledged, gapsAvailable))
    .sort(compareRows);
}

/** Die Zeilenliste ohne Kartenrahmen — beide Ansichten rendern dieselben Zeilen. */
export function VertretungRowList({
  rows,
  staffNames,
  canManage,
  onEdit,
}: {
  rows: ClassifiedRow[];
  staffNames: Map<string, string>;
  canManage: boolean;
  onEdit: (instanceId: string) => void;
}) {
  return (
    <ul className="divide-y divide-gray-200">
      {rows.map((row) => (
        <VertretungDayListRow
          key={row.instance.id}
          row={row}
          staffNames={staffNames}
          canManage={canManage}
          onEdit={onEdit}
        />
      ))}
    </ul>
  );
}

export function VertretungDayList({
  instances,
  gaps,
  acknowledged,
  gapsAvailable,
  staffNames,
  mode,
  onModeChange,
  canManage,
  onEdit,
  className,
}: VertretungDayListProps) {
  const containerClassName = className
    ? `${timetableSurface} ${className}`
    : timetableSurface;
  const filter = onModeChange ? (
    <VertretungListFilter
      mode={mode}
      onModeChange={onModeChange}
      allLabel="Ganzer Tag"
    />
  ) : null;

  if (instances.length === 0) {
    return (
      <div className={containerClassName}>
        {filter}
        <div className="px-4 py-2">
          <EmptyState variant="compact" title="Keine Termine an diesem Tag" />
        </div>
      </div>
    );
  }

  const classified = classifyInstances(
    instances,
    gaps,
    acknowledged,
    gapsAvailable,
  );
  const disturbed = classified.filter((row) => row.isDisturbed);
  // Ohne Gaps kann der Client nicht wissen, ob ein ansonsten unauffälliger
  // Block unterbesetzt ist. Deshalb weder Entwarnung noch Ganztags-Fallback.
  const showAll =
    mode === "ganzer-tag" || (gapsAvailable && disturbed.length === 0);
  const visibleRows = showAll ? classified : disturbed;

  return (
    <div className={containerClassName}>
      {filter}
      {!gapsAvailable && (
        <div className="border-b border-gray-200 p-3">
          <Alert
            type="warning"
            message="Störungslage konnte nicht vollständig geprüft werden"
          />
        </div>
      )}
      {gapsAvailable && disturbed.length === 0 && (
        <div className="border-b border-gray-200 px-3 py-2">
          <EmptyState variant="compact" title="Keine Störungen an diesem Tag" />
        </div>
      )}
      <VertretungRowList
        rows={visibleRows}
        staffNames={staffNames}
        canManage={canManage}
        onEdit={onEdit}
      />
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
  // Das Datenmodell kennt nur "Ersatz für diesen Block", nicht "Ersatz für
  // diese abwesende Person". Namen daher nur blockweit aggregiert ausgeben.
  const substituteNames = hasAbsence
    ? [
        ...new Set(
          instance.staff
            .filter((row) => row.isSubstitute && !row.isAbsent)
            .map((row) => staffLabel(staffNames, row.staffId)),
        ),
      ]
    : [];

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
        {/* Auch Lesenutzer brauchen ein Klickziel: unterhalb lg gibt es keine
            Kalenderspalte, und der Verlauf muss ohne schedules:manage lesbar
            bleiben (Kriterium 8). Der Editor selbst rendert ohne canManage
            nur Lese-Inhalte.
            Mit Verwaltungsrechten ist "Bearbeiten" die Hauptaktion der Zeile
            und trägt deshalb die weiße Kit-Fläche mit Rahmen (#2029). Ohne die
            Rechte bleibt "Details" die ruhigere Ghost-Variante, damit ein
            reiner Lesezugriff nicht wie eine Aufforderung zum Ändern wirkt. */}
        <Button
          type="button"
          variant={canManage ? "outline" : "ghost"}
          size="compact"
          onClick={() => onEdit(instance.id)}
        >
          {canManage ? "Bearbeiten" : "Details"}
        </Button>
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
        <div className="flex flex-col gap-1 text-[11px] text-gray-600">
          <ul className="flex flex-col gap-1">
            {absentPlannedRows.map((staffRow) => {
              const name = staffLabel(staffNames, staffRow.staffId);
              return (
                <li key={staffRow.staffId}>
                  <span className="font-medium text-gray-900">{name}</span>
                  {" abwesend"}
                  {staffRow.absenceReason ? ` (${staffRow.absenceReason})` : ""}
                </li>
              );
            })}
          </ul>
          <p>
            Ersatzkräfte:{" "}
            {substituteNames.length > 0 ? substituteNames.join(", ") : "keine"}
          </p>
        </div>
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
