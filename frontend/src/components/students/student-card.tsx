// components/students/student-card.tsx
// Shared student card component used across OGS groups and active supervisions pages

import type { ReactNode } from "react";
import {
  Clock,
  AlertTriangle,
  Check,
  ChevronRight,
  Loader2,
  LogIn,
  Minus,
  Plus,
} from "lucide-react";
import {
  getStudentTimeStatus,
  type StudentTimeStatus,
} from "~/lib/student-time-status";
import {
  LOCATION_COLORS,
  MOTO_COLOR_PALETTE,
  getLocationBadgeTone,
} from "~/lib/location-helper";
import type { StudentCheckinState } from "~/lib/hooks/use-school-checkin-mode";
import { Avatar } from "~/components/ui/avatar";
import { useStudentPhotosEnabled } from "~/lib/hooks/use-student-photos-enabled";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";

interface StudentCardProps {
  /** Unique student ID */
  readonly studentId: string;
  /** Student's first name */
  readonly firstName?: string;
  /** Student's last name */
  readonly lastName?: string;
  /**
   * Photo URL (gated server-side by operations.student_photos_enabled +
   * parental consent). When falsy the avatar falls back to brand initials,
   * so callers can pass `student.photo_url` unconditionally without
   * checking the feature flag themselves.
   */
  readonly photoUrl?: string | null;
  /** Legacy visual accent prop retained for existing callers. */
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
   * When true, the card is rendered in school check-in/out mode: the bottom
   * hint area becomes a coloured tap-strip whose copy + colour follow
   * checkinState, click fires onCheckinClick instead of onClick, and a
   * spinner replaces the strip while isCheckinPending is true. The card
   * body stays neutral white — the tap-strip carries the visual signal.
   */
  readonly checkinMode?: boolean;
  /** Required when checkinMode is true — drives the tap-strip colour + copy. */
  readonly checkinState?: StudentCheckinState;
  /** Render spinner inside the tap-strip and disable the button while the API call is in flight. */
  readonly isCheckinPending?: boolean;
  /** Click handler for check-in mode. Ignored when checkinMode is false. */
  readonly onCheckinClick?: () => void;
}

// Tap-strip styles for the bottom action area when checkinMode is on. The
// approach mirrors the time-tracking page's pill-pattern (subtle bg + brand
// hex text) so the visual stays consistent with the rest of the app — never
// flat-fill the card. Hex literals come from LOCATION_COLORS (CLAUDE.md §0).
const TAP_STRIP_STYLES: Record<
  StudentCheckinState,
  {
    background: string;
    text: string;
    copy: string;
    action: "anmelden" | "abmelden";
  }
> = {
  anwesend: {
    // Currently present → tap to check out (red accent communicates the destination state)
    background: MOTO_COLOR_PALETTE.red.soft,
    text: MOTO_COLOR_PALETTE.red.strong,
    copy: "Tippen zum Abmelden",
    action: "abmelden",
  },
  schulhof: {
    // On schoolyard counts as present → tap also checks out
    background: MOTO_COLOR_PALETTE.red.soft,
    text: MOTO_COLOR_PALETTE.red.strong,
    copy: "Tippen zum Abmelden",
    action: "abmelden",
  },
  abwesend: {
    // Currently absent → tap to check in (green accent for the destination state)
    background: MOTO_COLOR_PALETTE.green.soft,
    text: MOTO_COLOR_PALETTE.green.strong,
    copy: "Tippen zum Anmelden",
    action: "anmelden",
  },
  unknown: {
    background: `${LOCATION_COLORS.UNKNOWN}26`,
    text: MOTO_COLOR_PALETTE.neutral.strong,
    copy: "Tippen zum An-/Abmelden",
    action: "anmelden",
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
  photoUrl,
  gradient,
  onClick,
  locationBadge,
  extraContent,
  trackingIndicators,
  checkinMode = false,
  checkinState = "unknown",
  isCheckinPending = false,
  onCheckinClick,
}: StudentCardProps) {
  // Photo feature is per-tenant. When the school hasn't enabled it the
  // entire avatar overlay is suppressed — the card falls back to its
  // pre-feature shape including the bottom-right decorative ping.
  const { enabled: photosEnabled } = useStudentPhotosEnabled();
  // In checkinMode the card click triggers the toggle; otherwise it
  // navigates to the detail page. Falls back to navigation if the page
  // forgot to wire onCheckinClick (defensive — allows partial adoption).
  const activeHandler =
    checkinMode && onCheckinClick ? onCheckinClick : onClick;

  // Pull a Tap-Strip preset for the bottom action area in checkinMode.
  // The strip carries the colour signal; the card body stays neutral white.
  const tapStrip = checkinMode ? TAP_STRIP_STYLES[checkinState] : null;

  const ariaLabel = checkinMode
    ? `${firstName} ${lastName} - ${tapStrip?.copy ?? "Tippen zum An-/Abmelden"}`
    : `${firstName} ${lastName} - Tippen für mehr Infos`;
  void gradient;

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
      className={`group moto-content-surface moto-hover-elevated relative w-full cursor-pointer overflow-hidden rounded-2xl border border-gray-200 bg-white text-left shadow-[0_1px_2px_rgba(15,23,42,0.04),0_0_0_1px_rgba(15,23,42,0.02)] focus-visible:ring-2 focus-visible:ring-gray-300 focus-visible:ring-offset-2 focus-visible:outline-none disabled:cursor-wait disabled:opacity-70 ${
        // Click-time scale skipped in check-in mode: sub-pixel aliasing
        // during the scale animation flashes a 1px white seam at the
        // body→tap-strip boundary. Pending spinner gives the click feedback.
        checkinMode ? "" : "active:shadow-[0_10px_26px_rgba(15,23,42,0.1)]"
      }`}
    >
      <div className={`relative ${checkinMode ? "p-6 pb-0" : "p-6 pb-5"}`}>
        {!checkinMode && (
          <div className="pointer-events-none absolute inset-0 rounded-2xl ring-1 ring-transparent transition-[box-shadow] duration-300 ease-[cubic-bezier(0.2,0.8,0.2,1)] md:group-hover:shadow-[inset_0_1px_0_rgba(255,255,255,0.9)]" />
        )}

        {/* Card body content sits above the decorative layers */}
        <div className="relative">
          {/* Header with student name */}
          <div className="mb-3 flex items-start justify-between gap-3">
            {/* Student Name */}
            <div className="min-w-0 flex-1">
              {/* Avatar moved to top-left, inline with first/last name so only
                  the name gets pushed aside. extraContent below stays full
                  width. */}
              <div className="flex items-start gap-3">
                {photosEnabled && (
                  <Avatar
                    imageUrl={photoUrl ?? null}
                    name={`${firstName ?? ""} ${lastName ?? ""}`.trim() || "?"}
                    size="md"
                    className="flex-shrink-0"
                  />
                )}
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <h3 className="inline-block origin-left overflow-hidden text-lg font-bold text-ellipsis whitespace-nowrap text-gray-800 transition-[color,transform] duration-300 ease-[cubic-bezier(0.2,0.8,0.2,1)] motion-reduce:transition-none md:group-hover:scale-[1.025] md:group-hover:text-gray-950 motion-reduce:md:group-hover:scale-100">
                      {firstName}
                    </h3>
                    {/* Arrow hint only points to navigation; in check-in mode the
                      bottom strip carries the action signal instead. */}
                    {!checkinMode && (
                      <ChevronRight
                        className="h-4 w-4 flex-shrink-0 translate-x-0 text-gray-300 opacity-70 transition-[color,opacity,transform] duration-300 ease-[cubic-bezier(0.2,0.8,0.2,1)] motion-reduce:transition-none md:group-hover:translate-x-0.5 md:group-hover:text-gray-600 md:group-hover:opacity-100 motion-reduce:md:group-hover:translate-x-0"
                        aria-hidden="true"
                      />
                    )}
                  </div>
                  <p className="overflow-hidden text-base font-semibold text-ellipsis whitespace-nowrap text-gray-700 transition-colors duration-200 md:group-hover:text-gray-800">
                    {lastName}
                  </p>
                </div>
              </div>
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

          {/* Bottom hint in navigation mode only. */}
          {!checkinMode && (
            <p className="text-xs text-gray-400 transition-colors duration-200 md:group-hover:text-gray-500">
              Tippen für mehr Infos
            </p>
          )}
        </div>
      </div>

      {/* Tap-Strip — full-width action area at the bottom in check-in mode.
          Hex literals come via TAP_STRIP_STYLES from LOCATION_COLORS so the
          colour signalling stays inside the brand palette (CLAUDE.md §0).
          Layout stays identical between idle and pending states (icon swaps,
          text stays) so the strip's height never changes — otherwise the
          card resizes mid-toggle and a 1px white seam flashes at the
          body→strip boundary. */}
      {checkinMode && tapStrip && (
        <div
          className="relative flex min-h-[44px] items-center justify-center gap-2 px-4 py-3 text-sm font-semibold transition-colors duration-150"
          style={{
            backgroundColor: tapStrip.background,
            color: tapStrip.text,
          }}
          data-checkin-tap-strip="true"
        >
          {isCheckinPending ? (
            <Loader2
              className="h-4 w-4 flex-shrink-0 animate-spin"
              aria-hidden="true"
            />
          ) : tapStrip.action === "anmelden" ? (
            <Plus className="h-4 w-4 flex-shrink-0" aria-hidden="true" />
          ) : (
            <Minus className="h-4 w-4 flex-shrink-0" aria-hidden="true" />
          )}
          <span>{tapStrip.copy}</span>
        </div>
      )}
    </button>
  );
}

/** Icon for school class display */
export function SchoolClassIcon() {
  return <MotoConceptIcon concept="schools" size={14} />;
}

/** Icon for group display */
export function GroupIcon() {
  return <MotoConceptIcon concept="groups" size={14} />;
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
  return <MotoConceptIcon concept="careTimes" size={14} />;
}

/** Icon for exception indicator */
export function ExceptionIcon() {
  return <MotoConceptIcon concept="pickup" size={14} />;
}

/**
 * Neutral marker for "not coming today" rows (sick, excused, or a schedule
 * exception with no time). A known absence is information, not a warning — a
 * calm gray clock matches the student detail page and avoids the alarm an
 * amber triangle implies. Used for every absence line so the same state always
 * reads the same, across cards and detail.
 */
export function AbsenceIcon() {
  return <Clock className="h-3.5 w-3.5 text-gray-400" />;
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
      <StudentInfoRow icon={<AbsenceIcon />}>
        {notes || "Abwesend"}
      </StudentInfoRow>
    );
  }

  return (
    <TimeStatusRow
      label="Gehzeit"
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
 * Single neutral status line shown instead of the arrival + pickup rows when a
 * student is absent today (sick / excused). One line for the whole day — an
 * absent child has neither a meaningful arrival nor pickup time, so repeating
 * the reason on two rows would be noise. Mirrors the existing schedule-based
 * "Kommt heute nicht (…)" wording so every absence reads identically.
 */
export function StudentAbsenceRow({
  label,
  wording = "Kommt heute nicht",
}: Readonly<{
  label: string;
  /**
   * Leading phrase before the reason. The default fits the today view; a
   * non-today planning date (#1939) passes "Kommt nicht" so the card never
   * says "heute" about another day.
   */
  wording?: string;
}>) {
  return (
    <StudentInfoRow icon={<AbsenceIcon />}>
      {`${wording} (${label})`}
    </StudentInfoRow>
  );
}

/**
 * Informational badge for a child with a still-pending "entschuldigt" request
 * covering today (operations.parent_excused_requires_approval). It is NOT an
 * absence — the child stays "expected" and keeps its normal arrival/pickup rows
 * until the OGS confirms — so this renders as a single compact amber pill
 * alongside them, not in place of them. The parent's note (if any) is kept to
 * the hover title so the card stays as dense as its other rows. Amber hex comes
 * from LOCATION_COLORS.SICK (CLAUDE.md §0).
 */
export function StudentPendingExcusedRow({
  note,
}: Readonly<{ note?: string }>) {
  const tone = getLocationBadgeTone(LOCATION_COLORS.SICK);

  // Leading icon at the row's left edge (aligned with the other StudentInfoRow
  // icons) and the amber pill in the text column, so this line sits in the same
  // rhythm as the sibling rows instead of looking offset.
  return (
    <div className="mt-1 flex items-center gap-1.5">
      <span className="flex-shrink-0">
        <AbsenceIcon />
      </span>
      <span
        className="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium"
        style={{
          backgroundColor: tone.backgroundColor,
          color: tone.textColor,
        }}
        title={note ?? undefined}
      >
        Freigabe ausstehend
      </span>
    </div>
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
  absentWording = "Kommt heute nicht",
}: Readonly<{
  arrivalTime?: string;
  actualTime?: string;
  isException: boolean;
  isAbsent: boolean;
  notes?: string;
  now: Date;
  /** See StudentAbsenceRow — date-neutral phrase for non-today views (#1939). */
  absentWording?: string;
}>) {
  if (isAbsent) {
    return (
      <StudentInfoRow icon={<AbsenceIcon />}>
        {notes ? `${absentWording} (${notes})` : absentWording}
      </StudentInfoRow>
    );
  }

  if (isException && !arrivalTime && !actualTime) {
    return (
      <StudentInfoRow icon={<AbsenceIcon />}>
        {notes ? `${absentWording} (${notes})` : absentWording}
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
