// components/rooms/room-plan-tab.tsx
//
// Reiter „Plan“ der Raumseite /rooms/[id] (#4768): welche Blöcke laut
// Betreuungsplan in diesem Raum stattfinden und wann er frei ist, für einen
// Tag oder eine Woche. Nur Anzeige; geändert wird der Plan im Betreuungsplan.
// Die Bedienleiste (Datum, Tag/Woche) trägt die Kopfkarte der Seite, siehe
// `RoomPlanControls`.

"use client";

import { useMemo } from "react";

import { ButtonLink } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { PlanningContextBar } from "~/components/ui/planning-context-bar";
import { SectionCard } from "~/components/ui/section-card";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { Skeleton } from "~/components/ui/skeleton";
import { StatusBadge } from "~/components/ui/status-badge";
import { formatWeekLabel, parseISODate } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useTimetableDayHours } from "~/lib/hooks/use-timetable-day-hours";
import {
  roomDayPlan,
  roomPlanCareDay,
  roomPlanDayLabel,
  roomPlanStepDay,
  roomPlanStepWeek,
  roomPlanWeekDays,
  type RoomPlanBlock,
  type RoomPlanRow,
} from "~/lib/room-plan";
import { fetchRoomPlanBlocks } from "~/lib/room-plan-api";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";
import { useWeekendFollowsFriday } from "~/lib/tenant-context";
import type { RoomPlanView } from "./room-plan-navigation";

export type { RoomPlanView } from "./room-plan-navigation";

/** Die angezeigten Tage: ein Betreuungstag oder dessen Woche. */
function useRoomPlanDays(view: RoomPlanView, anchorISO: string) {
  const weekendOpen = useWeekendFollowsFriday();
  const todayISO = useBerlinToday();
  const dayISO = roomPlanCareDay(anchorISO, weekendOpen);
  return useMemo(() => {
    if (view === "tag") {
      return { dayISO, days: [dayISO], todayISO, weekendOpen };
    }
    return {
      dayISO,
      days: roomPlanWeekDays(dayISO, weekendOpen, todayISO),
      todayISO,
      weekendOpen,
    };
  }, [dayISO, todayISO, view, weekendOpen]);
}
/** SWR-Schlüssel der Blöcke eines Raums im angezeigten Zeitraum. */
function roomPlanKey(roomId: string, from: string, to: string): string {
  return `room-plan-${roomId}-${from}-${to}`;
}

interface RoomPlanControlsProps {
  readonly view: RoomPlanView;
  readonly anchorISO: string;
  readonly onChange: (next: { view?: RoomPlanView; d?: string }) => void;
  /** Ziel „Im Betreuungsplan …“ mit Raumfilter; null ohne Zugang. */
  readonly planHref: string | null;
  readonly planLinkLabel: string;
}

/**
 * Bedienband des Reiters in der Kopfkarte: Zeitraum, Zurück/Heute/Weiter,
 * Tag/Woche und der Weg in den Betreuungsplan, wo der Plan geändert wird.
 */
export function RoomPlanControls({
  view,
  anchorISO,
  onChange,
  planHref,
  planLinkLabel,
}: RoomPlanControlsProps) {
  const { dayISO, days, todayISO, weekendOpen } = useRoomPlanDays(
    view,
    anchorISO,
  );
  const todayTarget = roomPlanCareDay(todayISO, weekendOpen);
  const showsToday = days.includes(todayTarget);
  const label =
    view === "tag"
      ? roomPlanDayLabel(dayISO)
      : formatWeekLabel(
          parseISODate(days[0]!),
          parseISODate(days[days.length - 1]!),
        );

  const step = (direction: 1 | -1) => {
    if (view === "tag") {
      onChange({ d: roomPlanStepDay(dayISO, direction, weekendOpen) });
      return;
    }
    onChange({ d: roomPlanStepWeek(days[0]!, direction) });
  };

  return (
    <PlanningContextBar
      dateLabel={label}
      onPrevious={() => step(-1)}
      onNext={() => step(1)}
      previousLabel={view === "tag" ? "Vorheriger Tag" : "Vorherige Woche"}
      nextLabel={view === "tag" ? "Nächster Tag" : "Nächste Woche"}
      onToday={showsToday ? undefined : () => onChange({ d: todayTarget })}
      viewSwitcher={
        <SegmentedControl
          ariaLabel="Ansicht"
          value={view}
          onChange={(next) => onChange({ view: next as RoomPlanView })}
          items={[
            { value: "tag", label: "Tag" },
            { value: "woche", label: "Woche" },
          ]}
        />
      }
      withoutContextRow={planHref === null}
    >
      {planHref && (
        <ButtonLink variant="outline" size="compact" href={planHref}>
          {planLinkLabel}
        </ButtonLink>
      )}
    </PlanningContextBar>
  );
}
interface RoomPlanTabProps {
  readonly roomId: string;
  readonly view: RoomPlanView;
  readonly anchorISO: string;
}

/** Inhalt des Reiters: je Tag die Blöcke des Raums und die freien Zeiten. */
export function RoomPlanTab({ roomId, view, anchorISO }: RoomPlanTabProps) {
  const { days } = useRoomPlanDays(view, anchorISO);
  const { dayStartTime, dayEndTime } = useTimetableDayHours();
  const tenantMutate = useTenantMutate();
  const from = days[0]!;
  const to = days[days.length - 1]!;
  const key = roomPlanKey(roomId, from, to);
  const { data, error, isLoading } = useSWRAuth(key, () =>
    fetchRoomPlanBlocks(roomId, from, to),
  );
  const loadError = useSwrLoadError(error, "den Plan des Raums", () =>
    tenantMutate(key),
  );

  const byDay = useMemo(() => {
    const map = new Map<string, RoomPlanBlock[]>();
    for (const instance of data ?? []) {
      const list = map.get(instance.date) ?? [];
      list.push(instance);
      map.set(instance.date, list);
    }
    return map;
  }, [data]);

  if (error && !data) {
    return (
      <SectionCard>
        <LoadErrorAlert error={loadError} />
      </SectionCard>
    );
  }

  return (
    <SectionCard
      title="Belegung laut Betreuungsplan"
      description="Wann der Raum laut Plan belegt ist und wann er frei ist. Abgesagte Termine belegen den Raum nicht."
      testId="room-plan"
    >
      {isLoading && !data ? (
        <div className="space-y-3" aria-hidden>
          <Skeleton className="h-5 w-48" />
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
        </div>
      ) : (
        <div className="space-y-6">
          {error && <LoadErrorAlert error={loadError} />}
          {days.map((day) => (
            <RoomPlanDay
              key={day}
              dayISO={day}
              showHeading={view === "woche"}
              rows={roomDayPlan(byDay.get(day) ?? [], dayStartTime, dayEndTime)}
            />
          ))}
        </div>
      )}
    </SectionCard>
  );
}

function RoomPlanDay({
  dayISO,
  showHeading,
  rows,
}: {
  readonly dayISO: string;
  readonly showHeading: boolean;
  readonly rows: readonly RoomPlanRow[];
}) {
  const label = roomPlanDayLabel(dayISO);
  return (
    <section aria-label={label} data-testid={`room-plan-day-${dayISO}`}>
      {showHeading && (
        <h3 className="mb-2 text-sm font-semibold text-gray-900">{label}</h3>
      )}
      {rows.length === 0 ? (
        <EmptyState variant="compact" title="Keine Planungszeit" />
      ) : (
        <ul className="moto-content-surface divide-y divide-gray-100 overflow-hidden rounded-xl border shadow-sm">
          {rows.map((row) =>
            row.kind === "free" ? (
              <li
                key={`free-${row.start}`}
                className="flex items-center gap-3 px-4 py-2.5 text-sm text-gray-500"
              >
                <span className="w-28 shrink-0 tabular-nums">
                  {row.start}–{row.end}
                </span>
                <span>Frei</span>
              </li>
            ) : (
              <RoomPlanBlockRow key={row.instance.id} instance={row.instance} />
            ),
          )}
        </ul>
      )}
    </section>
  );
}

function RoomPlanBlockRow({ instance }: { readonly instance: RoomPlanBlock }) {
  const cancelled = instance.status === "cancelled";
  return (
    <li
      className="flex flex-wrap items-center gap-x-3 gap-y-1 px-4 py-2.5 text-sm"
      data-testid={`room-plan-block-${instance.id}`}
    >
      <span
        className={`w-28 shrink-0 tabular-nums ${cancelled ? "text-gray-400 line-through" : "font-medium text-gray-900"}`}
      >
        {instance.startTime}–{instance.endTime}
      </span>
      <span
        className={`min-w-0 flex-1 ${cancelled ? "text-gray-500 line-through" : "text-gray-900"}`}
      >
        {instance.title}
        {instance.groupName ? (
          <span className="text-gray-500"> · {instance.groupName}</span>
        ) : null}
      </span>
      {cancelled ? (
        <StatusBadge
          label="Abgesagt"
          tone="gray"
          title={instance.cancelReason}
        />
      ) : null}
    </li>
  );
}
