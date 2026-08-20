type DayPlanningStatus = "comes_today" | "not_coming_today";

export interface DayPlanningStudent {
  day_planning_status?: DayPlanningStatus;
  day_planning_reason?: string;
  day_planning_label?: string;
  arrival_is_exception?: boolean;
  arrival_time?: string;
  arrival_notes?: string;
  actual_arrival_time?: string;
  current_location?: string | null;
}

export function getDayPlanningNotComingLabel(
  student: Pick<
    DayPlanningStudent,
    | "day_planning_status"
    | "day_planning_label"
    | "actual_arrival_time"
    | "current_location"
  >,
  options?: {
    /**
     * Skip the actual-attendance suppression. Set when the planning status was
     * computed for a non-today date (#1939): a child being present right now
     * says nothing about another day, so the "not expected" label must stay.
     */
    ignoreCurrentAttendance?: boolean;
  },
): string | null {
  if (!options?.ignoreCurrentAttendance && hasActualAttendance(student)) {
    return null;
  }
  if (student.day_planning_status !== "not_coming_today") return null;
  return student.day_planning_label ?? "kein Plan für heute";
}

export function getStudentPresenceBadgePlanning(student: DayPlanningStudent): {
  notArrivalToday: boolean;
  notArrivalReason: string | null;
} {
  if (
    student.day_planning_reason === "unplanned_attendance" &&
    hasActualAttendance(student)
  ) {
    return {
      notArrivalToday: true,
      notArrivalReason: student.day_planning_label ?? "ungeplant anwesend",
    };
  }

  const dayPlanningLabel = getDayPlanningNotComingLabel(student);
  const arrivalExceptionAbsent =
    (student.arrival_is_exception ?? false) && !student.arrival_time;

  return {
    notArrivalToday: dayPlanningLabel !== null || arrivalExceptionAbsent,
    notArrivalReason: dayPlanningLabel ?? student.arrival_notes ?? null,
  };
}

function hasActualAttendance(
  student: Pick<DayPlanningStudent, "actual_arrival_time" | "current_location">,
): boolean {
  const location = student.current_location?.trim().toLowerCase();
  if (location) return location !== "zuhause" && location !== "abwesend";
  return !!student.actual_arrival_time;
}
