"use client";

import {
  Home,
  LogIn,
  Thermometer,
  Heart,
  CalendarX,
  CalendarCheck,
} from "lucide-react";

// Type for the action the user can perform
type StudentActionType = "checkout" | "checkin" | "none";

interface StudentCheckoutSectionProps {
  readonly onCheckoutClick: () => void;
}

export function StudentCheckoutSection({
  onCheckoutClick,
}: StudentCheckoutSectionProps) {
  return (
    <button
      onClick={onCheckoutClick}
      className="moto-content-surface flex flex-1 flex-col items-center gap-3 rounded-3xl border px-3 py-4 backdrop-blur-md transition-all hover:shadow-sm active:scale-[0.97] sm:gap-4 sm:py-6"
    >
      <div className="flex h-12 w-12 items-center justify-center rounded-full border-2 border-[#FF3130] text-[#FF3130] sm:h-14 sm:w-14">
        <Home className="h-5 w-5 sm:h-6 sm:w-6" />
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
  return (
    <button
      onClick={onCheckinClick}
      className="moto-content-surface flex flex-1 flex-col items-center gap-3 rounded-3xl border px-3 py-4 backdrop-blur-md transition-all hover:shadow-sm active:scale-[0.97] sm:gap-4 sm:py-6"
    >
      <div className="flex h-12 w-12 items-center justify-center rounded-full border-2 border-[#83CD2D] text-[#83CD2D] sm:h-14 sm:w-14">
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
    if (isSick) {
      return <Heart className="h-5 w-5 sm:h-6 sm:w-6" />;
    }
    return <Thermometer className="h-5 w-5 sm:h-6 sm:w-6" />;
  };

  return (
    <button
      onClick={onToggle}
      disabled={isLoading}
      className="moto-content-surface flex flex-1 flex-col items-center gap-3 rounded-3xl border px-3 py-4 backdrop-blur-md transition-all hover:shadow-sm active:scale-[0.97] disabled:opacity-50 sm:gap-4 sm:py-6"
    >
      <div
        className={`flex h-12 w-12 items-center justify-center rounded-full border-2 sm:h-14 sm:w-14 ${
          isSick
            ? "border-[#83CD2D] text-[#83CD2D]"
            : "border-amber-500 text-amber-500"
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
    if (isExcused) {
      return <CalendarCheck className="h-5 w-5 sm:h-6 sm:w-6" />;
    }
    return <CalendarX className="h-5 w-5 sm:h-6 sm:w-6" />;
  };

  return (
    <button
      onClick={onToggle}
      disabled={isLoading}
      className="moto-content-surface flex flex-1 flex-col items-center gap-3 rounded-3xl border px-3 py-4 backdrop-blur-md transition-all hover:shadow-sm active:scale-[0.97] disabled:opacity-50 sm:gap-4 sm:py-6"
    >
      <div
        className={`flex h-12 w-12 items-center justify-center rounded-full border-2 sm:h-14 sm:w-14 ${
          isExcused
            ? "border-[#83CD2D] text-[#83CD2D]"
            : "border-[#7C3AED] text-[#7C3AED]"
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
