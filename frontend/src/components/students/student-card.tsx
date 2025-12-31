// components/students/student-card.tsx
// Shared student card component used across OGS groups and active supervisions pages

import type { ReactNode } from "react";

interface StudentCardProps {
  /** Unique student ID */
  studentId: string;
  /** Student's first name */
  firstName?: string;
  /** Student's last name */
  lastName?: string;
  /** Gradient class for the card overlay */
  gradient?: string;
  /** Click handler for navigation */
  onClick: () => void;
  /** Location badge component to render */
  locationBadge: ReactNode;
  /** Optional extra content between name and click hint */
  extraContent?: ReactNode;
}

/**
 * Reusable student card component with modern styling.
 * Used in OGS groups and active supervisions pages.
 */
export function StudentCard({
  studentId,
  firstName,
  lastName,
  gradient = "from-blue-50/80 to-cyan-100/80",
  onClick,
  locationBadge,
  extraContent,
}: StudentCardProps) {
  return (
    <button
      key={studentId}
      type="button"
      onClick={onClick}
      aria-label={`${firstName} ${lastName} - Tippen für mehr Infos`}
      className="group relative w-full cursor-pointer overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 text-left shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-500 focus:ring-2 focus:ring-blue-500/50 focus:outline-none active:scale-[0.97] md:hover:-translate-y-3 md:hover:scale-[1.03] md:hover:border-[#5080D8]/30 md:hover:bg-white md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)]"
    >
      {/* Modern gradient overlay */}
      <div
        className={`absolute inset-0 bg-gradient-to-br ${gradient} rounded-3xl opacity-[0.03]`}
      />
      {/* Subtle inner glow */}
      <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20" />
      {/* Modern border highlight */}
      <div className="absolute inset-0 rounded-3xl ring-1 ring-white/20 transition-all duration-300 md:group-hover:ring-blue-200/60" />

      <div className="relative p-6">
        {/* Header with student name */}
        <div className="mb-3 flex items-start justify-between gap-3">
          {/* Student Name */}
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <h3 className="overflow-hidden text-lg font-bold text-ellipsis whitespace-nowrap text-gray-800 transition-colors duration-300 md:group-hover:text-blue-600">
                {firstName}
              </h3>
              {/* Subtle integrated arrow */}
              <svg
                className="h-4 w-4 flex-shrink-0 text-gray-300 transition-all duration-300 md:group-hover:translate-x-1 md:group-hover:text-blue-500"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M9 5l7 7-7 7"
                />
              </svg>
            </div>
            <p className="overflow-hidden text-base font-semibold text-ellipsis whitespace-nowrap text-gray-700 transition-colors duration-300 md:group-hover:text-blue-500">
              {lastName}
            </p>
            {/* Extra content slot (school class, group name, etc.) */}
            {extraContent}
          </div>

          {/* Location Badge */}
          {locationBadge}
        </div>

        {/* Bottom row with click hint */}
        <div className="flex justify-start">
          <p className="text-xs text-gray-400 transition-colors duration-300 md:group-hover:text-blue-400">
            Tippen für mehr Infos
          </p>
        </div>

        {/* Decorative elements */}
        <div className="absolute top-3 left-3 h-5 w-5 animate-ping rounded-full bg-white/20" />
        <div className="absolute right-3 bottom-3 h-3 w-3 rounded-full bg-white/30" />
      </div>

      {/* Glowing border effect */}
      <div className="absolute inset-0 rounded-3xl bg-gradient-to-r from-transparent via-blue-100/30 to-transparent opacity-0 transition-opacity duration-300 md:group-hover:opacity-100" />
    </button>
  );
}

/** Icon for school class display */
export function SchoolClassIcon() {
  return (
    <svg
      className="h-3.5 w-3.5 text-gray-400"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253"
      />
    </svg>
  );
}

/** Icon for group display */
export function GroupIcon() {
  return (
    <svg
      className="h-3.5 w-3.5 text-gray-400"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
      />
    </svg>
  );
}

/** Reusable info row for school class or group */
export function StudentInfoRow({
  icon,
  children,
}: {
  icon: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="mt-1 flex items-center gap-1.5">
      {icon}
      <span className="overflow-hidden text-xs font-medium text-ellipsis whitespace-nowrap text-gray-500">
        {children}
      </span>
    </div>
  );
}
