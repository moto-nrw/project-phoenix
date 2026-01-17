"use client";

// Type for the action the user can perform
export type StudentActionType = "checkout" | "checkin" | "none";

interface StudentCheckoutSectionProps {
  readonly onCheckoutClick: () => void;
}

export function StudentCheckoutSection({
  onCheckoutClick,
}: StudentCheckoutSectionProps) {
  return (
    <div className="mb-6 rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm sm:p-6">
      <div className="mb-4 flex items-center gap-3">
        <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-xl bg-[#FF3130] text-white sm:h-10 sm:w-10">
          <svg
            className="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"
            />
          </svg>
        </div>
        <h3 className="text-base font-semibold text-gray-900 sm:text-lg">
          Abmeldung
        </h3>
      </div>
      <button
        onClick={onCheckoutClick}
        className="flex w-full items-center justify-center gap-2 rounded-lg bg-gray-900 px-4 py-3 text-sm font-medium text-white transition-all duration-200 hover:scale-[1.01] hover:bg-gray-700 hover:shadow-lg active:scale-[0.99] sm:py-2.5"
      >
        <svg
          className="h-5 w-5"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"
          />
        </svg>
        Kind abmelden
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
    <div className="mb-6 rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm sm:p-6">
      <div className="mb-4 flex items-center gap-3">
        <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-xl bg-[#83CD2D] text-white sm:h-10 sm:w-10">
          <svg
            className="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M11 16l-4-4m0 0l4-4m-4 4h14m-5 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h7a3 3 0 013 3v1"
            />
          </svg>
        </div>
        <h3 className="text-base font-semibold text-gray-900 sm:text-lg">
          Anmeldung
        </h3>
      </div>
      <button
        onClick={onCheckinClick}
        className="flex w-full items-center justify-center gap-2 rounded-lg bg-gray-900 px-4 py-3 text-sm font-medium text-white transition-all duration-200 hover:scale-[1.01] hover:bg-gray-700 hover:shadow-lg active:scale-[0.99] sm:py-2.5"
      >
        <svg
          className="h-5 w-5"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M11 16l-4-4m0 0l4-4m-4 4h14m-5 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h7a3 3 0 013 3v1"
          />
        </svg>
        Kind anmelden
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
