"use client";

// Der Tag einer Lehrkraft, wenn gerade keine Aufsicht läuft (#2527).
//
// Eine Liste, keine Kacheln: nebeneinander liegende Karten mit je einem Knopf
// lesen sich als "beides gleichzeitig", und genau das kann eine Lehrkraft
// nicht. Auch kein Startknopf pro Zeile — man tippt die Aufsicht an und
// startet sie dort, wo man dann ohnehin arbeitet. Ein Weg, nicht zwei.

import { ChevronRight } from "lucide-react";
import { useSession } from "next-auth/react";
import { EmptyState } from "~/components/ui/empty-state";
import { SectionCard } from "~/components/ui/section-card";
import { StatCard } from "~/components/ui/stat-card";
import { StatusDotBadge } from "~/components/ui/status-dot-badge";
import { getUserDisplayName } from "~/lib/auth-utils";
import { formatDate } from "~/lib/date-helpers";
import { getTimeBasedGreeting } from "~/lib/greeting";
import { LOCATION_COLORS } from "~/lib/location-helper";
import type { PlannedTimetableInstance } from "~/lib/timetable-operations-types";
import { startProximityLabel, summarizeDay } from "./view-model";

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
  const cancelled = instance.status === "cancelled";
  const detail = cancelled
    ? room
    : `${room} · ${instance.expectedStudentsCount} Kinder erwartet`;
  const badge = STATUS_BADGES[instance.status];
  const proximity = startProximityLabel(instance);

  return (
    <li>
      <button
        type="button"
        onClick={onOpen}
        className="flex w-full items-center gap-3 border-b border-gray-100 px-4 py-3.5 text-left transition-colors last:border-b-0 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
      >
        <span className="w-[4.25rem] shrink-0 sm:w-24">
          <span className="block text-sm font-semibold text-gray-900 tabular-nums">
            {instance.startTime}
          </span>
          <span className="block text-xs text-gray-500 tabular-nums">
            bis {instance.endTime}
          </span>
        </span>
        <span className="min-w-0 flex-1">
          <span className="block truncate text-sm font-medium text-gray-900">
            {instance.title}
          </span>
          <span className="block truncate text-xs text-gray-500">{detail}</span>
          {proximity ? (
            <span className="block text-xs font-medium text-gray-600">
              {proximity}
            </span>
          ) : null}
          {/* Auf schmalen Geräten steht das Etikett unter dem Namen statt
              neben ihm: der Name ist das, wonach man sucht. */}
          {badge ? (
            <span className="mt-1 flex sm:hidden">
              <StatusDotBadge label={badge.label} color={badge.color} />
            </span>
          ) : null}
        </span>
        {instance.isSubstitute ? (
          <span className="hidden shrink-0 text-xs text-gray-500 lg:block">
            Vertretung
          </span>
        ) : null}
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
  today,
  onOpen,
}: Readonly<{
  instances: readonly PlannedTimetableInstance[];
  /** ISO-Datum des Tages, den die Liste zeigt. */
  today: string;
  onOpen: (id: string) => void;
}>) {
  const { data: session } = useSession();
  const summary = summarizeDay(instances);

  const description =
    instances.length === 0
      ? `Ihr Tag am ${formatDate(today)}.`
      : `Ihre Aufsichten am ${formatDate(today)}. Sie sehen nur die Termine, für die Sie im Betreuungsplan eingeteilt sind.`;

  return (
    <SectionCard
      kicker="Meine Aufsichten"
      title={`${getTimeBasedGreeting()}, ${getUserDisplayName(session)}`}
      description={description}
    >
      {instances.length === 0 ? (
        <EmptyState
          title="Heute keine Aufsicht für Sie"
          description="Für heute sind Sie im Betreuungsplan keiner Aufsicht zugeteilt. Die Einteilung macht das OGS-Büro."
        />
      ) : (
        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
            <StatCard
              variant="tile"
              label="Aufsichten heute"
              value={summary.count}
            />
            <StatCard
              variant="tile"
              label="Kinder insgesamt"
              value={summary.children}
            />
            <StatCard
              variant="tile"
              label={summary.running ? "Läuft gerade" : "Als Nächstes"}
              value={
                summary.running?.startTime ?? summary.next?.startTime ?? "—"
              }
            />
          </div>

          <div className="overflow-hidden rounded-2xl border border-gray-200 bg-white">
            <h2 className="border-b border-gray-100 bg-gray-50 px-4 py-2 text-sm font-semibold text-gray-900">
              Ihr Tag
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
          </div>
        </div>
      )}
    </SectionCard>
  );
}
