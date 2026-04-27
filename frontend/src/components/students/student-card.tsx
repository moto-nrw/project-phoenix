// components/students/student-card.tsx
// Shared student card component used across OGS groups and active supervisions pages

import type { ReactNode } from "react";
import { Clock, AlertTriangle, Check, LogIn } from "lucide-react";
import {
  getStudentTimeStatus,
  type StudentTimeStatus,
} from "~/lib/student-time-status";

interface StudentCardProps {
  /** Unique student ID */
  readonly studentId: string;
  /** Student's first name */
  readonly firstName?: string;
  /** Student's last name */
  readonly lastName?: string;
  /** Gradient class for the card overlay */
  readonly gradient?: string;
  /** Click handler for navigation */
  readonly onClick: () => void;
  /** Location badge component to render */
  readonly locationBadge: ReactNode;
  /** Optional extra content between name and click hint */
  readonly extraContent?: ReactNode;
  /** Optional tracking indicators (right-aligned, below location badge) */
  readonly trackingIndicators?: ReactNode;
  /** Whether both arrival and pickup are resolved for today */
  readonly isClosedOut?: boolean;
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
  trackingIndicators,
  isClosedOut = false,
}: StudentCardProps) {
  return (
    <button
      key={studentId}
      type="button"
      onClick={onClick}
      aria-label={`${firstName} ${lastName} - Tippen für mehr Infos`}
      className={`group relative w-full cursor-pointer overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 text-left shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-150 focus:ring-2 focus:ring-blue-500/50 focus:outline-none active:scale-[0.98] md:hover:-translate-y-0.5 md:hover:border-[#5080D8]/40 md:hover:bg-white md:hover:shadow-[0_12px_40px_rgb(0,0,0,0.18)] ${
        isClosedOut ? "opacity-60" : ""
      }`}
    >
      {/* Modern gradient overlay */}
      <div
        className={`absolute inset-0 bg-gradient-to-br ${gradient} rounded-3xl opacity-[0.03]`}
      />
      {/* Subtle inner glow */}
      <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20" />
      {/* Modern border highlight */}
      <div className="absolute inset-0 rounded-3xl ring-1 ring-white/20 transition-all duration-150 md:group-hover:ring-blue-200/60" />

      <div className="relative p-6">
        {/* Header with student name */}
        <div className="mb-3 flex items-start justify-between gap-3">
          {/* Student Name */}
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <h3 className="overflow-hidden text-lg font-bold text-ellipsis whitespace-nowrap text-gray-800 transition-colors duration-150 md:group-hover:text-blue-600">
                {firstName}
              </h3>
              {/* Subtle integrated arrow */}
              <svg
                className="h-4 w-4 flex-shrink-0 text-gray-300 transition-colors duration-150 md:group-hover:text-blue-500"
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
            <p className="overflow-hidden text-base font-semibold text-ellipsis whitespace-nowrap text-gray-700 transition-colors duration-150 md:group-hover:text-blue-500">
              {lastName}
            </p>
            {/* Extra content slot (school class, group name, etc.) */}
            {extraContent}
          </div>

          {/* Location Badge + optional Tracking Indicators */}
          {trackingIndicators ? (
            <div className="flex flex-col items-end gap-1">
              {locationBadge}
              {trackingIndicators}
            </div>
          ) : (
            locationBadge
          )}
        </div>

        {/* Bottom row with click hint */}
        <div className="flex justify-start">
          <p className="text-xs text-gray-400 transition-colors duration-150 md:group-hover:text-blue-400">
            Tippen für mehr Infos
          </p>
        </div>

        {/* Decorative elements */}
        <div className="absolute top-3 left-3 h-5 w-5 animate-ping rounded-full bg-white/20" />
        <div className="absolute right-3 bottom-3 h-3 w-3 rounded-full bg-white/30" />
      </div>

      {/* Glowing border effect */}
      <div className="absolute inset-0 rounded-3xl bg-gradient-to-r from-transparent via-blue-100/30 to-transparent opacity-0 transition-opacity duration-150 md:group-hover:opacity-100" />
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
}: Readonly<{
  icon: ReactNode;
  children: ReactNode;
}>) {
  return (
    <div className="mt-1 flex items-center gap-1.5">
      <span className="flex-shrink-0">{icon}</span>
      <span className="overflow-hidden text-xs font-medium text-ellipsis whitespace-nowrap text-gray-500">
        {children}
      </span>
    </div>
  );
}

/** Icon for pickup time display */
export function PickupTimeIcon() {
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
        d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
      />
    </svg>
  );
}

/** Icon for exception indicator */
export function ExceptionIcon() {
  return (
    <svg
      className="h-3.5 w-3.5 text-orange-500"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
      />
    </svg>
  );
}

/**
 * Shared pickup time display row used across OGS groups, active supervisions,
 * and student search pages. Normalizes the three rendering branches:
 *   1. Has planned or actual time → show status-aware time row
 *   2. Exception without time → show exception reason
 *   3. Neither → show "Abholzeit: —" fallback
 */
function ArrivalTimeIcon() {
  return <LogIn className="h-3.5 w-3.5 text-gray-400" />;
}

function renderTimeStatusIcon(
  status: StudentTimeStatus,
  kind: "arrival" | "pickup",
): ReactNode {
  if (status.icon === "warning") {
    return (
      <AlertTriangle
        className="h-3.5 w-3.5"
        style={{ color: status.iconColor }}
      />
    );
  }

  if (status.icon === "check") {
    return (
      <Check className="h-3.5 w-3.5" style={{ color: status.iconColor }} />
    );
  }

  if (kind === "arrival") {
    return (
      <LogIn
        className={`h-3.5 w-3.5 ${
          status.state === "approaching" ? "animate-pulse" : ""
        }`}
        style={{ color: status.iconColor }}
      />
    );
  }

  return (
    <Clock
      className={`h-3.5 w-3.5 ${
        status.state === "approaching" ? "animate-pulse" : ""
      }`}
      style={{ color: status.iconColor }}
    />
  );
}

function TimeStatusRow({
  label,
  plannedTime,
  actualTime,
  isException,
  notes,
  now,
  kind,
}: Readonly<{
  label: string;
  plannedTime?: string;
  actualTime?: string;
  isException?: boolean;
  notes?: string;
  now: Date;
  kind: "arrival" | "pickup";
}>) {
  const status = getStudentTimeStatus({ plannedTime, actualTime, now });

  if (!status.displayTime) {
    const fallbackIcon =
      kind === "arrival" ? <ArrivalTimeIcon /> : <PickupTimeIcon />;
    return <StudentInfoRow icon={fallbackIcon}>{label}: —</StudentInfoRow>;
  }

  const icon = isException ? (
    <ExceptionIcon />
  ) : (
    renderTimeStatusIcon(status, kind)
  );

  // Keep label and time in a single text node so testing-library matchers
  // and screen-reader output read the row as one continuous string. Splitting
  // the time into its own <span> for the colored states would break exact
  // text matching even though the rendered characters are identical.
  const fullText = `${label}: ${status.displayTime} Uhr`;

  return (
    <StudentInfoRow icon={icon}>
      {status.textColor ? (
        <span style={{ color: status.textColor }}>{fullText}</span>
      ) : (
        fullText
      )}
      {notes && <span className="ml-1 text-gray-500">({notes})</span>}
    </StudentInfoRow>
  );
}

export function PickupTimeRow({
  pickupTime,
  actualTime,
  isException,
  notes,
  now,
}: Readonly<{
  pickupTime?: string;
  actualTime?: string;
  isException: boolean;
  notes?: string;
  now: Date;
}>) {
  if (isException && !pickupTime && !actualTime) {
    return (
      <StudentInfoRow icon={<ExceptionIcon />}>
        {notes || "Abwesend"}
      </StudentInfoRow>
    );
  }

  return (
    <TimeStatusRow
      label="Abholzeit"
      plannedTime={pickupTime}
      actualTime={actualTime}
      isException={isException}
      notes={notes}
      now={now}
      kind="pickup"
    />
  );
}

/**
 * Shared arrival time display row, mirror of PickupTimeRow:
 *   1. isAbsent → "Kommt heute nicht" (with reason if provided)
 *   2. Has arrivalTime → show time with urgency/exception icon
 *   3. Neither → show "Ankunftszeit: —" fallback
 */
export function ArrivalTimeRow({
  arrivalTime,
  actualTime,
  isException,
  isAbsent,
  notes,
  now,
}: Readonly<{
  arrivalTime?: string;
  actualTime?: string;
  isException: boolean;
  isAbsent: boolean;
  notes?: string;
  now: Date;
}>) {
  if (isAbsent) {
    return (
      <StudentInfoRow icon={<ExceptionIcon />}>
        {notes ? `Kommt heute nicht (${notes})` : "Kommt heute nicht"}
      </StudentInfoRow>
    );
  }

  if (isException && !arrivalTime && !actualTime) {
    return (
      <StudentInfoRow icon={<ExceptionIcon />}>
        {notes ? `Kommt heute nicht (${notes})` : "Kommt heute nicht"}
      </StudentInfoRow>
    );
  }

  return (
    <TimeStatusRow
      label="Ankunftszeit"
      plannedTime={arrivalTime}
      actualTime={actualTime}
      isException={isException}
      notes={notes}
      now={now}
      kind="arrival"
    />
  );
}
