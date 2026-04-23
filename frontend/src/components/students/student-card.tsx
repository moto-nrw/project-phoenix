// components/students/student-card.tsx
// Shared student card component used across OGS groups and active supervisions pages

import type { ReactNode } from "react";
import { Clock, AlertTriangle, Loader2 } from "lucide-react";
import { getPickupUrgency, type PickupUrgency } from "~/lib/pickup-helpers";
import { LOCATION_COLORS } from "~/lib/location-helper";
import type { StudentCheckinState } from "~/lib/hooks/use-school-checkin-mode";

interface StudentCardProps {
  /** Unique student ID */
  readonly studentId: string;
  /** Student's first name */
  readonly firstName?: string;
  /** Student's last name */
  readonly lastName?: string;
  /** Gradient class for the card overlay */
  readonly gradient?: string;
  /** Click handler for navigation (used when checkinMode is false/absent) */
  readonly onClick: () => void;
  /** Location badge component to render */
  readonly locationBadge: ReactNode;
  /** Optional extra content between name and click hint */
  readonly extraContent?: ReactNode;
  /** Optional tracking indicators (right-aligned, below location badge) */
  readonly trackingIndicators?: ReactNode;
  /**
   * When true, the card is rendered in school check-in/out mode: tinted by
   * checkinState, click fires onCheckinClick instead of onClick, the "more
   * info" hint is replaced with "Tippen zum An-/Abmelden", and a spinner
   * overlay appears while isCheckinPending is true.
   */
  readonly checkinMode?: boolean;
  /** Required when checkinMode is true — drives the tint color. */
  readonly checkinState?: StudentCheckinState;
  /** Render spinner overlay + disable the button while the API call is in flight. */
  readonly isCheckinPending?: boolean;
  /** Click handler for check-in mode. Ignored when checkinMode is false. */
  readonly onCheckinClick?: () => void;
}

// Brand-hex overrides applied to the card when checkinMode is on. Matches
// the colors the PresenceBadge uses so card + badge visually reinforce each
// other at a glance.
const CHECKIN_TINT: Record<
  StudentCheckinState,
  { border: string; background: string; focusRing: string }
> = {
  anwesend: {
    border: LOCATION_COLORS.GROUP_ROOM, // #83CD2D green
    background: `${LOCATION_COLORS.GROUP_ROOM}1a`, // ~10% alpha
    focusRing: `${LOCATION_COLORS.GROUP_ROOM}80`, // ~50% alpha for focus
  },
  schulhof: {
    border: LOCATION_COLORS.SCHOOLYARD, // #F78C10 orange
    background: `${LOCATION_COLORS.SCHOOLYARD}1a`,
    focusRing: `${LOCATION_COLORS.SCHOOLYARD}80`,
  },
  abwesend: {
    border: LOCATION_COLORS.HOME, // #FF3130 red
    background: `${LOCATION_COLORS.HOME}1a`,
    focusRing: `${LOCATION_COLORS.HOME}80`,
  },
  unknown: {
    border: LOCATION_COLORS.UNKNOWN, // gray-ish
    background: `${LOCATION_COLORS.UNKNOWN}1a`,
    focusRing: `${LOCATION_COLORS.UNKNOWN}80`,
  },
};

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
  checkinMode = false,
  checkinState = "unknown",
  isCheckinPending = false,
  onCheckinClick,
}: StudentCardProps) {
  // Active handler + a11y label swap when the page is in check-in mode.
  // Falls back to navigation when checkinMode is off, or when the caller
  // omitted onCheckinClick (defensive — allows partial adoption).
  const activeHandler =
    checkinMode && onCheckinClick ? onCheckinClick : onClick;
  const ariaLabel = checkinMode
    ? `${firstName} ${lastName} - Tippen zum An-/Abmelden`
    : `${firstName} ${lastName} - Tippen für mehr Infos`;

  // Inline styles when in check-in mode. We avoid Tailwind arbitrary
  // syntax for dynamic per-state hex values because six generated classes
  // would never be purged cleanly; inline style keeps the brand hex bound
  // to runtime state.
  const tint = checkinMode ? CHECKIN_TINT[checkinState] : null;
  const style = tint
    ? ({
        borderColor: tint.border,
        backgroundColor: tint.background,
        "--checkin-focus-ring": tint.focusRing,
      } as React.CSSProperties)
    : undefined;

  return (
    <button
      key={studentId}
      type="button"
      onClick={activeHandler}
      disabled={isCheckinPending}
      aria-label={ariaLabel}
      aria-busy={isCheckinPending}
      data-checkin-mode={checkinMode || undefined}
      data-checkin-state={checkinMode ? checkinState : undefined}
      style={style}
      className={
        checkinMode
          ? "group relative w-full cursor-pointer overflow-hidden rounded-3xl border-2 text-left shadow-[0_8px_30px_rgb(0,0,0,0.08)] backdrop-blur-md transition-all duration-150 focus:ring-2 focus:outline-none active:scale-[0.98] disabled:cursor-wait disabled:opacity-70 disabled:active:scale-100 md:hover:-translate-y-0.5 md:hover:shadow-[0_12px_40px_rgb(0,0,0,0.14)]"
          : "group relative w-full cursor-pointer overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 text-left shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-150 focus:ring-2 focus:ring-blue-500/50 focus:outline-none active:scale-[0.98] md:hover:-translate-y-0.5 md:hover:border-[#5080D8]/40 md:hover:bg-white md:hover:shadow-[0_12px_40px_rgb(0,0,0,0.18)]"
      }
    >
      {/* Decorative layers — skipped in check-in mode so the brand tint
          stays visually clean. */}
      {!checkinMode && (
        <>
          <div
            className={`absolute inset-0 bg-gradient-to-br ${gradient} rounded-3xl opacity-[0.03]`}
          />
          <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20" />
          <div className="absolute inset-0 rounded-3xl ring-1 ring-white/20 transition-all duration-150 md:group-hover:ring-blue-200/60" />
        </>
      )}

      <div className="relative p-6">
        {/* Header with student name */}
        <div className="mb-3 flex items-start justify-between gap-3">
          {/* Student Name */}
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <h3
                className={
                  checkinMode
                    ? "overflow-hidden text-lg font-bold text-ellipsis whitespace-nowrap text-gray-900"
                    : "overflow-hidden text-lg font-bold text-ellipsis whitespace-nowrap text-gray-800 transition-colors duration-150 md:group-hover:text-blue-600"
                }
              >
                {firstName}
              </h3>
              {/* Arrow hint only makes sense for navigation; in check-in
                  mode the whole card IS the action. */}
              {!checkinMode && (
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
              )}
            </div>
            <p
              className={
                checkinMode
                  ? "overflow-hidden text-base font-semibold text-ellipsis whitespace-nowrap text-gray-800"
                  : "overflow-hidden text-base font-semibold text-ellipsis whitespace-nowrap text-gray-700 transition-colors duration-150 md:group-hover:text-blue-500"
              }
            >
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

        {/* Bottom row hint — different copy per mode */}
        <div className="flex justify-start">
          <p
            className={
              checkinMode
                ? "text-xs font-medium text-gray-600"
                : "text-xs text-gray-400 transition-colors duration-150 md:group-hover:text-blue-400"
            }
          >
            {checkinMode ? "Tippen zum An-/Abmelden" : "Tippen für mehr Infos"}
          </p>
        </div>

        {/* Decorative pings — only in navigation mode */}
        {!checkinMode && (
          <>
            <div className="absolute top-3 left-3 h-5 w-5 animate-ping rounded-full bg-white/20" />
            <div className="absolute right-3 bottom-3 h-3 w-3 rounded-full bg-white/30" />
          </>
        )}

        {/* Spinner overlay while the API call is in flight. */}
        {isCheckinPending && (
          <div
            className="pointer-events-none absolute inset-0 flex items-center justify-center rounded-3xl bg-white/40 backdrop-blur-[1px]"
            data-checkin-pending="true"
          >
            <Loader2
              className="h-8 w-8 animate-spin text-gray-700"
              aria-hidden="true"
            />
          </div>
        )}
      </div>

      {/* Glowing border effect — navigation mode only */}
      {!checkinMode && (
        <div className="absolute inset-0 rounded-3xl bg-gradient-to-r from-transparent via-blue-100/30 to-transparent opacity-0 transition-opacity duration-150 md:group-hover:opacity-100" />
      )}
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
 *   1. Has pickup time → show time with urgency/exception icon
 *   2. Exception without time → show exception reason
 *   3. Neither → show "Abholzeit: —" fallback
 */
export function PickupTimeRow({
  pickupTime,
  isException,
  notes,
  isHome,
  now,
}: Readonly<{
  pickupTime?: string;
  isException: boolean;
  notes?: string;
  isHome: boolean;
  now: Date;
}>) {
  const urgency = isHome
    ? ("none" as const)
    : getPickupUrgency(pickupTime, now);

  if (pickupTime) {
    return (
      <StudentInfoRow
        icon={isException ? <ExceptionIcon /> : renderPickupIcon(urgency)}
      >
        Abholzeit: {pickupTime} Uhr
        {notes && <span className="ml-1 text-gray-500">({notes})</span>}
      </StudentInfoRow>
    );
  }

  if (isException) {
    return (
      <StudentInfoRow icon={<ExceptionIcon />}>
        {notes || "Abwesend"}
      </StudentInfoRow>
    );
  }

  return (
    <StudentInfoRow icon={<PickupTimeIcon />}>Abholzeit: —</StudentInfoRow>
  );
}

/** Renders the appropriate pickup icon based on urgency level */
export function renderPickupIcon(urgency: PickupUrgency): ReactNode {
  if (urgency === "overdue") {
    return <AlertTriangle className="h-3.5 w-3.5 text-red-500" />;
  }
  if (urgency === "soon") {
    return <Clock className="h-3.5 w-3.5 animate-pulse text-orange-500" />;
  }
  // normal / none — default gray clock
  return <PickupTimeIcon />;
}
