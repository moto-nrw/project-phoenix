"use client";

import { Home, LogIn, Thermometer, Check } from "lucide-react";

// Type for the action the user can perform
type StudentActionType = "checkout" | "checkin" | "none";

interface StudentCheckoutSectionProps {
  readonly onCheckoutClick: () => void;
}

export function StudentCheckoutSection({
  onCheckoutClick,
}: StudentCheckoutSectionProps) {
  return (
    <div className="rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm">
      <div className="mb-4 flex items-center gap-2 sm:gap-3">
        <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-[#FF3130] text-white sm:h-10 sm:w-10">
          <Home className="h-5 w-5" />
        </div>
        <h2 className="text-base font-semibold text-gray-900 sm:text-lg">
          Abmeldung
        </h2>
      </div>
      <div className="flex items-center justify-center py-2">
        <button
          onClick={onCheckoutClick}
          className="flex h-14 w-14 items-center justify-center rounded-full bg-[#FF3130] text-white shadow-sm transition-all hover:shadow-md active:scale-95"
        >
          <Home className="h-6 w-6" />
        </button>
      </div>
      <p className="mt-2 text-center text-xs leading-relaxed text-gray-400">
        Für heute aus der OGS abmelden
      </p>
    </div>
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
    <div className="rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm">
      <div className="mb-4 flex items-center gap-2 sm:gap-3">
        <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-[#83CD2D] text-white sm:h-10 sm:w-10">
          <LogIn className="h-5 w-5" />
        </div>
        <h2 className="text-base font-semibold text-gray-900 sm:text-lg">
          Anmeldung
        </h2>
      </div>
      <div className="flex items-center justify-center py-2">
        <button
          onClick={onCheckinClick}
          className="flex h-14 w-14 items-center justify-center rounded-full bg-[#83CD2D] text-white shadow-sm transition-all hover:shadow-md active:scale-95"
        >
          <LogIn className="h-6 w-6" />
        </button>
      </div>
      <p className="mt-2 text-center text-xs leading-relaxed text-gray-400">
        Für heute in der OGS anmelden
      </p>
    </div>
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

  return (
    <div
      className={`rounded-2xl border p-4 backdrop-blur-sm ${
        isSick
          ? "border-amber-200 bg-amber-50/80"
          : "border-gray-100 bg-white/50"
      }`}
    >
      <div className="mb-4 flex items-center gap-2 sm:gap-3">
        <div
          className={`flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg text-white sm:h-10 sm:w-10 ${
            isSick ? "bg-amber-500" : "bg-amber-400"
          }`}
        >
          {isSick ? (
            <Check className="h-5 w-5" />
          ) : (
            <Thermometer className="h-5 w-5" />
          )}
        </div>
        <div className="min-w-0">
          <h2 className="text-base font-semibold text-gray-900 sm:text-lg">
            Krankmeldung
          </h2>
          {isSick && sickSinceDisplay && (
            <p className="text-xs text-amber-600">seit {sickSinceDisplay}</p>
          )}
        </div>
      </div>
      <div className="flex items-center justify-center py-2">
        <button
          onClick={onToggle}
          disabled={isLoading}
          className={`flex h-14 w-14 items-center justify-center rounded-full text-white shadow-sm transition-all hover:shadow-md active:scale-95 disabled:opacity-50 ${
            isSick ? "bg-[#83CD2D]" : "bg-amber-500"
          }`}
        >
          {isLoading ? (
            <div className="h-5 w-5 animate-spin rounded-full border-2 border-white border-t-transparent" />
          ) : isSick ? (
            <Check className="h-6 w-6" />
          ) : (
            <Thermometer className="h-6 w-6" />
          )}
        </button>
      </div>
      <p className="mt-2 text-center text-xs leading-relaxed text-gray-400">
        {isSick
          ? "Krankmeldung aufheben und gesund melden"
          : "Als krank melden"}
      </p>
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
