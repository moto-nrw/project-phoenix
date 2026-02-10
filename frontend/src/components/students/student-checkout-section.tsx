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
      <div className="flex flex-col items-center gap-3 py-2">
        <button
          onClick={onCheckoutClick}
          className="flex h-16 w-16 items-center justify-center rounded-full border-2 border-[#FF3130] text-[#FF3130] transition-all hover:bg-[#FF3130]/5 active:scale-95"
          aria-label="Kind abmelden"
        >
          <Home className="h-7 w-7" />
        </button>
        <span className="text-sm font-medium text-gray-700">
          Geht nach Hause
        </span>
      </div>
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
      <div className="flex flex-col items-center gap-3 py-2">
        <button
          onClick={onCheckinClick}
          className="flex h-16 w-16 items-center justify-center rounded-full border-2 border-[#83CD2D] text-[#83CD2D] transition-all hover:bg-[#83CD2D]/5 active:scale-95"
          aria-label="Kind anmelden"
        >
          <LogIn className="h-7 w-7" />
        </button>
        <span className="text-sm font-medium text-gray-700">Anmelden</span>
      </div>
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
      <div className="flex flex-col items-center gap-3 py-2">
        <button
          onClick={onToggle}
          disabled={isLoading}
          className={`flex h-16 w-16 items-center justify-center rounded-full border-2 transition-all active:scale-95 disabled:opacity-50 ${
            isSick
              ? "border-[#83CD2D] text-[#83CD2D] hover:bg-[#83CD2D]/5"
              : "border-amber-400 text-amber-500 hover:bg-amber-50"
          }`}
          aria-label={isSick ? "Kind gesundmelden" : "Kind krankmelden"}
        >
          {isLoading ? (
            <div className="h-5 w-5 animate-spin rounded-full border-2 border-current border-t-transparent" />
          ) : isSick ? (
            <Check className="h-7 w-7" />
          ) : (
            <Thermometer className="h-7 w-7" />
          )}
        </button>
        <div className="text-center">
          <span className="text-sm font-medium text-gray-700">
            {isSick ? "Gesundmelden" : "Krankmelden"}
          </span>
          {isSick && sickSinceDisplay && (
            <p className="text-xs text-amber-600">seit {sickSinceDisplay}</p>
          )}
        </div>
      </div>
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
