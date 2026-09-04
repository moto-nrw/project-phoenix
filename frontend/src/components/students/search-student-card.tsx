"use client";

// components/students/search-student-card.tsx
// One Kinderkarte of the Kindersuche list (#2975).
//
// This used to be a ~185-line closure inside the search page, which meant every
// card was rebuilt whenever anything on the page changed — a keystroke, a
// minute tick, a single check-in tap. As its own memoised component it only
// re-renders when this child's own data changes: the page render stops at the
// card boundary. Everything the card needs is derived HERE from stable props;
// building JSX (badges, rows, indicators) in the page and handing it down as
// props would defeat the memo again, because JSX is a new object every render.

import React from "react";

import { DataTableStatusBadge } from "~/components/ui/data-table";
import { StudentPresenceBadge } from "~/components/ui/student-presence-badge";
import { TrackingIndicators } from "~/components/students/tracking-indicators";
import {
  StudentCard,
  SchoolClassIcon,
  GroupIcon,
  DepartureModeIcon,
  StudentInfoRow,
  PickupTimeRow,
  ArrivalTimeRow,
  StudentAbsenceRow,
  StudentPendingExcusedRow,
} from "~/components/students/student-card";
import type { Student } from "~/lib/api";
import type { TrackingIndicatorsResponse } from "~/lib/active-helpers";
import type { DepartureMode } from "~/lib/student-helpers";
import { getStudentAbsence } from "~/lib/student-time-status";
import {
  getDayPlanningNotComingLabel,
  getStudentPresenceBadgePlanning,
} from "~/lib/day-planning-helper";
import { deriveCheckinState } from "~/lib/hooks/use-school-checkin-mode";

const DAILY_DEPARTURE_MODE_LABELS: Record<DepartureMode, string> = {
  alone: "Geht alleine nach Hause",
  bus: "Bus",
  pickup: "Wird abgeholt",
  accompanied: "Mit anderem Kind",
};

/** Heimweg line of a card, also used by the page's Heimweg grouping. */
export function dailyDepartureLabelForStudent(student: Student): string {
  if (student.has_full_access === false) return "Nicht einsehbar";
  const legacyLabel = student.departure_label?.trim();
  if (legacyLabel) return legacyLabel;
  const modes = student.departure_modes ?? [];
  if (modes.length === 0) return "–";
  return modes.map((mode) => DAILY_DEPARTURE_MODE_LABELS[mode]).join(", ");
}

export interface SearchStudentCardProps {
  readonly student: Student;
  /** Live presence view (today) vs. planned expectation (any other day, #1939). */
  readonly isToday: boolean;
  readonly checkinMode: boolean;
  readonly checkinSelectMode: boolean;
  readonly isCheckinSelected: boolean;
  readonly isCheckinPending: boolean;
  readonly userGroups: string[];
  readonly groupRooms: string[];
  readonly supervisedRooms: string[];
  /**
   * The whole tracking response, not this child's slice: slicing in the page
   * would hand every card a fresh array on each render.
   */
  readonly trackingData: TrackingIndicatorsResponse | undefined;
  readonly onOpen: (student: Student) => void;
  readonly onCheckinClick: (student: Student) => void;
}

function SearchStudentCardImpl({
  student,
  isToday,
  checkinMode,
  checkinSelectMode,
  isCheckinSelected,
  isCheckinPending,
  userGroups,
  groupRooms,
  supervisedRooms,
  trackingData,
  onOpen,
  onCheckinClick,
}: Readonly<SearchStudentCardProps>) {
  const checkinState = deriveCheckinState(student.current_location);
  const showTracking =
    isToday &&
    Boolean(trackingData?.labels?.length) &&
    student.has_full_access !== false;

  return (
    <StudentCard
      studentId={student.id}
      firstName={student.first_name}
      lastName={student.second_name}
      photoUrl={student.photo_url ?? null}
      onClick={() => onOpen(student)}
      checkinMode={checkinMode}
      checkinState={checkinState}
      checkinSelectMode={checkinSelectMode}
      isCheckinSelected={isCheckinSelected}
      isCheckinPending={isCheckinPending}
      onCheckinClick={() => onCheckinClick(student)}
      locationBadge={
        isToday ? (
          <StudentPresenceBadge
            student={(() => {
              const badgePlanning = getStudentPresenceBadgePlanning(student);
              return {
                ...student,
                not_arrival_today: badgePlanning.notArrivalToday,
                not_arrival_reason: badgePlanning.notArrivalReason,
              };
            })()}
            displayMode="contextAware"
            userGroups={userGroups}
            groupRooms={groupRooms}
            supervisedRooms={supervisedRooms}
            variant="modern"
            size="md"
          />
        ) : (
          // Non-today dates show the planned expectation, never the live
          // location (#1939). When the caller lacks full access the backend
          // skips day-planning enrichment and omits day_planning_status; render
          // an unknown state rather than asserting "Kommt nicht" for a result
          // that was never calculated or disclosed.
          <DataTableStatusBadge
            active={student.day_planning_status === "comes_today"}
            unknown={student.day_planning_status === undefined}
            activeLabel="Kommt"
            inactiveLabel="Kommt nicht"
            unknownLabel="Keine Angabe"
          />
        )
      }
      extraContent={
        <>
          <StudentInfoRow icon={<SchoolClassIcon />}>
            {student.school_class || "—"}
          </StudentInfoRow>
          <StudentInfoRow icon={<GroupIcon />}>
            Gruppe: {student.group_name || "—"}
          </StudentInfoRow>
          {student.has_full_access !== false && (
            <StudentInfoRow icon={<DepartureModeIcon />} wrap>
              Heimweg: {dailyDepartureLabelForStudent(student)}
            </StudentInfoRow>
          )}
          {student.has_full_access !== false &&
            student.pending_excused_note !== undefined && (
              <StudentPendingExcusedRow note={student.pending_excused_note} />
            )}
          {student.has_full_access !== false &&
            (() => {
              const absence = getStudentAbsence({
                sick: student.sick,
                classTrip: student.class_trip,
                excused: student.excused,
              });
              const absenceWording = isToday ? undefined : "Kommt nicht";
              // Absence rows fill the arrival slot; a neutral "Gehzeit: —" row
              // (deliberately without the planned time, so no overdue styling
              // can fire for a child who is not coming) keeps absent and
              // present cards at the same four-row height.
              const absencePickupRow = <PickupTimeRow isException={false} />;
              if (absence && !student.actual_pickup_time) {
                return (
                  <>
                    <StudentAbsenceRow
                      label={absence.label}
                      wording={absenceWording}
                    />
                    {absencePickupRow}
                  </>
                );
              }
              const dayPlanningNotComingLabel = getDayPlanningNotComingLabel(
                student,
                { ignoreCurrentAttendance: !isToday },
              );
              if (dayPlanningNotComingLabel && !student.actual_pickup_time) {
                return (
                  <>
                    <StudentAbsenceRow
                      label={dayPlanningNotComingLabel}
                      wording={absenceWording}
                    />
                    {absencePickupRow}
                  </>
                );
              }
              return (
                <>
                  <ArrivalTimeRow
                    arrivalTime={student.arrival_time}
                    actualTime={student.actual_arrival_time}
                    isException={student.arrival_is_exception ?? false}
                    isAbsent={
                      (student.arrival_is_exception ?? false) &&
                      !student.arrival_time
                    }
                    notes={student.arrival_notes}
                    absentWording={absenceWording}
                  />
                  <PickupTimeRow
                    pickupTime={student.pickup_time ?? undefined}
                    actualTime={student.actual_pickup_time}
                    isException={student.pickup_is_exception ?? false}
                    notes={student.pickup_notes}
                  />
                </>
              );
            })()}
        </>
      }
      trackingIndicators={
        showTracking && trackingData ? (
          <TrackingIndicators
            labels={trackingData.labels}
            results={trackingData.results[student.id] ?? []}
          />
        ) : undefined
      }
    />
  );
}

/**
 * Every prop is a primitive, a stable array from the user context, the SWR
 * object itself or a ref-stable callback, so the default shallow comparison is
 * enough: a card re-renders when its own student object, its own check-in flags
 * or the tracking response change — and not when the page re-renders for any
 * other reason.
 */
export const SearchStudentCard = React.memo(SearchStudentCardImpl);
