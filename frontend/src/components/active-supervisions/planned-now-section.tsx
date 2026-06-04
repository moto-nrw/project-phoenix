"use client";

import { useMemo, useState } from "react";
import {
  CalendarClock,
  ChevronDown,
  MapPin,
  Play,
  ShieldCheck,
  UserCheck,
  Users,
} from "lucide-react";
import { LOCATION_COLORS } from "~/lib/location-helper";
import type {
  PlannedTimetableInstance,
  TimetableRosterRow,
} from "~/lib/timetable-operations-types";

interface PlannedNowSectionProps {
  readonly plannedNow: PlannedTimetableInstance[];
  readonly isStartingInstance: string | null;
  readonly onStart: (instance: PlannedTimetableInstance) => void;
}

const SOON_THRESHOLD_MINUTES = 15;

export function PlannedNowSection({
  plannedNow,
  isStartingInstance,
  onStart,
}: PlannedNowSectionProps) {
  const [expandedIds, setExpandedIds] = useState<Set<string>>(() => new Set());
  const sortedPlanned = useMemo(
    () =>
      [...plannedNow].sort((a, b) => {
        if (a.isOverdue !== b.isOverdue) return a.isOverdue ? -1 : 1;
        return a.minutesUntilStart - b.minutesUntilStart;
      }),
    [plannedNow],
  );

  if (sortedPlanned.length === 0) {
    return (
      <section className="moto-content-surface mb-4 rounded-2xl border p-4 shadow-sm backdrop-blur-md">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
              Als Nächstes
            </p>
            <h2 className="mt-1 text-base font-semibold text-gray-900">
              Keine geplante Betreuung in Sicht
            </h2>
            <p className="mt-1 text-sm text-gray-600">
              Zugewiesene Betreuungsslots erscheinen hier, sobald sie im
              heutigen Zeitfenster liegen.
            </p>
          </div>
          <span className="inline-flex h-9 items-center gap-2 rounded-lg bg-gray-100 px-3 text-sm font-medium text-gray-600">
            <CalendarClock className="h-4 w-4" aria-hidden="true" />
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

  return (
    <section className="moto-content-surface mb-5 rounded-2xl border p-4 shadow-sm backdrop-blur-md sm:p-5">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
            Als Nächstes
          </p>
          <h2 className="mt-1 text-base font-semibold text-gray-900">
            Anstehende Betreuung
          </h2>
        </div>
        <span className="inline-flex h-8 items-center gap-2 rounded-lg bg-[#5080D8]/10 px-3 text-sm font-medium text-[#4070C8]">
          <CalendarClock className="h-4 w-4" aria-hidden="true" />
          {sortedPlanned.length} geplant
        </span>
      </div>

      <div className="mt-4 grid gap-3 xl:grid-cols-2">
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
              className="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm"
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
                        <CalendarClock
                          className="h-4 w-4 text-gray-400"
                          aria-hidden="true"
                        />
                        {instance.startTime}-{instance.endTime}
                      </span>
                      <span className="inline-flex items-center gap-1.5">
                        <MapPin
                          className="h-4 w-4 text-gray-400"
                          aria-hidden="true"
                        />
                        {instance.roomName ?? `Raum ${instance.roomId}`}
                      </span>
                    </div>
                  </div>

                  <button
                    type="button"
                    disabled={isStartingInstance === instance.id}
                    onClick={() => onStart(instance)}
                    className="inline-flex h-9 shrink-0 items-center justify-center gap-2 rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    <Play className="h-4 w-4" aria-hidden="true" />
                    {isStartingInstance === instance.id
                      ? "Startet..."
                      : "Jetzt starten"}
                  </button>
                </div>

                <div className="mt-4 grid grid-cols-3 gap-2">
                  <SlotStat
                    label="Erwartet"
                    value={instance.expectedStudentsCount}
                  />
                  <SlotStat
                    label="Anwesend"
                    value={instance.presentStudentsCount}
                  />
                  <SlotStat
                    label="Betreuende"
                    value={instance.assignedStaffIds.length}
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
                    <Users
                      className="h-4 w-4 text-gray-400"
                      aria-hidden="true"
                    />
                    Zugeordnete Kinder
                    <span className="text-xs font-normal text-gray-500">
                      {instance.rosterPreview.length > 0
                        ? `${instance.rosterPreview.length} sichtbar`
                        : "Liste nicht verfügbar"}
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
                        className="text-xs font-medium text-[#4070C8] hover:text-[#305FAE] focus-visible:ring-2 focus-visible:ring-[#5080D8]/30 focus-visible:outline-none"
                      >
                        {hiddenCount} weitere anzeigen
                      </button>
                    ) : null}
                  </div>
                ) : (
                  <p className="mt-2 text-sm text-gray-500">
                    Kinderliste für diesen Slot nicht verfügbar.
                  </p>
                )}
              </div>
            </article>
          );
        })}
      </div>
    </section>
  );
}

function SlotStatusBadge({
  instance,
}: Readonly<{ instance: PlannedTimetableInstance }>) {
  if (instance.isOverdue) {
    return (
      <span className="rounded-full bg-[#F3B63F]/20 px-2 py-0.5 text-xs font-medium text-[#A66F00]">
        Überfällig
      </span>
    );
  }
  if (instance.minutesUntilStart <= SOON_THRESHOLD_MINUTES) {
    return (
      <span className="rounded-full bg-[#83CD2D]/15 px-2 py-0.5 text-xs font-medium text-[#5A8B1F]">
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

function ResponsibilityBadge({
  instance,
}: Readonly<{ instance: PlannedTimetableInstance }>) {
  const label = instance.isPrimary
    ? "Primär"
    : instance.isSubstitute
      ? "Vertretung"
      : instance.isAssigned
        ? "Zugewiesen"
        : "Info";
  const className = instance.isAssigned
    ? "bg-[#5080D8]/10 text-[#4070C8]"
    : "bg-gray-100 text-gray-500";
  const Icon = instance.isPrimary ? ShieldCheck : UserCheck;
  return (
    <span
      className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium ${className}`}
    >
      <Icon className="h-3.5 w-3.5" aria-hidden="true" />
      {label}
    </span>
  );
}

function SlotStat({
  label,
  value,
}: Readonly<{ label: string; value: number }>) {
  return (
    <div className="rounded-lg bg-gray-50 px-3 py-2">
      <span className="block text-sm font-semibold text-gray-900">{value}</span>
      <span className="block text-[11px] font-medium text-gray-500">
        {label}
      </span>
    </div>
  );
}

function RosterPreviewRow({ row }: Readonly<{ row: TimetableRosterRow }>) {
  return (
    <div className="flex items-center justify-between gap-3 rounded-lg bg-white px-3 py-2 text-sm">
      <div className="min-w-0">
        <p className="truncate font-medium text-gray-900">
          {row.studentName || `Schüler ${row.studentId}`}
        </p>
        <p className="mt-0.5 truncate text-xs text-gray-500">
          {[row.schoolClass, row.groupName].filter(Boolean).join(" · ") ||
            "Ohne Klassengruppe"}
        </p>
      </div>
      <span
        className="h-2.5 w-2.5 shrink-0 rounded-full"
        style={{ backgroundColor: rosterDotColor(row) }}
        aria-label={rosterStatusLabel(row)}
      />
    </div>
  );
}

function rosterDotColor(row: TimetableRosterRow) {
  if (row.currentlyPresent || row.status === "present") {
    return LOCATION_COLORS.GROUP_ROOM;
  }
  if (row.status === "absent") {
    return LOCATION_COLORS.HOME;
  }
  return "#D1D5DB";
}

function rosterStatusLabel(row: TimetableRosterRow) {
  if (row.currentlyPresent || row.status === "present") return "Anwesend";
  if (row.status === "absent") return "Abwesend";
  return "Erwartet";
}
