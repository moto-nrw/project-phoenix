"use client";

// Der Tag einer Lehrkraft, wenn gerade keine Aufsicht läuft (#2527).
//
// Eine Liste, keine Kacheln: nebeneinander liegende Karten mit je einem Knopf
// lesen sich als "beides gleichzeitig", und genau das kann eine Lehrkraft
// nicht. Auch kein Startknopf pro Zeile — man tippt die Aufsicht an und
// startet sie dort, wo man dann ohnehin arbeitet. Ein Weg, nicht zwei.

import { ChevronRight } from "lucide-react";
import { EmptyState } from "~/components/ui/empty-state";
import { StatusDotBadge } from "~/components/ui/status-dot-badge";
import { LOCATION_COLORS } from "~/lib/location-helper";
import type { PlannedTimetableInstance } from "~/lib/timetable-operations-types";

// Nur abweichende Zustände tragen ein Etikett. "Noch nicht gestartet" ist im
// Tagesüberblick der Normalfall — ihn zu beschriften, klebt an jede Zeile ein
// Schild, das nichts unterscheidet, und drängt auf schmalen Geräten den Namen
// heraus, auf den es ankommt.
const STATUS_BADGES: Partial<
  Record<PlannedTimetableInstance["status"], { label: string; color: string }>
> = {
  active: { label: "Läuft", color: LOCATION_COLORS.GROUP_ROOM },
  completed: { label: "Beendet", color: LOCATION_COLORS.OTHER_ROOM },
  // DANGER, nicht SICK: der Termin fällt aus, das Kind ist nicht krank.
  // Gleicher Hex, aber der Name muss sagen, was gemeint ist.
  cancelled: { label: "Fällt aus", color: LOCATION_COLORS.DANGER },
};

function SupervisionRow({
  instance,
  onOpen,
}: Readonly<{
  instance: PlannedTimetableInstance;
  onOpen: () => void;
}>) {
  const room = instance.roomName ?? `Raum ${instance.roomId}`;
  const detail =
    instance.status === "cancelled"
      ? room
      : `${room} · ${instance.expectedStudentsCount} Kinder erwartet`;
  const badge = STATUS_BADGES[instance.status];

  return (
    <li>
      <button
        type="button"
        onClick={onOpen}
        className="flex w-full items-center gap-3 border-b border-gray-100 px-4 py-3 text-left transition-colors last:border-b-0 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
      >
        <span className="w-12 shrink-0 text-sm font-semibold text-gray-900 tabular-nums">
          {instance.startTime}
        </span>
        <span className="min-w-0 flex-1">
          <span className="block truncate text-sm font-medium text-gray-900">
            {instance.title}
          </span>
          <span className="block truncate text-xs text-gray-500">{detail}</span>
          {/* Auf schmalen Geräten steht das Etikett unter dem Namen statt
              neben ihm: der Name ist das, wonach man sucht. */}
          {badge ? (
            <span className="mt-1 flex sm:hidden">
              <StatusDotBadge label={badge.label} color={badge.color} />
            </span>
          ) : null}
        </span>
        {badge ? (
          <span className="hidden shrink-0 sm:flex">
            <StatusDotBadge label={badge.label} color={badge.color} />
          </span>
        ) : null}
        <ChevronRight
          className="h-4 w-4 shrink-0 text-gray-400"
          aria-hidden="true"
        />
      </button>
    </li>
  );
}

export function SupervisionsOverview({
  instances,
  onOpen,
}: Readonly<{
  instances: readonly PlannedTimetableInstance[];
  onOpen: (id: string) => void;
}>) {
  if (instances.length === 0) {
    return (
      <section className="moto-content-surface rounded-2xl border p-5 shadow-sm backdrop-blur-md">
        <EmptyState
          title="Heute keine Aufsicht für Sie"
          description="Für heute sind Sie im Betreuungsplan keiner Aufsicht zugeteilt. Die Einteilung macht das OGS-Büro."
        />
      </section>
    );
  }

  return (
    <section className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md">
      <h2 className="border-b border-gray-100 px-4 py-3 text-sm font-semibold text-gray-900">
        Ihre Aufsichten heute
      </h2>
      <ul>
        {instances.map((instance) => (
          <SupervisionRow
            key={instance.id}
            instance={instance}
            onOpen={() => onOpen(instance.id)}
          />
        ))}
      </ul>
    </section>
  );
}
