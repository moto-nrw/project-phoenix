"use client";

import { useMemo, useState } from "react";
import { ChevronDown, CircleAlert, Play } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import type { MotoConceptKey } from "~/lib/moto-concepts";
import { LOCATION_COLORS, MOTO_COLOR_PALETTE } from "~/lib/location-helper";
import { isCareDayExpected } from "~/lib/timetable-types";
import { useShowTimetableCounts } from "~/lib/tenant-context";
import { useMinuteClock } from "~/lib/pickup-helpers";
import {
  canStartPlannedInstance,
  isPlannedStartExpired,
} from "~/lib/timetable-lifecycle";
import type {
  PlannedTimetableInstance,
  TimetableRosterRow,
} from "~/lib/timetable-operations-types";

interface PlannedNowSectionProps {
  readonly plannedNow: PlannedTimetableInstance[];
  readonly hasActiveTimetableSession?: boolean;
  readonly isStartingInstance: string | null;
  readonly onStart: (instance: PlannedTimetableInstance) => void;
}

const SOON_THRESHOLD_MINUTES = 15;

// Both numbers come from the backend, which counts only rows that are still
// expected AND scheduled today. Deriving them from the roster length would
// count sick, absent and already-present children as "erwartet" and contradict
// the Erwartet stat on the very same card (#1747 review).
function rosterPreviewLabel(
  rosterLength: number,
  expectedCount: number,
  notScheduledCount: number,
  showTimetableCounts: boolean,
): string {
  if (rosterLength === 0) return "keine Liste";
  if (!showTimetableCounts) return "Liste verfügbar";
  // Name the children the care plan leaves out (#1747) instead of quietly
  // showing a smaller number than the assignment list.
  if (notScheduledCount > 0) {
    return `${expectedCount} erwartet · ${notScheduledCount} heute nicht eingeplant`;
  }
  return `${expectedCount} erwartet`;
}

// Both non-expected verdicts count here — "not_scheduled" (never booked that
// weekday) and "cancelled" ("kommt heute nicht"). The backend leaves both out
// of expected_students_count, so testing one value would show a child as
// "Erwartet" that the header count does not include (#1747).
function isNotScheduled(row: TimetableRosterRow): boolean {
  return (
    !isCareDayExpected(row.careDayStatus) &&
    !row.currentlyPresent &&
    row.status !== "present"
  );
}

export function PlannedNowSection({
  plannedNow,
  hasActiveTimetableSession = false,
  isStartingInstance,
  onStart,
}: PlannedNowSectionProps) {
  const showTimetableCounts = useShowTimetableCounts();
  const now = useMinuteClock();
  const [expandedIds, setExpandedIds] = useState<Set<string>>(() => new Set());
  const [sectionExpanded, setSectionExpanded] = useState<boolean | null>(null);
  const sortedPlanned = useMemo(
    () =>
      [...plannedNow]
        .filter(
          (instance) => !isPlannedStartExpired(instance.startExpiresAt, now),
        )
        .sort((a, b) => {
          if (a.isOverdue !== b.isOverdue) return a.isOverdue ? -1 : 1;
          return a.minutesUntilStart - b.minutesUntilStart;
        }),
    [now, plannedNow],
  );

  if (sortedPlanned.length === 0) {
    return (
      <section className="moto-content-surface mb-4 rounded-2xl border p-4 shadow-sm backdrop-blur-md">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 className="text-base font-semibold text-gray-900">
              Als Nächstes
            </h2>
            <p className="mt-1 text-sm text-gray-600">
              Keine geplante Betreuung in Sicht
            </p>
          </div>
          <span className="inline-flex h-9 items-center gap-2 rounded-lg bg-gray-100 px-3 text-sm font-medium text-gray-600">
            <MotoConceptIcon concept="carePlan" size={18} />
            Heute
          </span>
        </div>
      </section>
    );
  }

  const toggleExpanded = (id: string) => {
    setExpandedIds((current) => {
      const next = new Set(current);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  const overdueCount = sortedPlanned.filter(
    (instance) => instance.isOverdue,
  ).length;
  const soonCount = sortedPlanned.filter(
    (instance) =>
      !instance.isOverdue &&
      instance.minutesUntilStart <= SOON_THRESHOLD_MINUTES,
  ).length;
  const startableCount = sortedPlanned.filter((instance) =>
    canStartPlannedInstance(instance, now),
  ).length;
  const expectedCount = sortedPlanned.reduce(
    (sum, instance) => sum + instance.expectedStudentsCount,
    0,
  );
  const hasActionableSlot =
    !hasActiveTimetableSession && (overdueCount > 0 || startableCount > 0);
  const isSectionExpanded = sectionExpanded ?? hasActionableSlot;

  return (
    <section className="moto-content-surface mb-5 overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md">
      <div className="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between sm:p-5">
        <button
          type="button"
          onClick={() =>
            setSectionExpanded((current) => !(current ?? hasActionableSlot))
          }
          className="group focus-visible:ring-moto-blue/30 flex min-w-0 items-center gap-3 text-left focus-visible:ring-2 focus-visible:outline-none"
          aria-expanded={isSectionExpanded}
        >
          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-gray-100">
            <MotoConceptIcon concept="carePlan" size={20} />
          </span>
          <span className="min-w-0">
            <span className="block text-base font-semibold text-gray-900">
              Als Nächstes
            </span>
            <span className="mt-0.5 block truncate text-sm text-gray-600">
              {sortedPlanned[0]?.title}
            </span>
          </span>
          <ChevronDown
            className={`h-4 w-4 shrink-0 text-gray-400 transition-transform group-hover:text-gray-600 ${isSectionExpanded ? "rotate-180" : ""}`}
            aria-hidden="true"
          />
        </button>
        <div className="flex flex-wrap gap-2 sm:justify-end">
          <SummaryPill
            concept="carePlan"
            label={`${sortedPlanned.length} geplant`}
            tone="info"
          />
          {showTimetableCounts && (
            <SummaryPill concept="children" label={`${expectedCount} Kinder`} />
          )}
          {overdueCount > 0 ? (
            <SummaryPill
              concept="emergency"
              label={`${overdueCount} überfällig`}
              tone="warning"
            />
          ) : soonCount > 0 ? (
            <SummaryPill
              concept="careTimes"
              label={`${soonCount} startet gleich`}
              tone="success"
            />
          ) : null}
        </div>
      </div>

      {isSectionExpanded ? (
        <div className="overflow-hidden">
          <div className="grid gap-3 border-t border-gray-100 bg-gray-50/50 p-4 sm:p-5 xl:grid-cols-2">
            {sortedPlanned.map((instance) => {
              const isExpanded = expandedIds.has(instance.id);
              const visibleRows = isExpanded
                ? instance.rosterPreview
                : instance.rosterPreview.slice(0, 4);
              const hiddenCount = Math.max(
                0,
                instance.rosterPreview.length - visibleRows.length,
              );

              return (
                <article
                  key={instance.id}
                  className="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-sm"
                >
                  <div className="p-4">
                    <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                      <div className="min-w-0">
                        <div className="flex flex-wrap items-center gap-2">
                          <h3 className="truncate text-sm font-semibold text-gray-900">
                            {instance.title}
                          </h3>
                          <SlotStatusBadge instance={instance} />
                          <ResponsibilityBadge instance={instance} />
                        </div>
                        <div className="mt-2 flex flex-wrap gap-3 text-sm text-gray-600">
                          <span className="inline-flex items-center gap-1.5">
                            <MotoConceptIcon concept="careTimes" size={16} />
                            {instance.startTime}-{instance.endTime}
                          </span>
                          <span className="inline-flex items-center gap-1.5">
                            <MotoConceptIcon concept="rooms" size={18} />
                            {instance.roomName ?? `Raum ${instance.roomId}`}
                          </span>
                        </div>
                      </div>

                      <button
                        type="button"
                        disabled={
                          isStartingInstance === instance.id ||
                          !canStartPlannedInstance(instance, now)
                        }
                        onClick={() => onStart(instance)}
                        className="inline-flex h-9 shrink-0 items-center justify-center gap-2 rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
                      >
                        <Play className="h-4 w-4" aria-hidden="true" />
                        {isStartingInstance === instance.id
                          ? "Startet..."
                          : canStartPlannedInstance(instance, now)
                            ? "Starten"
                            : instance.startAvailableAt
                              ? `Starten ab ${new Intl.DateTimeFormat("de-DE", { hour: "2-digit", minute: "2-digit" }).format(new Date(instance.startAvailableAt))}`
                              : "Noch nicht verfügbar"}
                      </button>
                    </div>

                    <div
                      className={`mt-4 grid gap-2 ${showTimetableCounts ? "grid-cols-3" : "grid-cols-1"}`}
                    >
                      {showTimetableCounts && (
                        <>
                          <SlotStat
                            label="Erwartet"
                            value={instance.expectedStudentsCount}
                            tone="neutral"
                          />
                          <SlotStat
                            label="Anwesend"
                            value={instance.presentStudentsCount}
                            tone={
                              instance.presentStudentsCount > 0
                                ? "success"
                                : "neutral"
                            }
                          />
                        </>
                      )}
                      <SlotStat
                        label="Betreuende"
                        value={instance.assignedStaffIds.length}
                        tone={instance.isAssigned ? "info" : "neutral"}
                      />
                    </div>
                  </div>

                  <div className="border-t border-gray-100 bg-gray-50/70 px-4 py-3">
                    <button
                      type="button"
                      onClick={() => toggleExpanded(instance.id)}
                      className="flex w-full items-center justify-between gap-3 text-left text-sm font-medium text-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                      aria-expanded={isExpanded}
                    >
                      <span className="inline-flex items-center gap-2">
                        <MotoConceptIcon concept="children" size={18} />
                        Kinder
                        <span className="text-xs font-normal text-gray-500">
                          {rosterPreviewLabel(
                            instance.rosterPreview.length,
                            instance.expectedStudentsCount,
                            instance.notScheduledStudentsCount,
                            showTimetableCounts,
                          )}
                        </span>
                      </span>
                      <ChevronDown
                        className={`h-4 w-4 text-gray-400 transition-transform ${isExpanded ? "rotate-180" : ""}`}
                        aria-hidden="true"
                      />
                    </button>

                    {instance.rosterPreview.length > 0 ? (
                      <div className="mt-3 space-y-2">
                        {visibleRows.map((row) => (
                          <RosterPreviewRow key={row.studentId} row={row} />
                        ))}
                        {hiddenCount > 0 ? (
                          <button
                            type="button"
                            onClick={() => toggleExpanded(instance.id)}
                            className="focus-visible:ring-moto-blue/30 text-moto-blue-hover hover:text-moto-blue-strong text-xs font-medium focus-visible:ring-2 focus-visible:outline-none"
                          >
                            {showTimetableCounts
                              ? `${hiddenCount} weitere anzeigen`
                              : "Weitere anzeigen"}
                          </button>
                        ) : null}
                      </div>
                    ) : null}
                  </div>
                </article>
              );
            })}
          </div>
        </div>
      ) : null}
    </section>
  );
}

function SlotStatusBadge({
  instance,
}: Readonly<{ instance: PlannedTimetableInstance }>) {
  if (instance.isOverdue) {
    return (
      <span className="bg-moto-amber/20 text-moto-amber-strong rounded-full px-2 py-0.5 text-xs font-medium">
        Überfällig
      </span>
    );
  }
  if (instance.minutesUntilStart <= SOON_THRESHOLD_MINUTES) {
    return (
      <span className="bg-moto-green/15 text-moto-green-strong rounded-full px-2 py-0.5 text-xs font-medium">
        Startet gleich
      </span>
    );
  }
  return (
    <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600">
      Heute
    </span>
  );
}

/**
 * Mirrors the label ranking below field for field. Branching on a different
 * field than the label does leaves "Zuständig" wearing the same icon as
 * "Zugewiesen" and "Info", which is what happened when this moved from a
 * ShieldCheck/UserCheck pair to the concept system.
 */
function responsibilityConcept(
  instance: PlannedTimetableInstance,
): MotoConceptKey {
  if (instance.isPrimary) return "responsibility";
  if (instance.isSubstitute) return "substitution";
  return "supervision";
}

function ResponsibilityBadge({
  instance,
}: Readonly<{ instance: PlannedTimetableInstance }>) {
  const label = instance.isPrimary
    ? "Zuständig"
    : instance.isSubstitute
      ? "Vertretung"
      : instance.isAssigned
        ? "Zugewiesen"
        : "Info";
  const className = instance.isAssigned
    ? "bg-moto-blue/10 text-moto-blue-hover"
    : "bg-gray-100 text-gray-500";
  return (
    <span
      className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium ${className}`}
    >
      <MotoConceptIcon concept={responsibilityConcept(instance)} size={16} />
      {label}
    </span>
  );
}

function SlotStat({
  label,
  value,
  tone,
}: Readonly<{
  label: string;
  value: number;
  tone: "neutral" | "success" | "info";
}>) {
  const className =
    tone === "success"
      ? "bg-moto-green/10 text-moto-green-strong"
      : tone === "info"
        ? "bg-moto-blue/10 text-moto-blue-hover"
        : "bg-gray-50 text-gray-900";
  return (
    <div className={`rounded-lg px-3 py-2 ${className}`}>
      <span className="block text-sm font-semibold">{value}</span>
      <span className="block text-[11px] font-medium text-gray-500">
        {label}
      </span>
    </div>
  );
}

function SummaryPill({
  concept,
  label,
  tone = "neutral",
}: Readonly<{
  concept?: MotoConceptKey;
  label: string;
  tone?: "neutral" | "info" | "success" | "warning";
}>) {
  const className =
    tone === "info"
      ? "bg-moto-blue/10 text-moto-blue-hover"
      : tone === "success"
        ? "bg-moto-green/10 text-moto-green-strong"
        : tone === "warning"
          ? "bg-moto-amber/20 text-moto-amber-strong"
          : "bg-gray-100 text-gray-600";
  return (
    <span
      className={`inline-flex h-8 items-center gap-2 rounded-lg px-3 text-sm font-medium ${className}`}
    >
      {concept ? <MotoConceptIcon concept={concept} size={18} /> : null}
      {label}
    </span>
  );
}

function RosterPreviewRow({ row }: Readonly<{ row: TimetableRosterRow }>) {
  const warnings = row.warnings ?? [];
  const warningLabel =
    warnings.length > 0
      ? warnings.map((warning) => warning.message).join("\n")
      : null;
  // Set apart, never hidden: a child who turns up anyway must still be one tap
  // away from a check-in (#1747).
  const notScheduled = isNotScheduled(row);
  return (
    <div
      className={`rounded-lg px-3 py-2 text-sm shadow-[0_1px_0_rgba(17,24,39,0.04)] ${
        notScheduled ? "bg-gray-50" : "bg-white"
      }`}
    >
      <div className="flex items-center justify-between gap-3">
        <div className="min-w-0">
          <p
            className={`truncate font-medium ${
              notScheduled ? "text-gray-500" : "text-gray-900"
            }`}
          >
            {row.studentName || `Kind ${row.studentId}`}
          </p>
          <p className="mt-0.5 truncate text-xs text-gray-500">
            {[row.schoolClass, row.groupName].filter(Boolean).join(" · ") ||
              "Ohne Klassengruppe"}
          </p>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {warningLabel ? (
            <span
              className="bg-moto-amber/20 text-moto-amber-strong inline-flex h-7 w-7 items-center justify-center rounded-full"
              title={warningLabel}
              aria-label={`${warnings.length} Planungs-Hinweis`}
            >
              <CircleAlert className="h-4 w-4" aria-hidden="true" />
            </span>
          ) : null}
          <span className="inline-flex items-center gap-1.5 rounded-full bg-gray-50 px-2 py-1 text-xs font-medium text-gray-600">
            <span
              className="h-2.5 w-2.5 rounded-full"
              style={{ backgroundColor: rosterDotColor(row) }}
              aria-hidden="true"
            />
            {rosterStatusLabel(row)}
          </span>
        </div>
      </div>
      {warnings.length > 0 ? (
        <p className="text-moto-amber-strong mt-2 text-xs">
          {warnings[0]?.message}
        </p>
      ) : null}
    </div>
  );
}

/**
 * The "Erwartet" dot is the one roster state with no standing in the location
 * palette — it says "nothing has happened yet", so it takes the light neutral
 * from the shared palette rather than borrowing a status hue. Never a raw hex:
 * status and planning colours come from the tokens only.
 */
const EXPECTED_DOT_COLOR = MOTO_COLOR_PALETTE.neutral.light;

function rosterDotColor(row: TimetableRosterRow) {
  if (row.currentlyPresent || row.status === "present") {
    return LOCATION_COLORS.GROUP_ROOM;
  }
  // The care-day verdict wins over "absent": a child the care plan leaves out
  // today is reported as absent by the status-day layer, but it is not a real
  // absence and must match the "nicht eingeplant" count in the header (#1747).
  if (isNotScheduled(row)) {
    return LOCATION_COLORS.UNKNOWN;
  }
  if (row.status === "absent") {
    return LOCATION_COLORS.DANGER;
  }
  // "Erwartet" — deliberately NOT LOCATION_COLORS.UNKNOWN, which the
  // isNotScheduled branch above already owns. Sharing the hex would make a
  // child that is merely expected indistinguishable from one the care plan
  // leaves out today.
  return EXPECTED_DOT_COLOR;
}

function rosterStatusLabel(row: TimetableRosterRow) {
  if (row.currentlyPresent || row.status === "present") return "Anwesend";
  if (isNotScheduled(row)) {
    return row.careDayStatus === "cancelled"
      ? "Abgemeldet"
      : "Nicht eingeplant";
  }
  if (row.status === "absent") return "Abwesend";
  return "Erwartet";
}
