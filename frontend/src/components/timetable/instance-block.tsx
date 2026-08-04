"use client";

/**
 * InstanceBlock — absolutely-positioned event card for the WeeklyCalendarGrid.
 *
 * Die Block-Rezeptur (weiße Fläche, 3px Farbkante, 10-Prozent-Tönung,
 * `cancelled`-Rendering, genau ein Status-Icon) lebt ausschließlich im
 * Kit-Baustein `ui/plan-block.tsx` (docs/planung-redesign/docs/06-betreuungsplan.md
 * Abschnitt 2.2, plan-design-guards.test.ts). InstanceBlock baut keine eigene
 * Farbfläche mehr: es leitet nur die Kategoriefarbe ab, komponiert den
 * datenreichen Fuß (Besetzung als `CoverageIndicator`, Personen-/Kinderzahlen,
 * Spontan-, Abwesend- und Ersatz-Marker) und positioniert den Block absolut in
 * seiner Tagesspalte. Die Daten-Props bleiben unverändert.
 */

import type { ReactNode } from "react";

import { CircleCheck, TriangleAlert } from "lucide-react";

import { CoverageIndicator } from "~/components/ui/coverage-indicator";
import { PlanBlock } from "~/components/ui/plan-block";
import type { EnrichedInstance } from "~/lib/timetable-types";
import { useShowTimetableCounts } from "~/lib/tenant-context";

interface InstanceBlockProps {
  instance: EnrichedInstance;
  /** Pixel offset from the day column's hour-0 line. */
  top: number;
  /** Block height in pixels. */
  height: number;
  /** Inset from the column's left edge as a percentage string ("0%" / "50%"). */
  left: string;
  /** Block width as a percentage string ("100%" / "50%"). */
  width: string;
  isSelected: boolean;
  onClick: () => void;
  /**
   * Markiert diesen Block als OFFENE (nicht quittierte) Personal-Lücke — seine
   * Instanz-ID liegt im `gapInstanceIds`-Set des WeeklyCalendarGrid. Treibt das
   * eine #F78C10-Lücken-Icon. Ein abgesagter Block ignoriert das Flag: das
   * Cancelled-Rendering hat Vorrang und zeigt kein Lücken-Icon.
   */
  isGap?: boolean;
}

const COMPACT_HEIGHT_PX = 48;
const TINY_HEIGHT_PX = 30;
/**
 * Zeitzeile (17px) + Titel (15px) + Padding und Rahmen (14px) brauchen rund
 * 46px, die Fußzeile (Besetzung, Raum, Zählungen) noch einmal rund 17px. Ein
 * kürzerer Block bekommt daher keine Fußzeile: sie ragte sonst unten heraus,
 * wurde von `overflow-hidden` abgeschnitten und drückte den Titel optisch an
 * die Unterkante. Ein 30-Minuten-Block ist bei 90px/Stunde genau 45px hoch —
 * der Regelfall, nicht der Ausnahmefall.
 */
const FOOTER_MIN_HEIGHT_PX = 64;

/** Bewusst-unbesetzt-Blöcke lesen sich vollständig neutral: graue Kante, keine
 *  Tönung; die Semantik trägt der CoverageIndicator-Vermerk (Spec 5.3). */
const ACK_EDGE_COLOR = "#6B7280";

export function InstanceBlock({
  instance,
  top,
  height,
  left,
  width,
  isSelected,
  onClick,
  isGap = false,
}: InstanceBlockProps) {
  const showTimetableCounts = useShowTimetableCounts();
  const isCancelled = instance.status === "cancelled";
  const isActive = instance.status === "active";
  const hasConflict = instance.conflictWarnings.length > 0;
  const isCompact = height <= COMPACT_HEIGHT_PX;
  const isTiny = height <= TINY_HEIGHT_PX;
  const showFooter = height >= FOOTER_MIN_HEIGHT_PX;
  const showCompactStatus = !showFooter;

  // #1840 Vertretungsplan deviation signals. A removed substitute keeps
  // is_substitute=true but is marked is_absent=true — they are no longer
  // covering, so only a NON-absent substitute counts as an active replacement.
  const isUnderstaffedAck = instance.understaffedAck === true && !isCancelled;
  const hasSubstitute = instance.staff.some(
    (s) => s.isSubstitute && !s.isAbsent,
  );
  const absentCount = instance.absentStaffCount;
  const hasCoverageGap =
    instance.requiredStaffCount > 0 &&
    instance.assignedStaffCount < instance.requiredStaffCount;

  const compactStatusDetails = [
    isUnderstaffedAck ? "bewusst unbesetzt" : null,
    !isUnderstaffedAck && hasCoverageGap
      ? `${instance.assignedStaffCount} von ${instance.requiredStaffCount} Positionen besetzt`
      : null,
    isActive ? "läuft" : null,
    absentCount > 0 ? `${absentCount} abwesend` : null,
    hasSubstitute ? "Ersatz" : null,
  ].filter((detail): detail is string => detail !== null);

  const edgeColor = isUnderstaffedAck
    ? ACK_EDGE_COLOR
    : instance.planningTrackColor;

  // Genau ein Status-Icon (PlanBlock erlaubt eines). Priorität:
  // cancelled (kein Icon, Cancelled-Rendering gewinnt) > offene Lücke >
  // bewusst unbesetzt (kein Icon, per CoverageIndicator-Vermerk) > Konflikt.
  let statusIcon: ReactNode = null;
  if (!isCancelled) {
    if (showCompactStatus && isUnderstaffedAck) {
      // Der reguläre CoverageIndicator liegt in der Fußzeile. Bei kurzen
      // Blöcken ist diese bewusst ausgeblendet; der Check hält den
      // quittierten Personalmangel trotzdem unmittelbar sichtbar.
      statusIcon = (
        <CircleCheck
          className="h-3.5 w-3.5 text-gray-500"
          aria-label="Bewusst unbesetzt"
        />
      );
    } else if (isGap) {
      statusIcon = (
        <TriangleAlert
          className="h-3.5 w-3.5 text-[#F78C10]"
          aria-label="Offene Lücke"
        />
      );
    } else if (!isUnderstaffedAck && hasConflict) {
      statusIcon = (
        <TriangleAlert
          className="h-3.5 w-3.5 text-[#EAB308]"
          aria-label={`${instance.conflictWarnings.length} Konflikte`}
        />
      );
    } else if (showCompactStatus && compactStatusDetails.length > 0) {
      // Ein Block ohne Fußzeile bietet keinen Platz für die ausführliche
      // Statusdarstellung.
      // Die vier kleinen Marker bewahren deshalb die unabhängigen Signale
      // Besetzung, laufender Termin, Abwesenheit und Ersatz zugleich.
      statusIcon = (
        <span
          className="grid h-3.5 w-3.5 grid-cols-2 content-center justify-items-center gap-px"
          aria-label={compactStatusDetails.join(", ")}
        >
          {hasCoverageGap && (
            <span className="h-1.5 w-1.5 rounded-full bg-[#F78C10]" />
          )}
          {isActive && (
            <span className="h-1.5 w-1.5 rounded-full bg-[#83CD2D]" />
          )}
          {absentCount > 0 && (
            <span className="h-1.5 w-1.5 rounded-full bg-[#CC2626]" />
          )}
          {hasSubstitute && (
            <span className="h-1.5 w-1.5 rounded-full bg-[#5A8E1F]" />
          )}
        </span>
      );
    }
  }

  // Besetzung als CoverageIndicator (Kriterium 6). Abgesagte Blöcke haben
  // keinen Bedarf; bewusst unbesetzt zeigt den grauen Vermerk unabhängig von
  // der Soll-Zahl; sonst nur bei tatsächlichem Personalbedarf.
  let coverage: ReactNode = null;
  if (!isCancelled) {
    if (isUnderstaffedAck) {
      coverage = (
        <CoverageIndicator
          size="sm"
          state="acknowledged"
          note="bewusst unbesetzt"
        />
      );
    } else if (instance.requiredStaffCount > 0) {
      coverage = (
        <CoverageIndicator
          size="sm"
          state={
            instance.assignedStaffCount >= instance.requiredStaffCount
              ? "covered"
              : "gap"
          }
          current={instance.assignedStaffCount}
          total={instance.requiredStaffCount}
        />
      );
    }
  }

  const totalStudents =
    instance.expectedStudentsCount + instance.presentStudentsCount;

  const footer = !showFooter ? undefined : (
    <span className="flex min-w-0 flex-col gap-0.5">
      {!isCompact && !isCancelled && instance.roomName && (
        <span className="truncate text-[10px] text-gray-500">
          {instance.roomName}
        </span>
      )}

      {!isCompact && !isCancelled && instance.planningTrackName && (
        <span className="truncate text-[10px] font-medium text-gray-600">
          {instance.planningTrackName}
        </span>
      )}

      {!isCompact && instance.isSpontaneous && !isCancelled && (
        <span
          className="truncate text-[10px] font-semibold text-gray-600"
          title="Dieser Termin wurde spontan gestartet und war nicht geplant."
        >
          Spontan
        </span>
      )}

      {!isCancelled && (coverage || isActive || !isCompact) && (
        <span className="flex flex-wrap items-center gap-1.5 text-[10px] text-gray-500">
          {isActive && (
            <span
              className="inline-block h-1.5 w-1.5 rounded-full bg-[#83CD2D]"
              aria-label="läuft"
            />
          )}
          {coverage}
          {!isCompact && (
            <span className="truncate">
              {instance.staffCount} P
              {showTimetableCounts && ` · ${totalStudents} K`}
              {showTimetableCounts && isActive && totalStudents > 0
                ? ` · ${instance.presentStudentsCount} anwesend`
                : ""}
            </span>
          )}
        </span>
      )}

      {!isCancelled && (absentCount > 0 || hasSubstitute) && (
        <span className="flex flex-wrap gap-1 text-[10px]">
          {absentCount > 0 && (
            <span className="font-semibold text-[#CC2626]">
              {absentCount} abwesend
            </span>
          )}
          {hasSubstitute && (
            <span className="font-semibold text-[#5A8E1F]">Ersatz</span>
          )}
        </span>
      )}
    </span>
  );

  return (
    <PlanBlock
      timeRange={`${instance.startTime} – ${instance.endTime}`}
      label={instance.title}
      color={edgeColor}
      tinted={!isUnderstaffedAck}
      // Sehr niedrige Blöcke (<=30px) einzeilig rendern — das zweizeilige
      // Default-Layout (Zeitzeile + Titel) würde in der Höhe abgeschnitten.
      size={isTiny ? "compact" : "default"}
      status={isCancelled ? "cancelled" : "default"}
      selected={isSelected}
      statusIcon={statusIcon}
      footer={footer}
      onClick={onClick}
      className="overflow-hidden"
      // Inline `position: absolute` schlägt die `relative`-Klasse der
      // PlanBlock-Basis; die Kalenderspalte positioniert den Block per Pixel.
      style={{
        position: "absolute",
        top: `${top}px`,
        height: `${Math.max(height, 18)}px`,
        left,
        width,
        // Ohne Fußzeile füllen Zeit und Titel den Block nicht aus: dann
        // vertikal zentrieren mit knapperem Padding, statt beide oben
        // anzuschlagen und den Titel an der Unterkante kleben zu lassen.
        ...(showFooter
          ? {}
          : {
              display: "flex",
              flexDirection: "column",
              justifyContent: "center",
              paddingTop: 2,
              paddingBottom: 2,
            }),
      }}
      aria-label={[
        `${instance.title}, ${instance.startTime} bis ${instance.endTime}`,
        ...(instance.planningTrackName
          ? [`Planungsspur ${instance.planningTrackName}`]
          : []),
        ...(showCompactStatus ? compactStatusDetails : []),
      ].join(", ")}
    />
  );
}
