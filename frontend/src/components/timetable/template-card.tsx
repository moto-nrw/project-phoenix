"use client";

/**
 * TemplateCard — single recurring series tile.
 *
 * Replaces the legacy two-column list+aside in `template-list.tsx` with a
 * self-contained card that exposes the highest-signal info inline (weekday
 * pattern visualization, time, room, counts) plus two quick actions:
 * "Bearbeiten" (opens existing modal) and "Termine erzeugen" (materializes
 * the series' calendar period). Visual style follows shadcn conventions —
 * single border, no default shadow, brand accent reserved for the color
 * bar that ties the card back to the activity type.
 */

import { Archive, Palette, Users } from "lucide-react";
import { Button } from "~/components/ui/button";
import { SectionCard } from "~/components/ui/section-card";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { berlinTodayISO, formatDate } from "~/lib/date-helpers";
import { schoolClassLabel } from "~/lib/school-class-label";
import type { RosterDeviation } from "~/lib/roster-deviation";
import { getGermanWeekdayShort } from "~/lib/timetable-helpers";
import type { TimetableTemplate } from "~/lib/timetable-types";
import { RosterMaintenanceBadge } from "./roster-maintenance-badge";
import { SeriesRosterReview } from "./series-roster-review";
import { capacityTone, TimetableRatioPill } from "./timetable-ratio-pill";
import {
  TIMETABLE_NEUTRAL_COLOR,
  TIMETABLE_UNTYPED_EDGE_COLOR,
} from "./timetable-colors";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { careTimesConcept } from "~/lib/concepts/careTimes";
import { roomsConcept } from "~/lib/concepts/rooms";
import { supervisionConcept } from "~/lib/concepts/supervision";
import { StatusDot } from "~/components/ui/status-dot";
import { StatusColorBadge } from "~/components/ui/status-color-badge";

interface TemplateCardProps {
  template: TimetableTemplate;
  onEdit: (template: TimetableTemplate) => void;
  onApply: (template: TimetableTemplate) => void;
  onArchive: (template: TimetableTemplate) => void;
  /** Leseansicht (#2283): false blendet die Karten-Aktionen aus. */
  canManage?: boolean;
  /** Erkannte Abweichung von den Anmeldungen (#5103). */
  deviation?: RosterDeviation;
  onConnect?: (template: TimetableTemplate, deviation: RosterDeviation) => void;
  /** „Liste so lassen“ setzen oder zurücknehmen. */
  onSetManualRoster?: (template: TimetableTemplate, confirmed: boolean) => void;
}

const WEEKDAYS = [1, 2, 3, 4, 5] as const;

const TYPE_LABELS: Record<TimetableTemplate["type"], string> = {
  care: "Betreuung",
  activity: "AG",
  external: "Extern",
  duty: "Dienst",
};

function weekdayShort(iso: number): string {
  // 2024-01-01 was a Monday — anchor for getGermanWeekdayShort.
  const ref = new Date(2024, 0, 1);
  ref.setDate(ref.getDate() + (iso - 1));
  return getGermanWeekdayShort(ref);
}

function summarizeTimeRange(template: TimetableTemplate): string | null {
  if (template.schedules.length === 0) return null;
  // All schedules typically share the same time-of-day window; show the
  // first and append "+1" if any other schedule diverges.
  const first = template.schedules[0]!;
  const allSame = template.schedules.every(
    (s) => s.startTime === first.startTime && s.endTime === first.endTime,
  );
  const base = `${first.startTime} – ${first.endTime}`;
  return allSame ? base : `${base} +`;
}

function summarizeTargets(template: TimetableTemplate): string | null {
  const targets = template.targets ?? [];
  if (targets.length === 0) return null;
  const labels = targets.map((target) => {
    if (target.type === "jahrgang") return `Jahrgang ${target.gradeLevel}`;
    if (target.type === "klasse") {
      return schoolClassLabel(target.schoolClass ?? "") || "Klasse";
    }
    return target.educationGroupName ?? "Gruppe";
  });
  return labels.join(", ");
}

export function TemplateCard({
  template,
  onEdit,
  onApply,
  onArchive,
  canManage = true,
  deviation,
  onConnect,
  onSetManualRoster,
}: TemplateCardProps) {
  const color = template.planningTrackColor ?? TIMETABLE_UNTYPED_EDGE_COLOR;
  // Schichtfarbe kommt aus den Tenant-Daten; fehlt sie, steht dort das
  // neutrale Grau der Palette, nie eine erfundene Buntfarbe.
  const shiftTypeColor = template.shiftTypeColor || TIMETABLE_NEUTRAL_COLOR;
  const activeWeekdays = new Set(template.schedules.map((s) => s.weekday));
  const timeRange = summarizeTimeRange(template);
  const targetSummary = summarizeTargets(template);

  return (
    <SectionCard
      as="article"
      hoverElevated
      padding="none"
      overflow="visible"
      className="relative flex flex-col"
      // Der Rumpf wächst mit, damit „Termine erzeugen“ in einer Reihe
      // ungleich hoher Karten unten bündig bleibt (`mt-auto`).
      bodyClassName="flex flex-1 flex-col"
    >
      {/* The card itself must not clip: the roster-maintenance tooltip has to
          leave it. Only this layer clips, so the colour bar follows the card's
          rounded edge. It sits in the SectionCard body, not directly in the
          card, so it names the card radius instead of inheriting it. */}
      <div
        className="pointer-events-none absolute inset-0 overflow-hidden rounded-2xl"
        aria-hidden
      >
        <div className="h-full w-1" style={{ backgroundColor: color }} />
      </div>

      <div className="flex flex-col gap-3 p-4 pl-5">
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0 flex-1">
            <h3 className="truncate text-sm font-semibold text-gray-900">
              {template.name}
            </h3>
            <p className="mt-0.5 text-xs text-gray-500">
              {TYPE_LABELS[template.type]}
              {template.categoryName ? ` · ${template.categoryName}` : ""}
            </p>
            {template.rosterMaintenance &&
            template.type !== "duty" &&
            !deviation ? (
              <RosterMaintenanceBadge
                state={template.rosterMaintenance}
                className="mt-1 mr-1.5"
              />
            ) : null}
            {template.shiftTypeName ? (
              <span className="mt-1 inline-flex">
                <StatusColorBadge
                  label={template.shiftTypeName}
                  color={shiftTypeColor}
                  title={`Dienstart: ${template.shiftTypeName}`}
                  compact
                  wrap
                />
              </span>
            ) : null}
          </div>
          {canManage && (
            <OverflowMenu
              ariaLabel={`Aktionen für ${template.name}`}
              triggerSize="sm"
              items={[
                {
                  label: "Bearbeiten",
                  description: "Öffnet den Regeltermin zum Ändern",
                  onClick: () => onEdit(template),
                },
                {
                  label: "Archivieren",
                  icon: <Archive className="h-3.5 w-3.5" aria-hidden />,
                  description:
                    "Nimmt den Regeltermin aus der Liste. moto fragt vorher nach",
                  onClick: () => onArchive(template),
                },
              ]}
            />
          )}
        </div>

        {/* #5103: die Abweichung ersetzt die graue Plakette „Manuell“. Bleibt
            die Serie bewusst von Hand gepflegt, sagt die Karte das und
            bietet den Rückweg, wie die Termindetails. */}
        {template.type !== "duty" ? (
          <SeriesRosterReview
            deviation={deviation}
            manualRosterConfirmed={template.manualRosterConfirmed === true}
            className=""
            onConnect={
              canManage && onConnect
                ? (found) => onConnect(template, found)
                : undefined
            }
            onSetManualRoster={
              canManage && onSetManualRoster
                ? (confirmed) => onSetManualRoster(template, confirmed)
                : undefined
            }
          />
        ) : null}

        <div className="flex gap-1">
          {WEEKDAYS.map((wd) => {
            const active = activeWeekdays.has(wd);
            return (
              <div
                key={wd}
                className={`flex h-6 w-7 items-center justify-center rounded-full text-xs font-semibold ${
                  active
                    ? "bg-gray-900 text-white"
                    : "bg-gray-100 text-gray-400"
                }`}
              >
                {weekdayShort(wd)}
              </div>
            );
          })}
        </div>

        <dl className="space-y-1.5 text-xs text-gray-600">
          <div className="flex items-center gap-2">
            <MotoConceptIcon concept={careTimesConcept} size={14} />
            <span className="tabular-nums">
              {timeRange ?? "Keine Zeiten hinterlegt"}
              {/* #3594: eine Serie kann vor dem Planungszeitraum enden. Danach
                  bleibt sie in der Liste und heißt „beendet“. */}
              {template.endDate
                ? `, ${template.endDate < berlinTodayISO() ? "beendet am" : "bis"} ${formatDate(template.endDate)}`
                : ""}
            </span>
          </div>
          {targetSummary ? (
            <div className="flex items-start gap-2">
              <Users className="mt-0.5 h-3.5 w-3.5 text-gray-400" aria-hidden />
              <span className="line-clamp-2" title={targetSummary}>
                {targetSummary}
              </span>
            </div>
          ) : null}
          <div className="flex items-center gap-2">
            <MotoConceptIcon concept={roomsConcept} size={16} />
            <span className="truncate">{template.roomName ?? "Kein Raum"}</span>
          </div>
          <div className="flex items-center gap-2">
            <Palette className="h-3.5 w-3.5 text-gray-400" aria-hidden />
            <span className="truncate">
              {template.planningTrackName ?? "Keine Planungsspur"}
            </span>
          </div>
          <div className="flex items-center gap-2">
            <Users className="h-3.5 w-3.5 text-gray-400" aria-hidden />
            <span>
              {template.type === "duty" ? (
                "Dienst ohne Kinder"
              ) : (
                <>
                  {template.enrollmentCount}{" "}
                  {template.enrollmentCount === 1 ? "Kind" : "Kinder"}
                </>
              )}
              {" · "}
              {template.supervisorCount} Personal
            </span>
          </div>
          {template.requiredStaffCount > 0 && (
            <div className="flex items-center gap-2">
              <MotoConceptIcon concept={supervisionConcept} size={16} />
              <TimetableRatioPill
                icon={
                  <MotoConceptIcon concept={supervisionConcept} size={16} />
                }
                label="Besetzung"
                value={`${template.assignedStaffCount}/${template.requiredStaffCount}`}
                tone={capacityTone(
                  template.assignedStaffCount,
                  template.requiredStaffCount,
                )}
                variant="compact"
              />
            </div>
          )}
        </dl>

        {!template.roomId && template.type !== "duty" && (
          <p className="flex items-center gap-1.5 text-xs text-gray-600">
            <StatusDot className="bg-moto-amber" decorative />
            Raum fehlt – wird nicht eingeplant
          </p>
        )}
      </div>

      {canManage && (
        <div className="mt-auto border-t border-gray-100 px-4 py-2.5 pl-5">
          <Button
            type="button"
            variant="primary"
            size="compact"
            className="w-full"
            // Das Backend trägt alle Regeltermine des Zeitraums ein, nicht
            // nur diesen (POST /timetable/materialize kennt keinen Regeltermin).
            tooltip="Trägt die Termine aller Regeltermine im Planungszeitraum ein."
            onClick={() => onApply(template)}
          >
            Termine erzeugen
          </Button>
        </div>
      )}
    </SectionCard>
  );
}
