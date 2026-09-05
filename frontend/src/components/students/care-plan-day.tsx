"use client";

/**
 * CarePlanDayTimeline — one child's care plan for a single day, rendered as a
 * vertical timeline: arrival anchor → time-ordered activity blocks with the
 * free-care (Freispiel) bands that fill the gaps between them → pickup anchor.
 * A deviation banner (krank / entschuldigt / Klassenfahrt / Absage) sits on top
 * and visually overrides the plan.
 *
 * Read-only. The data comes from GET /api/timetable/student/{id}/{day,week}
 * (student-care-plan-api.ts); free-care bands and the deviation are composed in
 * care-plan-helpers.ts from data the student detail page already holds.
 */

import { Ban, Check, LogIn, LogOut } from "lucide-react";

import {
  computeFreeCareBands,
  type DayDeviation,
  type FreeCareBand,
} from "~/lib/care-plan-helpers";
import { LOCATION_COLORS } from "~/lib/location-helper";
import type {
  CarePlanDay,
  CarePlanInstance,
  CarePlanSlot,
} from "~/lib/student-care-plan-api";
import { parseTimeToMinutes } from "~/lib/timetable-helpers";

interface CarePlanDayTimelineProps {
  readonly day: CarePlanDay | null;
  readonly deviation: DayDeviation | null;
  readonly compact?: boolean;
  /** Cross-link to the Betreuungszeiten editor (day view only). */
  readonly onEditSchedule?: () => void;
}

type TimelineItem =
  | { kind: "instance"; startMin: number; instance: CarePlanInstance }
  | { kind: "free"; startMin: number; band: FreeCareBand };

const DEVIATION_COLORS: Record<DayDeviation["kind"], string> = {
  sick: LOCATION_COLORS.SICK, // amber
  excused: LOCATION_COLORS.EXCUSED, // purple
  class_trip: LOCATION_COLORS.CLASS_TRIP, // blue
};

/** A cancelled block frees its slot instead of occupying it. */
function isCancelled(instance: CarePlanInstance): boolean {
  return instance.status === "cancelled";
}

/**
 * Combine activity blocks and free-care bands into one time-sorted list.
 *
 * Cancelled blocks are deliberately NOT part of it: computeFreeCareBands treats
 * their slot as free (isOccupying in care-plan-helpers.ts), so a cancelled
 * 12:00–13:00 activity already appears as a 12:00–13:00 "Freie Betreuung" band.
 * Listing the block chronologically as well put two contradictory entries over
 * the same minutes. The cancellation is still shown — see the separate section
 * the component renders below the timeline.
 */
function buildTimeline(day: CarePlanDay): TimelineItem[] {
  const bands = computeFreeCareBands(day);
  const items: TimelineItem[] = [
    ...day.instances
      .filter((instance) => !isCancelled(instance))
      .map((instance): TimelineItem => ({
        kind: "instance",
        startMin: parseTimeToMinutes(instance.startTime),
        instance,
      })),
    ...bands.map((band): TimelineItem => ({
      kind: "free",
      startMin: parseTimeToMinutes(band.startTime),
      band,
    })),
  ];
  return items
    .filter((i) => !Number.isNaN(i.startMin))
    .sort((a, b) => a.startMin - b.startMin);
}

function DeviationBanner({ deviation }: { readonly deviation: DayDeviation }) {
  const color = DEVIATION_COLORS[deviation.kind];
  return (
    <div
      className="mb-3 rounded-lg border px-3 py-2 text-sm font-semibold"
      style={{
        borderColor: `${color}40`,
        backgroundColor: `${color}14`,
        color,
      }}
    >
      {deviation.label}
      {deviation.note ? (
        <span className="ml-1 font-normal text-gray-600">
          · {deviation.note}
        </span>
      ) : null}
    </div>
  );
}

/** Arrival / pickup anchor. Handles schedule, exception (with reason), absence. */
function SlotAnchor({
  slot,
  label,
  icon,
}: {
  readonly slot: CarePlanSlot;
  readonly label: string;
  readonly icon: "in" | "out";
}) {
  const Icon = icon === "in" ? LogIn : LogOut;
  const isException = slot.source === "exception";
  const timeText = slot.expectedTime ?? "–";
  return (
    <div className="flex items-center gap-2 py-1.5">
      <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-gray-100 text-gray-500">
        <Icon className="h-4 w-4" aria-hidden="true" />
      </span>
      <div className="min-w-0 text-sm">
        <span className="font-semibold text-gray-900">{label}</span>{" "}
        <span className="text-gray-700 tabular-nums">{timeText}</span>
        {isException ? (
          <span className="ml-1 rounded-full bg-gray-100 px-1.5 py-0.5 text-xs font-medium text-gray-500">
            abweichend
          </span>
        ) : null}
        {slot.reason ? (
          <span className="ml-1 text-xs text-gray-500">· {slot.reason}</span>
        ) : null}
      </div>
    </div>
  );
}

function AttendanceTag({ instance }: { readonly instance: CarePlanInstance }) {
  const { status, isUnplanned } = instance.attendance;
  if (instance.status === "cancelled") {
    return (
      <span className="inline-flex items-center gap-1 text-xs font-medium text-gray-500">
        <Ban className="h-3 w-3" aria-hidden="true" /> abgesagt
      </span>
    );
  }
  if (status === "present") {
    return (
      <span
        className="inline-flex items-center gap-1 text-xs font-medium"
        style={{ color: LOCATION_COLORS.GROUP_ROOM }}
      >
        <Check className="h-3 w-3" aria-hidden="true" /> anwesend
        {isUnplanned ? " (spontan)" : ""}
      </span>
    );
  }
  if (status === "absent") {
    return <span className="text-xs font-medium text-gray-400">abwesend</span>;
  }
  return <span className="text-xs font-medium text-gray-400">geplant</span>;
}

function InstanceBlock({
  instance,
  compact,
}: {
  readonly instance: CarePlanInstance;
  readonly compact: boolean;
}) {
  const cancelled = instance.status === "cancelled";
  return (
    <div
      className="rounded-lg border border-l-4 border-gray-200 bg-white px-3 py-2 shadow-sm"
      style={{ borderLeftColor: LOCATION_COLORS.GROUP_ROOM }}
    >
      <div className="flex items-baseline justify-between gap-2">
        <span
          className={`truncate font-semibold text-gray-900 ${
            compact ? "text-sm" : ""
          } ${cancelled ? "line-through" : ""}`}
        >
          {instance.title}
        </span>
        <span className="shrink-0 text-xs text-gray-500 tabular-nums">
          {instance.startTime}–{instance.endTime}
        </span>
      </div>
      <div className="mt-0.5">
        <AttendanceTag instance={instance} />
      </div>
    </div>
  );
}

function FreeCareBandRow({
  band,
  compact,
}: {
  readonly band: FreeCareBand;
  readonly compact: boolean;
}) {
  return (
    <div
      className="rounded-lg border border-dashed border-gray-300 bg-gray-50 px-3 py-2"
      style={{ color: LOCATION_COLORS.UNKNOWN }}
    >
      <div className="flex items-baseline justify-between gap-2">
        <span className={`font-medium ${compact ? "text-sm" : ""}`}>
          Freie Betreuung
        </span>
        <span className="shrink-0 text-xs tabular-nums">
          {band.startTime}–{band.endTime}
        </span>
      </div>
    </div>
  );
}

/**
 * "Kommt heute nicht" — the backend marks a planned absence/cancellation as an
 * arrival OR pickup exception with no time (expectedTime === null; see
 * TestGetStudentDay_{Arrival,Pickup}Exception_NilTimeMeansAbsence). That
 * overrides the normal plan: the child is not in care, so activities/free-care/
 * pickup are suppressed for the day.
 */
function AbsenceNotice({ reason }: { readonly reason?: string }) {
  return (
    <div
      className="rounded-lg border px-3 py-2 text-sm"
      style={{
        borderColor: `${LOCATION_COLORS.NOT_ARRIVAL}33`,
        backgroundColor: `${LOCATION_COLORS.NOT_ARRIVAL}14`,
      }}
    >
      <span
        className="font-semibold"
        style={{ color: LOCATION_COLORS.NOT_ARRIVAL }}
      >
        Kommt heute nicht
      </span>
      {reason ? <span className="ml-1 text-gray-600">· {reason}</span> : null}
    </div>
  );
}

/**
 * A no-time exception on EITHER the arrival or the pickup slot is a planned
 * absence for the day ("kommt heute nicht"). Arrival takes precedence for the
 * reason. Returns { absent: false } when the child is expected.
 */
function resolveAbsence(day: CarePlanDay): {
  absent: boolean;
  reason?: string;
} {
  for (const slot of [day.arrival, day.pickup]) {
    if (slot.source === "exception" && slot.expectedTime == null) {
      return { absent: true, reason: slot.reason };
    }
  }
  return { absent: false };
}

export function CarePlanDayTimeline({
  day,
  deviation,
  compact = false,
  onEditSchedule,
}: CarePlanDayTimelineProps) {
  const hasArrival = day != null && day.arrival.source !== "none";
  const hasPickup = day != null && day.pickup.source !== "none";
  // A no-time arrival/pickup exception is a planned absence that overrides the
  // normal plan for the day.
  const absence = day ? resolveAbsence(day) : { absent: false };
  // A status-day deviation (Krank / Entschuldigt / Klassenfahrt) is itself an
  // all-day absence — DayDeviation.kind is only ever one of those three — so it
  // suppresses the plan exactly like a no-time exception. Without this the
  // normal timeline rendered UNDER the "Krank" banner, contradicting it.
  const isAbsent = absence.absent || deviation != null;
  const timeline = day ? buildTimeline(day) : [];
  // Kept out of the chronological list (their slot already shows as free care)
  // but still worth showing: "Mittagessen fällt aus" is information staff act on.
  const cancelledInstances = day ? day.instances.filter(isCancelled) : [];
  const isEmpty =
    !hasArrival &&
    !hasPickup &&
    timeline.length === 0 &&
    cancelledInstances.length === 0;

  return (
    <div className={compact ? "space-y-2" : "space-y-2.5"}>
      {deviation ? <DeviationBanner deviation={deviation} /> : null}

      {isAbsent ? (
        // Suppress the plan; the deviation banner (if any) already explains why.
        // A status-day absence shows the banner alone; a plain no-time exception
        // with no banner falls back to its own notice.
        deviation ? null : (
          <AbsenceNotice reason={absence.reason} />
        )
      ) : isEmpty ? (
        <p className="py-6 text-center text-sm text-gray-500">
          Kein Betreuungsplan für diesen Tag.
        </p>
      ) : (
        <>
          {hasArrival ? (
            <SlotAnchor slot={day!.arrival} label="Ankunft" icon="in" />
          ) : null}

          {timeline.map((item) =>
            item.kind === "instance" ? (
              <InstanceBlock
                key={`i-${item.instance.id}`}
                instance={item.instance}
                compact={compact}
              />
            ) : (
              <FreeCareBandRow
                key={`f-${item.band.startTime}-${item.band.endTime}`}
                band={item.band}
                compact={compact}
              />
            ),
          )}

          {hasPickup ? (
            <SlotAnchor slot={day!.pickup} label="Abholung" icon="out" />
          ) : null}

          {cancelledInstances.length > 0 ? (
            <div className="space-y-2 pt-0.5">
              {/* Label only in the full view: a week column is too narrow to
                  spend a line on it, and the line-through title plus the
                  "abgesagt" tag already identify these blocks. */}
              {!compact ? (
                <p className="text-xs font-semibold text-gray-500">Abgesagt</p>
              ) : null}
              {cancelledInstances.map((instance) => (
                <InstanceBlock
                  key={`c-${instance.id}`}
                  instance={instance}
                  compact={compact}
                />
              ))}
            </div>
          ) : null}
        </>
      )}

      {!compact && onEditSchedule ? (
        <button
          type="button"
          onClick={onEditSchedule}
          className="text-moto-blue mt-1 text-sm font-semibold hover:underline"
        >
          Zeiten bearbeiten
        </button>
      ) : null}
    </div>
  );
}
