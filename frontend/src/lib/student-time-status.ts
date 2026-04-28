import { LOCATION_COLORS } from "./location-helper";

const APPROACHING_THRESHOLD_MINUTES = 30;

type TimeStatusState =
  | "none"
  | "planned"
  | "approaching"
  | "slightly-overdue"
  | "very-overdue"
  | "done-on-time"
  | "done-late";

type TimeStatusIcon = "clock" | "warning" | "check";

export interface StudentTimeStatus {
  state: TimeStatusState;
  displayTime?: string;
  icon: TimeStatusIcon;
  iconColor: string;
  textColor?: string;
  isResolved: boolean;
  detailAnnotation?: string;
  diffMinutes?: number;
}

interface StudentTimeStatusInput {
  plannedTime?: string;
  actualTime?: string;
  now: Date;
}

interface TimeParts {
  hours: number;
  minutes: number;
}

function parseTimeParts(time: string): TimeParts | null {
  const [hoursRaw, minutesRaw] = time.split(":");
  const hours = Number.parseInt(hoursRaw ?? "", 10);
  const minutes = Number.parseInt(minutesRaw ?? "", 10);

  if (Number.isNaN(hours) || Number.isNaN(minutes)) {
    return null;
  }

  return { hours, minutes };
}

function buildTimeDate(time: string, baseDate: Date): Date | null {
  const parts = parseTimeParts(time);
  if (!parts) {
    return null;
  }

  const result = new Date(baseDate);
  result.setHours(parts.hours, parts.minutes, 0, 0);
  return result;
}

function formatTimeDisplay(time?: string): string | undefined {
  if (!time) {
    return undefined;
  }

  return time.length > 5 ? time.slice(0, 5) : time;
}

function formatActualDiff(diffMinutes: number): string | undefined {
  if (diffMinutes === 0) {
    return undefined;
  }

  if (diffMinutes > 0) {
    return `+${diffMinutes} min`;
  }

  return `-${Math.abs(diffMinutes)} min früh`;
}

function formatOverdueDiff(diffMinutes: number): string | undefined {
  if (diffMinutes >= 0) {
    return undefined;
  }

  return `Überfällig seit ${Math.abs(diffMinutes)} min`;
}

export function getStudentTimeStatus({
  plannedTime,
  actualTime,
  now,
}: StudentTimeStatusInput): StudentTimeStatus {
  const displayActualTime = formatTimeDisplay(actualTime);
  const displayPlannedTime = formatTimeDisplay(plannedTime);

  const actualDate = actualTime ? buildTimeDate(actualTime, now) : null;
  const plannedDate = plannedTime ? buildTimeDate(plannedTime, now) : null;

  if (displayActualTime) {
    const diffMinutes =
      actualDate && plannedDate
        ? Math.round((actualDate.getTime() - plannedDate.getTime()) / 60000)
        : undefined;

    const doneLate = (diffMinutes ?? 0) > 0;
    return {
      state: doneLate ? "done-late" : "done-on-time",
      displayTime: displayActualTime,
      icon: "check",
      iconColor: doneLate
        ? LOCATION_COLORS.SCHOOLYARD
        : LOCATION_COLORS.GROUP_ROOM,
      textColor: undefined,
      isResolved: true,
      detailAnnotation:
        diffMinutes === undefined ? undefined : formatActualDiff(diffMinutes),
      diffMinutes,
    };
  }

  if (!displayPlannedTime || !plannedDate) {
    return {
      state: "none",
      displayTime: undefined,
      icon: "clock",
      iconColor: LOCATION_COLORS.UNKNOWN,
      textColor: undefined,
      isResolved: false,
    };
  }

  const diffMinutes = Math.round(
    (plannedDate.getTime() - now.getTime()) / 60000,
  );

  if (diffMinutes > APPROACHING_THRESHOLD_MINUTES) {
    return {
      state: "planned",
      displayTime: displayPlannedTime,
      icon: "clock",
      iconColor: LOCATION_COLORS.UNKNOWN,
      textColor: undefined,
      isResolved: false,
      diffMinutes,
    };
  }

  if (diffMinutes >= 0) {
    return {
      state: "approaching",
      displayTime: displayPlannedTime,
      icon: "clock",
      iconColor: LOCATION_COLORS.SCHOOLYARD,
      textColor: undefined,
      isResolved: false,
      diffMinutes,
    };
  }

  if (diffMinutes > -APPROACHING_THRESHOLD_MINUTES) {
    return {
      state: "slightly-overdue",
      displayTime: displayPlannedTime,
      icon: "warning",
      iconColor: LOCATION_COLORS.SCHOOLYARD,
      textColor: LOCATION_COLORS.SCHOOLYARD,
      isResolved: false,
      detailAnnotation: formatOverdueDiff(diffMinutes),
      diffMinutes,
    };
  }

  return {
    state: "very-overdue",
    displayTime: displayPlannedTime,
    icon: "warning",
    iconColor: LOCATION_COLORS.HOME,
    textColor: LOCATION_COLORS.HOME,
    isResolved: false,
    detailAnnotation: formatOverdueDiff(diffMinutes),
    diffMinutes,
  };
}

export function areStudentDayTimesResolved(
  arrivalStatus: StudentTimeStatus,
  pickupStatus: StudentTimeStatus,
): boolean {
  return arrivalStatus.isResolved && pickupStatus.isResolved;
}

export function combineTimeNotes(
  notes?: string,
  dayNotes?: ReadonlyArray<{ content: string }>,
): string | undefined {
  const combined = [notes, ...(dayNotes?.map((note) => note.content) ?? [])]
    .filter(Boolean)
    .join(", ");

  return combined || undefined;
}

export function getTimeStatusSortRank(status: StudentTimeStatus): number {
  switch (status.state) {
    case "very-overdue":
      return 0;
    case "slightly-overdue":
      return 1;
    case "approaching":
      return 2;
    case "planned":
      return 3;
    case "done-late":
      return 4;
    case "done-on-time":
      return 5;
    case "none":
    default:
      return 6;
  }
}
