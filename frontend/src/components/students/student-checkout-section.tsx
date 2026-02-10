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
      <div className="mb-3 flex items-center gap-2.5">
        <div className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg bg-[#FF3130] text-white">
          <Home className="h-4 w-4" />
        </div>
        <h3 className="text-sm font-semibold text-gray-900">Abmeldung</h3>
      </div>
      <button
        onClick={onCheckoutClick}
        className="flex w-full items-center justify-center gap-2 rounded-xl bg-[#FF3130] px-4 py-2.5 text-sm font-medium text-white transition-all hover:bg-[#e02b2a] hover:shadow-md active:scale-[0.98]"
      >
        <Home className="h-4 w-4" />
        Geht nach Hause
      </button>
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
      <div className="mb-3 flex items-center gap-2.5">
        <div className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg bg-[#83CD2D] text-white">
          <LogIn className="h-4 w-4" />
        </div>
        <h3 className="text-sm font-semibold text-gray-900">Anmeldung</h3>
      </div>
      <button
        onClick={onCheckinClick}
        className="flex w-full items-center justify-center gap-2 rounded-xl bg-[#83CD2D] px-4 py-2.5 text-sm font-medium text-white transition-all hover:bg-[#70b525] hover:shadow-md active:scale-[0.98]"
      >
        <LogIn className="h-4 w-4" />
        Kind anmelden
      </button>
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
      <div className="mb-3 flex items-center gap-2.5">
        <div
          className={`flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg text-white ${
            isSick ? "bg-amber-500" : "bg-amber-400"
          }`}
        >
          {isSick ? (
            <Check className="h-4 w-4" />
          ) : (
            <Thermometer className="h-4 w-4" />
          )}
        </div>
        <div className="min-w-0">
          <h3 className="text-sm font-semibold text-gray-900">Krankmeldung</h3>
          {isSick && sickSinceDisplay && (
            <p className="text-xs text-amber-600">seit {sickSinceDisplay}</p>
          )}
        </div>
      </div>
      <button
        onClick={onToggle}
        disabled={isLoading}
        className={`flex w-full items-center justify-center gap-2 rounded-xl px-4 py-2.5 text-sm font-medium text-white transition-all hover:shadow-md active:scale-[0.98] disabled:opacity-50 ${
          isSick
            ? "bg-[#83CD2D] hover:bg-[#70b525]"
            : "bg-amber-500 hover:bg-amber-600"
        }`}
      >
        {isLoading ? (
          <div className="h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" />
        ) : isSick ? (
          <Check className="h-4 w-4" />
        ) : (
          <Thermometer className="h-4 w-4" />
        )}
        {isLoading
          ? "Wird gespeichert..."
          : isSick
            ? "Gesundmelden"
            : "Krankmelden"}
      </button>
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
