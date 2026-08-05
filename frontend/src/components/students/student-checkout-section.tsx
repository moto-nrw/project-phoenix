"use client";

import { useEffect, useRef, useState } from "react";
import { LogIn, MoreVertical } from "lucide-react";
import { useAttendanceWebEnabled } from "~/lib/tenant-context";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";

// Type for the action the user can perform
type StudentActionType = "checkout" | "checkin" | "none";

interface StudentCheckoutSectionProps {
  readonly onCheckoutClick: () => void;
}

export function StudentCheckoutSection({
  onCheckoutClick,
}: StudentCheckoutSectionProps) {
  const attendanceWebEnabled = useAttendanceWebEnabled();
  if (!attendanceWebEnabled) return null;
  return (
    <button
      type="button"
      onClick={onCheckoutClick}
      className="moto-content-surface flex flex-1 flex-col items-center gap-3 rounded-3xl border px-3 py-4 backdrop-blur-md transition-all hover:shadow-sm active:scale-[0.97] sm:gap-4 sm:py-6"
    >
      <div className="flex h-12 w-12 items-center justify-center rounded-full border-2 border-gray-200 sm:h-14 sm:w-14">
        <MotoConceptIcon concept="home" size={26} />
      </div>
      <div className="text-center">
        <p className="text-base font-semibold text-gray-900">Abmelden</p>
        <p className="mt-0.5 hidden text-xs text-gray-400 sm:block">
          Für heute aus der OGS abmelden
        </p>
      </div>
    </button>
  );
}

// Component for check-in action (when student is at home)
interface StudentCheckinSectionProps {
  readonly onCheckinClick: () => void;
}

export function StudentCheckinSection({
  onCheckinClick,
}: StudentCheckinSectionProps) {
  const attendanceWebEnabled = useAttendanceWebEnabled();
  if (!attendanceWebEnabled) return null;
  return (
    <button
      type="button"
      onClick={onCheckinClick}
      className="moto-content-surface flex flex-1 flex-col items-center gap-3 rounded-3xl border px-3 py-4 backdrop-blur-md transition-all hover:shadow-sm active:scale-[0.97] sm:gap-4 sm:py-6"
    >
      <div className="border-moto-green text-moto-green flex h-12 w-12 items-center justify-center rounded-full border-2 sm:h-14 sm:w-14">
        <LogIn className="h-5 w-5 sm:h-6 sm:w-6" />
      </div>
      <div className="text-center">
        <p className="text-base font-semibold text-gray-900">Anmelden</p>
        <p className="mt-0.5 hidden text-xs text-gray-400 sm:block">
          Für heute in der OGS anmelden
        </p>
      </div>
    </button>
  );
}

// Component for sick report quick-action
interface StudentSickReportSectionProps {
  readonly isSick: boolean;
  readonly sickSince?: string;
  readonly onToggle: () => void;
  readonly isLoading: boolean;
}

export function StudentSickReportSection({
  isSick,
  sickSince,
  onToggle,
  isLoading,
}: StudentSickReportSectionProps) {
  const sickSinceDisplay = sickSince
    ? new Date(sickSince).toLocaleDateString("de-DE", {
        timeZone: "Europe/Berlin",
        day: "2-digit",
        month: "2-digit",
        year: "numeric",
      })
    : null;

  const renderSickIcon = () => {
    if (isLoading) {
      return (
        <div className="h-5 w-5 animate-spin rounded-full border-2 border-current border-t-transparent" />
      );
    }
    return <MotoConceptIcon concept="sick" size={26} />;
  };

  return (
    <button
      type="button"
      onClick={onToggle}
      disabled={isLoading}
      className="moto-content-surface flex flex-1 flex-col items-center gap-3 rounded-3xl border px-3 py-4 backdrop-blur-md transition-all hover:shadow-sm active:scale-[0.97] disabled:opacity-50 sm:gap-4 sm:py-6"
    >
      <div
        className={`flex h-12 w-12 items-center justify-center rounded-full border-2 sm:h-14 sm:w-14 ${
          isSick ? "border-moto-green" : "border-moto-red/30"
        }`}
      >
        {renderSickIcon()}
      </div>
      <div className="text-center">
        <p className="text-base font-semibold text-gray-900">
          {isSick ? "Gesund melden" : "Krank melden"}
        </p>
        <p className="mt-0.5 hidden text-xs text-gray-400 sm:block">
          {isSick && sickSinceDisplay
            ? `Krank gemeldet seit ${sickSinceDisplay}`
            : "Als krank melden"}
        </p>
      </div>
    </button>
  );
}

// Component for excused (entschuldigt) quick-action
interface StudentExcusedReportSectionProps {
  readonly isExcused: boolean;
  readonly excusedSince?: string;
  readonly onToggle: () => void;
  readonly isLoading: boolean;
}

export function StudentExcusedReportSection({
  isExcused,
  excusedSince,
  onToggle,
  isLoading,
}: StudentExcusedReportSectionProps) {
  const excusedSinceDisplay = excusedSince
    ? new Date(excusedSince).toLocaleDateString("de-DE", {
        timeZone: "Europe/Berlin",
        day: "2-digit",
        month: "2-digit",
        year: "numeric",
      })
    : null;

  const renderExcusedIcon = () => {
    if (isLoading) {
      return (
        <div className="h-5 w-5 animate-spin rounded-full border-2 border-current border-t-transparent" />
      );
    }
    return <MotoConceptIcon concept="excused" size={26} />;
  };

  return (
    <button
      type="button"
      onClick={onToggle}
      disabled={isLoading}
      className="moto-content-surface flex flex-1 flex-col items-center gap-3 rounded-3xl border px-3 py-4 backdrop-blur-md transition-all hover:shadow-sm active:scale-[0.97] disabled:opacity-50 sm:gap-4 sm:py-6"
    >
      <div
        className={`flex h-12 w-12 items-center justify-center rounded-full border-2 sm:h-14 sm:w-14 ${
          isExcused ? "border-moto-green" : "border-moto-purple/30"
        }`}
      >
        {renderExcusedIcon()}
      </div>
      <div className="text-center">
        <p className="text-base font-semibold text-gray-900">
          {isExcused ? "Entschuldigung aufheben" : "Entschuldigen"}
        </p>
        <p className="mt-0.5 hidden text-xs text-gray-400 sm:block">
          {isExcused && excusedSinceDisplay
            ? `Entschuldigt seit ${excusedSinceDisplay}`
            : "Als entschuldigt markieren"}
        </p>
      </div>
    </button>
  );
}

interface StudentStatusActionsMenuProps {
  readonly isClassTrip: boolean;
  readonly classTripSince?: string;
  readonly onPlanClassTrip: () => void;
  readonly isLoading: boolean;
}

export function StudentStatusActionsMenu({
  isClassTrip,
  classTripSince,
  onPlanClassTrip,
  isLoading,
}: StudentStatusActionsMenuProps) {
  const [open, setOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const classTripSinceDisplay = classTripSince
    ? new Date(classTripSince).toLocaleDateString("de-DE", {
        timeZone: "Europe/Berlin",
        day: "2-digit",
        month: "2-digit",
        year: "numeric",
      })
    : null;

  useEffect(() => {
    if (!open) {
      return;
    }

    function handleClickOutside(event: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setOpen(false);
      }
    }

    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [open]);

  return (
    <div className="relative shrink-0" ref={menuRef}>
      <button
        type="button"
        onClick={() => setOpen((prev) => !prev)}
        disabled={isLoading}
        aria-label="Weitere Statusaktionen"
        aria-haspopup="menu"
        aria-expanded={open}
        className="moto-content-surface flex h-full min-h-[116px] w-14 items-center justify-center rounded-3xl border text-gray-500 backdrop-blur-md transition-all hover:bg-gray-50 hover:text-gray-700 hover:shadow-sm active:scale-[0.97] disabled:opacity-50 sm:min-h-[140px] sm:w-16"
      >
        {isLoading ? (
          <div className="h-5 w-5 animate-spin rounded-full border-2 border-current border-t-transparent" />
        ) : (
          <MoreVertical className="h-5 w-5 sm:h-6 sm:w-6" aria-hidden />
        )}
      </button>
      {open ? (
        <div
          role="menu"
          className="moto-content-surface absolute top-full right-0 z-50 mt-2 w-64 overflow-hidden rounded-lg border py-1 shadow-lg"
        >
          <button
            type="button"
            role="menuitem"
            onClick={() => {
              setOpen(false);
              onPlanClassTrip();
            }}
            className="flex w-full items-start gap-3 px-3 py-2 text-left text-sm text-gray-700 hover:bg-gray-50"
          >
            <MotoConceptIcon concept="classTrip" size={18} className="mt-0.5" />
            <span>
              <span className="block font-medium">
                {isClassTrip
                  ? "Klassenfahrt bearbeiten"
                  : "Klassenfahrt planen"}
              </span>
              <span className="block text-xs text-gray-500">
                {isClassTrip && classTripSinceDisplay
                  ? `Seit ${classTripSinceDisplay}`
                  : "Zeitraum für seltene Abwesenheit setzen"}
              </span>
            </span>
          </button>
        </div>
      ) : null}
    </div>
  );
}

// Helper function to determine what action is available for a student
export function getStudentActionType(
  student: {
    group_id?: string;
    current_location?: string;
  },
  myGroups: string[],
  mySupervisedRooms: string[],
): StudentActionType {
  const isInMyGroup = Boolean(
    student.group_id && myGroups.includes(student.group_id),
  );
  const isInMySupervisedRoom = Boolean(
    student.current_location &&
    mySupervisedRooms.some((room) => student.current_location?.includes(room)),
  );

  // User must have access (be in student's group or supervising their room)
  const hasAccess = isInMyGroup || isInMySupervisedRoom;

  if (!hasAccess) {
    return "none";
  }

  // Check if student is at home
  const isAtHome =
    !student.current_location || student.current_location.startsWith("Zuhause");

  if (isAtHome) {
    // Student is at home - can check in (but only if user is in student's group)
    // Room supervisors can't check in students who aren't in a room
    return isInMyGroup ? "checkin" : "none";
  }

  // Student is checked in (in OGS) - can check out
  return "checkout";
}
