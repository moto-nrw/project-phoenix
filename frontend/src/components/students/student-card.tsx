// components/students/student-card.tsx
// Shared student card component used across OGS groups and active supervisions pages

import { useDataPrefetchIntent } from "~/components/ui/data-prefetch-intent";
import type { ReactNode } from "react";
import { TILE_CARD_SURFACE } from "~/components/ui/tile-card";
import {
  Clock,
  AlertTriangle,
  Check,
  ChevronRight,
  Loader2,
  LogIn,
  Minus,
  Plus,
  Route,
  StickyNote,
} from "lucide-react";
import {
  CalendarXIcon,
  ChalkboardTeacherIcon,
  ListChecksIcon,
  UsersThreeIcon,
  WarningCircleIcon,
} from "@phosphor-icons/react";
import {
  getStudentTimeStatus,
  ONLY_IF_LESSON_CANCELLED_LABEL,
  type StudentDayTimes,
  type StudentTimeStatus,
} from "~/lib/student-time-status";
import {
  GROUP_ROOM_SHADES,
  LOCATION_COLORS,
  MOTO_COLOR_PALETTE,
} from "~/lib/location-helper";
import type { StudentCheckinState } from "~/lib/hooks/use-school-checkin-mode";
import { Avatar } from "~/components/ui/avatar";
import { useStudentPhotosEnabled } from "~/lib/hooks/use-student-photos-enabled";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { useRowClock } from "~/components/students/student-card-clock";
import {
  DEPARTURE_MODE_LABELS,
  type DepartureMode,
} from "~/lib/student-helpers";

/**
 * What changed for today only (#3946): the Gehzeit, the Abholart (#3945) or
 * both. A change to the weekly plan is none of them.
 */
export interface DayHomeChange {
  readonly time: boolean;
  /** The new Abholart; absent when only the Gehzeit changed. */
  readonly mode?: DepartureMode;
}

function dayHomeChangeKind(change: DayHomeChange): "time" | "mode" | "both" {
  if (!change.mode) return "time";
  return change.time ? "both" : "mode";
}

/**
 * Visible hint of a one-day change (#3946). The new Gehzeit stands in its
 * row; the new Abholart has no row on every page, so the hint names it. The
 * value comes apart from the text so a narrow card wraps before it, not
 * inside it.
 */
function dayHomeChangeParts(change: DayHomeChange): {
  readonly text: string;
  readonly mode?: string;
} {
  if (!change.mode) return { text: "Gehzeit heute geändert" };
  const changed = change.time ? "Gehzeit und Abholart" : "Abholart";
  return {
    text: `${changed} heute geändert:`,
    mode: DEPARTURE_MODE_LABELS[change.mode],
  };
}

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
  /**
   * Selection sub-mode of check-in mode (#2359): a tap marks the card instead
   * of firing an API call, the tap-strip carries the selection state, and the
   * selected card gets a brand-green ring. Only meaningful with checkinMode.
   */
  readonly checkinSelectMode?: boolean;
  /** Whether this card is currently marked for the next bulk action. */
  readonly isCheckinSelected?: boolean;
  /**
   * Today's Gehzeit or Abholart differs from the weekly plan for this one day
   * (#3946). A full-width strip at the bottom of the card says so, in the
   * shape of the check-in tap strip, so staff see the change without reading
   * every row. Callers decide via `dayHomeChange`.
   */
  readonly dayHomeChange?: DayHomeChange;
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
  onClick,
  locationBadge,
  extraContent,
  trackingIndicators,
  checkinMode = false,
  checkinState = "unknown",
  isCheckinPending = false,
  onCheckinClick,
  checkinSelectMode = false,
  isCheckinSelected = false,
  dayHomeChange,
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

  // Selection sub-mode: taps mark the card, nothing is written yet (#2359).
  const selectMode = checkinMode && checkinSelectMode;

  // Pull a Tap-Strip preset for the bottom action area in checkinMode.
  // The strip carries the colour signal; the card body stays neutral white.
  const tapStrip = checkinMode ? TAP_STRIP_STYLES[checkinState] : null;
  // The button's label replaces its content for screen readers, so the
  // one-day change must be part of it (#3946).
  const dayChange = dayHomeChange
    ? dayHomeChangeParts(dayHomeChange)
    : undefined;
  const dayChangeLabel = dayChange
    ? [dayChange.text, dayChange.mode].filter(Boolean).join(" ")
    : undefined;
  const dayChangeKind = dayHomeChange
    ? dayHomeChangeKind(dayHomeChange)
    : undefined;
  const nameLabel = dayChangeLabel
    ? `${firstName} ${lastName}, ${dayChangeLabel}`
    : `${firstName} ${lastName}`;
  const ariaLabel = selectMode
    ? `${nameLabel} - ${isCheckinSelected ? "Auswahl entfernen" : "Auswählen"}`
    : checkinMode
      ? `${nameLabel} - ${tapStrip?.copy ?? "Tippen zum An-/Abmelden"}`
      : `${nameLabel} - Details öffnen`;

  // A selected card gets a brand-green ring on top of the resting shadow.
  // Inline style (not an arbitrary-value class) so the hex stays sourced from
  // LOCATION_COLORS.
  const isSelectedCard = selectMode && isCheckinSelected;
  const selectedRingStyle: React.CSSProperties | undefined = isSelectedCard
    ? {
        boxShadow: `0 0 0 1px ${LOCATION_COLORS.GROUP_ROOM}`,
        borderColor: "transparent",
      }
    : undefined;
  const hasTapStrip = checkinMode && !selectMode;
  const dataIntent = useDataPrefetchIntent(
    checkinMode || selectMode ? null : `/students/${studentId}`,
  );

  return (
    <button
      key={studentId}
      type="button"
      {...dataIntent}
      onClick={activeHandler}
      disabled={isCheckinPending}
      aria-label={ariaLabel}
      aria-busy={isCheckinPending}
      aria-pressed={selectMode ? isCheckinSelected : undefined}
      data-checkin-mode={checkinMode || undefined}
      data-setup-tour="student-card"
      data-checkin-state={checkinMode ? checkinState : undefined}
      data-checkin-selected={isSelectedCard || undefined}
      data-day-home-change={dayChangeKind}
      style={selectedRingStyle}
      className={`${TILE_CARD_SURFACE} flex cursor-pointer flex-col disabled:cursor-wait disabled:opacity-70`}
    >
      <div
        className={`relative ${hasTapStrip || dayChangeLabel ? "p-3 pb-0 sm:p-5 sm:pb-0" : "p-3 sm:p-5"}`}
      >
        {/* Card body content */}
        <div className="relative">
          {/* Header with student name */}
          <div className="mb-3 flex items-start justify-between gap-3">
            {/* Student Name */}
            <div className="min-w-0 flex-1">
              {/* Avatar moved to top-left, inline with first/last name so only
                  the name gets pushed aside. extraContent below stays full
                  width. */}
              <div className="flex items-start gap-2.5">
                {selectMode ? (
                  <span
                    className={`mt-0.5 flex size-5 shrink-0 items-center justify-center rounded-full border transition-[background-color,border-color,color] duration-150 ${
                      isCheckinSelected
                        ? "border-transparent text-white"
                        : "border-gray-300 bg-white text-transparent"
                    }`}
                    style={
                      isCheckinSelected
                        ? { backgroundColor: GROUP_ROOM_SHADES.fill }
                        : undefined
                    }
                    data-checkin-selection-indicator="true"
                    aria-hidden="true"
                  >
                    <Check className="size-3.5" strokeWidth={3} />
                  </span>
                ) : null}
                {photosEnabled && (
                  <Avatar
                    imageUrl={photoUrl ?? null}
                    name={`${firstName ?? ""} ${lastName ?? ""}`.trim() || "?"}
                    size="md"
                    className="flex-shrink-0"
                  />
                )}
                <div className="min-w-0 flex-1">
                  {/* Ein Name, eine Zeile — dieselbe Bauart wie die
                      Personal-Kachel. Der Umbruch zwischen Vor- und Nachname
                      ließ jede Karte wie zwei Personen aussehen. */}
                  <div className="flex min-w-0 items-center gap-1.5">
                    <h3 className="truncate text-base font-bold text-gray-900">
                      {firstName} {lastName}
                    </h3>
                    {/* Arrow hint only points to navigation; in check-in mode the
                      bottom strip carries the action signal instead. */}
                    {!checkinMode && (
                      <ChevronRight
                        className="h-4 w-4 flex-shrink-0 text-gray-300 transition-colors duration-200 md:group-hover:text-gray-500"
                        aria-hidden="true"
                      />
                    )}
                  </div>
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
        </div>
      </div>

      {/* Tap-Strip — full-width action area at the bottom in check-in mode.
          Hex literals come via TAP_STRIP_STYLES from LOCATION_COLORS so the
          colour signalling stays inside the brand palette (CLAUDE.md §0).
          Layout stays identical between idle and pending states (icon swaps,
          text stays) so the strip's height never changes — otherwise the
          card resizes mid-toggle and a 1px white seam flashes at the
          body→strip boundary. */}
      {/* One-day change (#3946): an information strip in the shape of the tap
          strip, left-aligned like the rows above so it reads as a fact, not
          an action. In check-in mode it sits above the tap strip. The icon is
          the one of the row that changed: the clock of the Gehzeit, the route
          of the Abholart. */}
      {dayChangeLabel ? (
        <div
          className="mt-auto flex min-h-[44px] items-center gap-2 px-3 py-2.5 text-sm font-semibold sm:px-5"
          style={{
            backgroundColor: MOTO_COLOR_PALETTE.orange.soft,
            color: MOTO_COLOR_PALETTE.orange.strong,
          }}
          data-day-home-change-strip="true"
        >
          {dayHomeChange?.mode === undefined ? (
            <Clock className="h-4 w-4 flex-shrink-0" aria-hidden="true" />
          ) : (
            <Route className="h-4 w-4 flex-shrink-0" aria-hidden="true" />
          )}
          <span>
            {dayChange?.text}
            {dayChange?.mode ? (
              <>
                {" "}
                <span className="whitespace-nowrap">{dayChange.mode}</span>
              </>
            ) : null}
          </span>
        </div>
      ) : null}

      {hasTapStrip && tapStrip ? (
        <div
          className={`relative ${dayChangeLabel ? "" : "mt-auto"} flex min-h-[44px] items-center justify-center gap-2 px-4 py-3 text-sm font-semibold transition-colors duration-150`}
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
      ) : null}
    </button>
  );
}

/** Icon for school class display */
export function SchoolClassIcon() {
  return (
    <MotoDuotoneIcon icon={ChalkboardTeacherIcon} tone="neutral" size={14} />
  );
}

/** Icon for group display */
export function GroupIcon() {
  return <MotoDuotoneIcon icon={UsersThreeIcon} tone="neutral" size={14} />;
}

/** Icon for activity (Angebot) display — the app's `activities` glyph */
export function ActivityIcon() {
  return <MotoDuotoneIcon icon={ListChecksIcon} tone="neutral" size={14} />;
}

export function DepartureModeIcon() {
  return <Route className="h-3.5 w-3.5 text-gray-400" aria-hidden="true" />;
}

/**
 * Where a row renders: on a Kinderkarte (small grey line under the name) or
 * in a table cell (#3834), where the column header already names the value.
 */
export type StudentRowVariant = "card" | "cell" | "compact";

/** Reusable info row for school class or group */
export function StudentInfoRow({
  icon,
  children,
  wrap = false,
  variant = "card",
}: Readonly<{
  icon: ReactNode;
  children: ReactNode;
  wrap?: boolean;
  variant?: StudentRowVariant;
}>) {
  return (
    <div
      className={`flex gap-1.5 ${variant === "card" ? "mt-1" : ""} ${wrap ? "items-start" : "items-center"}`}
    >
      {/* The one-line phone list (#3834) has no room for the icon; the
          coloured text still carries the state. */}
      {variant === "compact" ? null : (
        <span className="flex-shrink-0">{icon}</span>
      )}
      <span
        className={`${variant === "card" ? "text-xs text-gray-500" : "text-sm text-gray-700"} font-medium ${
          // A table cell wraps a long value ("Kommt heute nicht (kein Plan
          // für heute)") instead of widening the whole table (#3834).
          wrap || variant === "cell"
            ? "whitespace-normal"
            : "overflow-hidden text-ellipsis whitespace-nowrap"
        }`}
      >
        {children}
      </span>
    </div>
  );
}

/**
 * Icon for pickup time display. Same outline clock as the with-time state in
 * renderTimeStatusIcon, so a "Gehzeit" row always carries the same glyph no
 * matter whether it shows a time, a dash, or a colored status.
 */
export function PickupTimeIcon() {
  return <Clock className="h-3.5 w-3.5 text-gray-400" />;
}

/** Icon for exception indicator */
export function ExceptionIcon() {
  return <MotoDuotoneIcon icon={WarningCircleIcon} tone="orange" size={14} />;
}

/**
 * Neutral marker for "not coming today" rows (sick, excused, or a schedule
 * exception with no time). A known absence is information, not a warning, so
 * it stays calm gray and avoids the alarm an amber triangle implies. Uses the
 * CalendarX glyph of the notArrival concept instead of a clock, so absence
 * lines never share their icon with the Gehzeit rows. Used for every absence
 * line so the same state always reads the same, across cards and detail.
 */
export function AbsenceIcon() {
  return <MotoDuotoneIcon icon={CalendarXIcon} tone="neutral" size={14} />;
}

/**
 * Shared pickup time display row used across OGS groups, active supervisions,
 * and student search pages. Normalizes the three rendering branches:
 *   1. Has planned or actual time → show status-aware time row
 *   2. Exception without time → show exception reason
 *   3. Neither → show "Abholzeit: –" fallback
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
  day,
  variant = "card",
}: Readonly<{
  label: string;
  plannedTime?: string;
  actualTime?: string;
  isException?: boolean;
  notes?: string;
  now: Date;
  kind: "arrival" | "pickup";
  day?: StudentDayTimes;
  variant?: StudentRowVariant;
}>) {
  const status = getStudentTimeStatus({
    plannedTime,
    actualTime,
    now,
    kind,
    day,
  });

  if (!status.displayTime) {
    const fallbackIcon =
      kind === "arrival" ? <ArrivalTimeIcon /> : <PickupTimeIcon />;
    return (
      <StudentInfoRow icon={fallbackIcon} variant={variant}>
        {variant === "card" ? `${label}: –` : "–"}
      </StudentInfoRow>
    );
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
  // A day without care time says so instead of a time: the card row is too
  // narrow for time plus explanation, and the explanation is the news (#3373).
  // In a table cell the column header carries the label (#3834); the
  // one-line phone list shows the bare time.
  const timeText =
    variant === "compact" ? status.displayTime : `${status.displayTime} Uhr`;
  const fullText =
    status.state === "only-if-lesson-cancelled"
      ? ONLY_IF_LESSON_CANCELLED_LABEL
      : variant === "card"
        ? `${label}: ${timeText}`
        : timeText;

  // Auf der Karte bricht die Zeile um, statt abzuschneiden: Ein breites
  // Ortsetikett oder eine Notiz ließ sonst nur „Ankunftszeit: 08:…" übrig
  // (#3893).
  return (
    <StudentInfoRow icon={icon} variant={variant} wrap={variant === "card"}>
      {status.textColor ? (
        <span style={{ color: status.textColor }}>{fullText}</span>
      ) : (
        fullText
      )}
      {notes && variant !== "compact" && (
        <span className="ml-1 text-gray-500">({notes})</span>
      )}
    </StudentInfoRow>
  );
}

export function PickupTimeRow({
  pickupTime,
  actualTime,
  isException,
  notes,
  now,
  day,
  variant = "card",
}: Readonly<{
  pickupTime?: string;
  actualTime?: string;
  isException: boolean;
  notes?: string;
  /** Omit inside a `StudentCardClockProvider` — see `useRowClock` (#2975). */
  now?: Date;
  /** The whole day; without a check-in the pickup cannot be late (#3373). */
  day?: StudentDayTimes;
  variant?: StudentRowVariant;
}>) {
  const rowNow = useRowClock(now);

  if (isException && !pickupTime && !actualTime) {
    return (
      <StudentInfoRow icon={<AbsenceIcon />} variant={variant}>
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
      now={rowNow}
      kind="pickup"
      day={day}
      variant={variant}
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
  note,
  variant = "card",
}: Readonly<{
  label: string;
  /**
   * The day's note, shown on its own line below (#3369): staff see at a glance
   * that the child does not come AND why. A day without pickup time can carry
   * one, so the absence line must not swallow it.
   */
  note?: string;
  /**
   * Leading phrase before the reason. The default fits the today view; a
   * non-today planning date (#1939) passes "Kommt nicht" so the card never
   * says "heute" about another day.
   */
  wording?: string;
  variant?: StudentRowVariant;
}>) {
  return (
    <>
      <StudentInfoRow icon={<AbsenceIcon />} variant={variant}>
        {`${wording} (${label})`}
      </StudentInfoRow>
      {/* The one-line phone list has no room for the note (#3834). */}
      {note && variant !== "compact" ? (
        <StudentInfoRow
          icon={<StickyNote className="h-3.5 w-3.5 text-gray-400" />}
          wrap
          variant={variant}
        >
          Notiz: {note}
        </StudentInfoRow>
      ) : null}
    </>
  );
}

/**
 * Informational line for a child with a still-pending absence request covering
 * today. It is NOT an absence — the child stays "expected" and keeps its
 * normal arrival/pickup rows until the OGS confirms — so this renders as one
 * row alongside them, not in place of them. Plain text, not a pill: a card and
 * a table row carry at most one pill, the location (#4371). The parent's note
 * (if any) is kept to the hover title so the card stays as dense as its other
 * rows.
 */
export function StudentPendingExcusedRow({
  note,
}: Readonly<{ note?: string }>) {
  return (
    <div className="mt-1 flex items-center gap-1.5" title={note ?? undefined}>
      <span className="flex-shrink-0">
        <AbsenceIcon />
      </span>
      <span className="text-xs font-medium whitespace-nowrap text-gray-600">
        Freigabe ausstehend
      </span>
    </div>
  );
}

/**
 * Shared arrival time display row, mirror of PickupTimeRow:
 *   1. isAbsent → "Kommt heute nicht" (with reason if provided)
 *   2. Has arrivalTime → show time with urgency/exception icon
 *   3. Neither → show "Ankunftszeit: –" fallback
 */
export function ArrivalTimeRow({
  arrivalTime,
  actualTime,
  isException,
  isAbsent,
  notes,
  now,
  absentWording = "Kommt heute nicht",
  day,
  variant = "card",
}: Readonly<{
  arrivalTime?: string;
  actualTime?: string;
  isException: boolean;
  isAbsent: boolean;
  notes?: string;
  /** Omit inside a `StudentCardClockProvider` — see `useRowClock` (#2975). */
  now?: Date;
  /** See StudentAbsenceRow — date-neutral phrase for non-today views (#1939). */
  absentWording?: string;
  /** The whole day; arrival not before pickup means no care time (#3373). */
  day?: StudentDayTimes;
  variant?: StudentRowVariant;
}>) {
  const rowNow = useRowClock(now);

  if (isAbsent) {
    return (
      <StudentInfoRow icon={<AbsenceIcon />} variant={variant}>
        {notes ? `${absentWording} (${notes})` : absentWording}
      </StudentInfoRow>
    );
  }

  if (isException && !arrivalTime && !actualTime) {
    return (
      <StudentInfoRow icon={<AbsenceIcon />} variant={variant}>
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
      now={rowNow}
      kind="arrival"
      day={day}
      variant={variant}
    />
  );
}
