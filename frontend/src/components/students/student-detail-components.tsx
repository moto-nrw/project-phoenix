"use client";

import type React from "react";
import { useEffect, useState } from "react";
import useSWR from "swr";
import { AlertTriangle, Check, Clock, Info } from "lucide-react";
import { LocationBadge } from "@/components/ui/location-badge";
import type { ExtendedStudent } from "~/lib/hooks/use-student-data";
import { useMinuteClock } from "~/lib/pickup-helpers";
import {
  getStudentAbsence,
  getStudentTimeStatus,
} from "~/lib/student-time-status";
import {
  getDayPlanningNotComingLabel,
  getStudentPresenceBadgePlanning,
} from "~/lib/day-planning-helper";
import type { SupervisorContact } from "~/lib/student-helpers";
import type { StudentEnrollmentExtraFieldGroup } from "~/lib/student-api";
import {
  allowedDepartureModesFromDeparture,
  departureDaysFromLegacy,
} from "~/lib/student-helpers";
import { formatCustomValue } from "~/lib/enrollment-custom-value-format";
import { AllowedDepartureModesDisplay } from "~/components/students/allowed-departure-modes-display";
import { InfoCard, InfoItem } from "~/components/ui/info-card";
import {
  companionDisplayName,
  fetchStudentCompanions,
  formatCompanionWeekdays,
  subscribeStudentCompanionDisplayChanged,
  subscribeStudentCompanionsChanged,
  type StudentCompanion,
} from "~/lib/student-companion-api";
import { Avatar } from "~/components/ui/avatar";
import { PageIntro } from "~/components/ui/page-intro";
import { Button } from "~/components/ui/button";
import {
  ParentVisibleBadge,
  ParentVisibilityLegend,
} from "~/components/ui/parent-visible-badge";
import { PARENT_VISIBLE_HINTS } from "~/lib/parent-visible-fields";
import { useStudentPhotosEnabled } from "~/lib/hooks/use-student-photos-enabled";
import { createLogger } from "~/lib/logger";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";
import { MOTO_CONCEPTS, type MotoConceptKey } from "~/lib/moto-concepts";

const logger = createLogger({ component: "StudentDetailComponents" });

// =============================================================================
// ICONS - Reusable SVG icons
// =============================================================================

export function PersonIcon({
  className = "h-5 w-5",
}: Readonly<{ className?: string }>) {
  const concept = MOTO_CONCEPTS.children;
  return (
    <MotoDuotoneIcon
      icon={concept.icon}
      tone={concept.tone}
      size={20}
      className={className}
    />
  );
}

function ViewOnlyIcon({
  className = "h-3 w-3 sm:h-3.5 sm:w-3.5",
}: Readonly<{ className?: string }>) {
  return (
    <svg
      className={className}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
      />
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
      />
    </svg>
  );
}

function EmailIcon({
  className = "h-4 w-4",
}: Readonly<{ className?: string }>) {
  return (
    <svg
      className={className}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
      />
    </svg>
  );
}

function ClockIcon({
  className = "h-5 w-5",
}: Readonly<{ className?: string }>) {
  return (
    <svg
      className={className}
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

// =============================================================================
// FIELD HISTORY INFO (ⓘ next to a field — issue #1455)
// =============================================================================

interface FieldChangeEntry {
  id: string;
  field: string;
  edited_by: string;
  changed_at: string;
}

const changeHistoryFetcher = async (
  url: string,
): Promise<FieldChangeEntry[]> => {
  const res = await fetch(url);
  if (!res.ok) throw new Error(`change-history ${res.status}`);
  const body = (await res.json()) as { data?: FieldChangeEntry[] };
  return body.data ?? [];
};

/**
 * Discreet ⓘ shown next to a tracked field. On hover it reveals who last
 * changed the field and when — the quick-answer use case from issue #1455.
 * Renders nothing when there is no recorded change (or no access). All
 * instances for one student share a single request via SWR's cache key.
 */
function FieldHistoryInfo({
  studentId,
  fields,
}: Readonly<{ studentId: string; fields: readonly string[] }>) {
  const { data } = useSWR<FieldChangeEntry[]>(
    `/api/students/${studentId}/change-history`,
    changeHistoryFetcher,
    { shouldRetryOnError: false, revalidateOnFocus: false },
  );

  const [open, setOpen] = useState(false);

  // The API returns entries newest-first, so the first match is the latest.
  const latest = data?.find((entry) => fields.includes(entry.field));
  if (!latest) return null;

  const when = new Date(latest.changed_at).toLocaleDateString("de-DE", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });

  return (
    <span className="relative mt-0.5 inline-flex flex-shrink-0">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        onBlur={() => setOpen(false)}
        aria-label="Änderungsinformation anzeigen"
        className="text-moto-blue hover:text-moto-blue-hover inline-flex transition-colors"
      >
        <Info className="h-[18px] w-[18px]" />
      </button>
      {open && (
        <span className="absolute top-6 right-0 z-20 w-56 rounded-lg border border-gray-200 bg-white p-3 text-left shadow-lg">
          <span className="block text-xs font-semibold text-gray-900">
            Zuletzt geändert
          </span>
          <span className="mt-1 block text-sm text-gray-700">
            {latest.edited_by}
          </span>
          <span className="block text-xs text-gray-500">am {when}</span>
        </span>
      )}
    </span>
  );
}

function ChevronRightIcon({
  className = "h-4 w-4",
}: Readonly<{ className?: string }>) {
  return (
    <svg
      className={className}
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
  );
}

export function ChevronDownIcon({
  className = "h-4 w-4",
}: Readonly<{ className?: string }>) {
  return (
    <svg
      className={className}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M19 9l-7 7-7-7"
      />
    </svg>
  );
}

export function WarningIcon({
  className = "h-5 w-5",
}: Readonly<{ className?: string }>) {
  return (
    <svg
      className={className}
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

// =============================================================================
// VIEW ONLY BADGE
// =============================================================================

function ViewOnlyBadge() {
  return (
    <span className="inline-flex flex-shrink-0 items-center gap-1 rounded-md bg-gray-100 px-2 py-1 text-xs font-medium text-gray-600 sm:px-2.5">
      <ViewOnlyIcon />
      <span className="hidden sm:inline">Nur Ansicht</span>
      <span className="sm:hidden">Ansicht</span>
    </span>
  );
}

// =============================================================================
// STUDENT HEADER
// =============================================================================

interface StudentHeaderProps {
  student: ExtendedStudent;
  myGroups: string[];
  myGroupRooms: string[];
  mySupervisedRooms: string[];
  todayPickupPlannedTime?: string;
  todayPickupActualTime?: string;
  todayPickupNote?: string;
  isPickupException?: boolean;
  todayArrivalPlannedTime?: string;
  todayArrivalActualTime?: string;
  isArrivalException?: boolean;
  isArrivalAbsent?: boolean;
  todayArrivalNote?: string;
  /** Optional reason for today's sick status, shown next to the badge. */
  sickReason?: string;
}

/**
 * Foto des Kindes links vom Namen (das `leading` der Kopfkarte).
 *
 * Das Foto ist eine Einstellung je Schule. Ist sie aus, entfällt der ganze
 * Platz inklusive der Initialen, damit diese Schulen denselben Kopf sehen wie
 * vor der Funktion.
 */
export function StudentHeaderAvatar({
  student,
}: Readonly<{ student: ExtendedStudent }>) {
  const { enabled: photosEnabled } = useStudentPhotosEnabled();
  if (!photosEnabled) return null;

  const fullName =
    `${student.first_name ?? ""} ${student.second_name ?? ""}`.trim() ||
    student.name ||
    "Kind";

  // xl entspricht dem optischen Gewicht der Seitenüberschrift.
  return (
    <Avatar imageUrl={student.photo_url ?? null} name={fullName} size="xl" />
  );
}

/** Der Name des Kindes als Titel der Kopfkarte. */
export function studentHeaderTitle(student: ExtendedStudent): string {
  return (
    `${student.first_name ?? ""} ${student.second_name ?? ""}`.trim() ||
    student.name ||
    "Kind"
  );
}

/** Aktueller Aufenthaltsort, rechts in der Titelzeile der Kopfkarte. */
export function StudentHeaderLocation({
  student,
  myGroups,
  myGroupRooms,
  mySupervisedRooms,
  todayArrivalPlannedTime,
  isArrivalException,
  isArrivalAbsent,
  todayArrivalNote,
}: Readonly<
  Pick<
    StudentHeaderProps,
    | "student"
    | "myGroups"
    | "myGroupRooms"
    | "mySupervisedRooms"
    | "todayArrivalPlannedTime"
    | "isArrivalException"
    | "isArrivalAbsent"
    | "todayArrivalNote"
  >
>) {
  // Spread the student so any field LocationBadge consumes (current_room_color,
  // future room-derived attributes, etc.) propagates without maintaining a
  // parallel whitelist here. Only the not_arrival_* fields are computed locally.
  const badgePlanning = getStudentPresenceBadgePlanning({
    ...student,
    arrival_is_exception: isArrivalException,
    arrival_time: todayArrivalPlannedTime,
    arrival_notes: todayArrivalNote,
  });
  const badgeStudent = {
    ...student,
    not_arrival_today:
      badgePlanning.notArrivalToday || (isArrivalAbsent ?? false),
    not_arrival_reason:
      badgePlanning.notArrivalReason ?? todayArrivalNote ?? null,
  };

  return (
    <LocationBadge
      student={badgeStudent}
      displayMode="contextAware"
      userGroups={myGroups}
      groupRooms={myGroupRooms}
      supervisedRooms={mySupervisedRooms}
      variant="modern"
      size="md"
      showLocationSince={true}
    />
  );
}

/**
 * Statuszeile der Kopfkarte: Klasse und Gruppe des Kindes und die Zeiten des
 * heutigen Tages. Alles steht schon im geladenen Datensatz.
 *
 * Bewusst nur `span`-Elemente: die Zeile steht im Beschreibungsabsatz der
 * Kopfkarte.
 */
export function StudentHeaderStats({
  student,
  todayPickupPlannedTime,
  todayPickupActualTime,
  todayPickupNote,
  isPickupException,
  todayArrivalPlannedTime,
  todayArrivalActualTime,
  isArrivalException,
  isArrivalAbsent,
  todayArrivalNote,
  sickReason,
}: Readonly<
  Omit<StudentHeaderProps, "myGroups" | "myGroupRooms" | "mySupervisedRooms">
>) {
  // Variant B: a sick/excused child without a completed pickup should not
  // accrue overdue pickup urgency, even if they already checked in before the
  // absence flag was set. Once pickup is recorded, the actual resolved times
  // can render normally.
  const absence = getStudentAbsence({
    sick: student.sick,
    classTrip: student.class_trip,
    excused: student.excused,
  });
  const dayPlanningNotComingLabel = getDayPlanningNotComingLabel(student);
  const notComingLabel = absence?.label ?? dayPlanningNotComingLabel;

  return (
    <span className="block">
      {student.group_name || student.school_class ? (
        <span className="inline-flex flex-wrap items-center gap-2">
          {student.school_class ? (
            <span className="truncate">Klasse {student.school_class}</span>
          ) : null}
          {student.school_class && student.group_name ? (
            <span aria-hidden="true">·</span>
          ) : null}
          {student.group_name ? (
            <span className="inline-flex items-center gap-2">
              <MotoDuotoneIcon
                icon={MOTO_CONCEPTS.groups.icon}
                tone={MOTO_CONCEPTS.groups.tone}
                size={18}
              />
              <span className="truncate">{student.group_name}</span>
            </span>
          ) : null}
        </span>
      ) : (
        <span>Keine Klasse und keine Gruppe hinterlegt</span>
      )}

      {notComingLabel && !todayPickupActualTime ? (
        <span
          data-testid="today-absence-row"
          className="mt-1.5 flex items-center gap-2 text-sm text-gray-600"
        >
          <ClockIcon className="h-4 w-4 text-gray-400" />
          <span className="font-medium text-gray-900">
            {`Kommt heute nicht (${notComingLabel}${
              student.sick && sickReason ? `: ${sickReason}` : ""
            })`}
          </span>
        </span>
      ) : (
        <>
          <TodayTimeStatusInlineRow
            kind="arrival"
            label="Heutige Ankunft"
            plannedTime={todayArrivalPlannedTime}
            actualTime={todayArrivalActualTime}
            isException={isArrivalException}
            note={isArrivalAbsent ? undefined : todayArrivalNote}
            isAbsent={isArrivalAbsent}
            absentReason={todayArrivalNote}
          />
          <TodayTimeStatusInlineRow
            kind="pickup"
            label="Heutige Abholung"
            plannedTime={todayPickupPlannedTime}
            actualTime={todayPickupActualTime}
            isException={isPickupException}
            note={todayPickupNote}
          />
        </>
      )}
    </span>
  );
}

/**
 * Der Entitätskopf der Kindakte als eigenständige Kopfkarte.
 *
 * Die Kindakte selbst setzt diese Teile in `TenantPage` zusammen; diese
 * Komponente bleibt für Ansichten, die nur den Kopf brauchen.
 */
export function StudentDetailHeader({
  student,
  myGroups,
  myGroupRooms,
  mySupervisedRooms,
  todayPickupPlannedTime,
  todayPickupActualTime,
  todayPickupNote,
  isPickupException,
  todayArrivalPlannedTime,
  todayArrivalActualTime,
  isArrivalException,
  isArrivalAbsent,
  todayArrivalNote,
  sickReason,
}: Readonly<StudentHeaderProps>) {
  return (
    <PageIntro
      className="mb-6"
      leading={<StudentHeaderAvatar student={student} />}
      title={studentHeaderTitle(student)}
      description={
        <StudentHeaderStats
          student={student}
          todayPickupPlannedTime={todayPickupPlannedTime}
          todayPickupActualTime={todayPickupActualTime}
          todayPickupNote={todayPickupNote}
          isPickupException={isPickupException}
          todayArrivalPlannedTime={todayArrivalPlannedTime}
          todayArrivalActualTime={todayArrivalActualTime}
          isArrivalException={isArrivalException}
          isArrivalAbsent={isArrivalAbsent}
          todayArrivalNote={todayArrivalNote}
          sickReason={sickReason}
        />
      }
      actions={
        <StudentHeaderLocation
          student={student}
          myGroups={myGroups}
          myGroupRooms={myGroupRooms}
          mySupervisedRooms={mySupervisedRooms}
          todayArrivalPlannedTime={todayArrivalPlannedTime}
          isArrivalException={isArrivalException}
          isArrivalAbsent={isArrivalAbsent}
          todayArrivalNote={todayArrivalNote}
        />
      }
    />
  );
}

function renderInlineTimeIcon(
  state: ReturnType<typeof getStudentTimeStatus>,
): React.ReactNode {
  if (state.icon === "warning") {
    return (
      <AlertTriangle className="h-4 w-4" style={{ color: state.iconColor }} />
    );
  }

  if (state.icon === "check") {
    return <Check className="h-4 w-4" style={{ color: state.iconColor }} />;
  }

  return (
    <Clock
      className={`h-4 w-4 ${state.state === "approaching" ? "animate-pulse" : ""}`}
      style={{ color: state.iconColor }}
    />
  );
}

function TodayTimeStatusInlineRow({
  kind,
  label,
  plannedTime,
  actualTime,
  isException = false,
  note,
  isAbsent = false,
  absentReason,
}: Readonly<{
  kind: "arrival" | "pickup";
  label: string;
  plannedTime?: string;
  actualTime?: string;
  isException?: boolean;
  note?: string;
  isAbsent?: boolean;
  absentReason?: string;
}>) {
  const now = useMinuteClock();

  if (isAbsent) {
    return (
      <span
        data-testid={`today-time-row-${kind}`}
        className="mt-1.5 flex items-center gap-2 text-sm text-gray-600"
      >
        <ClockIcon className="h-4 w-4 text-gray-400" />
        <span>
          {label}:{" "}
          <span className="font-medium text-gray-900">Kommt heute nicht</span>
          {absentReason && (
            <span className="ml-1 text-gray-500">({absentReason})</span>
          )}
        </span>
      </span>
    );
  }

  const status = getStudentTimeStatus({
    plannedTime,
    actualTime,
    now,
  });

  const plannedDisplay = plannedTime?.slice(0, 5);
  const showPlannedHint = Boolean(plannedTime && actualTime);

  return (
    <span
      data-testid={`today-time-row-${kind}`}
      className="mt-1.5 flex items-center gap-2 text-sm text-gray-600"
    >
      {renderInlineTimeIcon(status)}
      <span>
        {label}:{" "}
        <span
          className={
            status.textColor ? "font-medium" : "font-medium text-gray-900"
          }
          style={status.textColor ? { color: status.textColor } : undefined}
        >
          {status.displayTime ?? "—"}
        </span>
        {showPlannedHint && plannedDisplay && (
          <span className="ml-1 text-gray-500">
            (geplant: {plannedDisplay}
            {status.detailAnnotation ? `, ${status.detailAnnotation}` : ""})
          </span>
        )}
        {!showPlannedHint && status.detailAnnotation && (
          <span className="ml-1 text-gray-500">
            ({status.detailAnnotation})
          </span>
        )}
        {note && <span className="ml-1 text-gray-500">({note})</span>}
        {isException && (
          <span
            className="bg-moto-orange ml-1.5 inline-flex h-2 w-2 rounded-full"
            title="Ausnahme"
          />
        )}
      </span>
    </span>
  );
}

// =============================================================================
// SUPERVISORS CARD
// =============================================================================

interface SupervisorsCardProps {
  supervisors: SupervisorContact[];
  studentName: string;
}

export function SupervisorsCard({
  supervisors,
  studentName,
}: Readonly<SupervisorsCardProps>) {
  if (supervisors.length === 0) return null;

  return (
    <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
      <div className="mb-4 flex items-center justify-between gap-2">
        <div className="flex min-w-0 flex-1 items-center gap-2 sm:gap-3">
          <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-xl bg-gray-100 sm:h-10 sm:w-10">
            <MotoDuotoneIcon
              icon={MOTO_CONCEPTS.staff.icon}
              tone={MOTO_CONCEPTS.staff.tone}
              size={22}
            />
          </div>
          <h2 className="truncate text-base font-semibold text-gray-900 sm:text-lg">
            Ansprechpartner
          </h2>
        </div>
        <ViewOnlyBadge />
      </div>

      <div className="space-y-4">
        {supervisors.map((supervisor, index) => (
          <SupervisorItem
            key={supervisor.id}
            supervisor={supervisor}
            studentName={studentName}
            showDivider={index > 0}
          />
        ))}
      </div>
    </div>
  );
}

interface SupervisorItemProps {
  supervisor: SupervisorContact;
  studentName: string;
  showDivider: boolean;
}

function SupervisorItem({
  supervisor,
  studentName,
  showDivider,
}: Readonly<SupervisorItemProps>) {
  const handleEmailClick = () => {
    globalThis.location.href = `mailto:${supervisor.email}?subject=Anfrage zu ${studentName}`;
  };

  return (
    <div>
      {showDivider && <div className="my-4 border-t border-gray-100" />}
      <div>
        <div className="flex flex-wrap items-center gap-2">
          <p className="text-base font-semibold text-gray-900">
            {supervisor.first_name} {supervisor.last_name}
          </p>
          <span className="inline-flex items-center rounded-full bg-gray-100 px-2.5 py-0.5 text-xs font-medium text-gray-700">
            Gruppenleitung
          </span>
        </div>
        {supervisor.email && (
          <>
            <p className="mt-1 text-sm text-gray-500">{supervisor.email}</p>
            <Button
              type="button"
              variant="primary"
              size="md"
              className="mt-3"
              onClick={handleEmailClick}
            >
              <EmailIcon />
              Kontakt aufnehmen
            </Button>
          </>
        )}
      </div>
    </div>
  );
}

// =============================================================================
// PERSONAL INFO READ ONLY
// =============================================================================

interface PersonalInfoReadOnlyProps {
  student: ExtendedStudent;
  enrollmentExtraGroups?: StudentEnrollmentExtraFieldGroup[];
  showEditButton?: boolean;
  onEditClick?: () => void;
}

const EMPTY_ENROLLMENT_EXTRA_GROUPS: StudentEnrollmentExtraFieldGroup[] = [];

export function PersonalInfoReadOnly({
  student,
  enrollmentExtraGroups = EMPTY_ENROLLMENT_EXTRA_GROUPS,
  showEditButton = false,
  onEditClick,
}: Readonly<PersonalInfoReadOnlyProps>) {
  // The Laufgemeinschaft lives in its own table, so it is fetched here rather
  // than riding along on the student payload. A failure must not break the rest
  // of the Stammdaten card, but it must not read as "walks alone" either: for a
  // child whose accompanied days are backed only by links there is no free-text
  // note, so silently dropping the row would leave "Mit anderem Kind" standing
  // without a name. Hence a distinct unavailable state next to the list.
  const [companions, setCompanions] = useState<StudentCompanion[]>([]);
  const [companionsUnavailable, setCompanionsUnavailable] = useState(false);
  // Saving the Stammdaten does not remount this card and does not bring the
  // links along in the student payload, so the fetch below must be re-run on
  // every companion write — including a symmetric one made from the linked
  // child's card. The counter is the effect's second trigger.
  const [companionsRevalidation, setCompanionsRevalidation] = useState(0);
  useEffect(() => {
    const revalidate = () => setCompanionsRevalidation((count) => count + 1);
    // Two buses, because two different writes make this card stale. The links
    // themselves change on a companion write; what the links DISPLAY changes on
    // a plain student write — a linked child renamed or moved to another group
    // emits only student_updated, and this effect's student id stays the same,
    // so the old name would otherwise survive here forever. After a group
    // reassignment that name can be one a fresh request would redact for the
    // viewing supervisor, which makes the refetch a visibility fix, not a
    // cosmetic one.
    const unsubscribeCompanions = subscribeStudentCompanionsChanged(revalidate);
    const unsubscribeDisplay =
      subscribeStudentCompanionDisplayChanged(revalidate);
    return () => {
      unsubscribeCompanions();
      unsubscribeDisplay();
    };
  }, []);
  // This card is reused across children without unmounting, and the links are
  // fetched separately from the student payload. Dropping the previous child's
  // companions only once the new request resolves would show one child's
  // departure companions under another child's name for the whole request —
  // safety-relevant information about who walks home with whom. Reset during
  // render (React's "adjusting state on prop change" pattern) so the wrong
  // names never reach the screen, not in the effect, which runs after paint.
  const [companionsStudentId, setCompanionsStudentId] = useState(student.id);
  if (companionsStudentId !== student.id) {
    setCompanionsStudentId(student.id);
    setCompanions([]);
    setCompanionsUnavailable(false);
  }
  useEffect(() => {
    if (!student.id) return;
    let cancelled = false;
    fetchStudentCompanions(student.id)
      .then((loaded) => {
        if (cancelled) return;
        setCompanions(loaded);
        setCompanionsUnavailable(false);
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        setCompanions([]);
        setCompanionsUnavailable(true);
        logger.error("student_companions_load_failed", {
          student_id: student.id,
          error: error instanceof Error ? error.message : String(error),
        });
      });
    return () => {
      cancelled = true;
    };
  }, [student.id, companionsRevalidation]);

  const birthdayDisplay = student.birthday
    ? new Date(student.birthday).toLocaleDateString("de-DE", {
        timeZone: "Europe/Berlin",
        day: "2-digit",
        month: "2-digit",
        year: "numeric",
      })
    : "Nicht angegeben";
  const addressDisplay = formatStudentAddress(student);

  return (
    <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
      <div className="mb-4 flex items-center justify-between gap-2">
        <div className="flex min-w-0 flex-1 items-center gap-2 sm:gap-3">
          <div className="bg-moto-green/10 text-moto-green flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg sm:h-10 sm:w-10">
            <PersonIcon />
          </div>
          <h2 className="truncate text-base font-semibold text-gray-900 sm:text-lg">
            <span className="sm:hidden">Persönliche Infos</span>
            <span className="hidden sm:inline">Persönliche Informationen</span>
          </h2>
        </div>
        {showEditButton && onEditClick ? (
          <Button
            type="button"
            variant="ghost"
            size="compact"
            onClick={onEditClick}
            title="Bearbeiten"
          >
            Bearbeiten
          </Button>
        ) : (
          <ViewOnlyBadge />
        )}
      </div>
      <ParentVisibilityLegend className="mb-4" />
      <div className="space-y-3">
        <InfoItem
          label={
            <ParentVisibleLabel
              label="Vollständiger Name"
              hint={PARENT_VISIBLE_HINTS.name}
            />
          }
          value={student.name}
        />
        <InfoItem
          label={
            <ParentVisibleLabel
              label="Klasse"
              hint={PARENT_VISIBLE_HINTS.schoolClass}
            />
          }
          value={student.school_class}
        />
        <InfoItem
          label="Gruppe"
          value={student.group_name ?? "Nicht zugewiesen"}
        />
        <InfoItem
          label={
            <ParentVisibleLabel
              label="Geburtsdatum"
              hint={PARENT_VISIBLE_HINTS.birthday}
            />
          }
          value={birthdayDisplay}
        />
        {addressDisplay && <InfoItem label="Adresse" value={addressDisplay} />}
        <InfoItem
          label={
            <ParentVisibleLabel
              label="Erlaubte Heimwege"
              hint={PARENT_VISIBLE_HINTS.departure}
            />
          }
          value={
            <span className="flex items-start gap-1.5">
              <span className="min-w-0 flex-1">
                <AllowedDepartureModesDisplay
                  value={
                    student.allowed_departure_modes ??
                    allowedDepartureModesFromDeparture(
                      student.departure_days ??
                        departureDaysFromLegacy(
                          student.bus_days,
                          student.pickup_days,
                        ),
                    )
                  }
                />
              </span>
              <FieldHistoryInfo
                studentId={student.id}
                fields={["departure_days", "pickup_status"]}
              />
            </span>
          }
        />
        {companionsUnavailable && (
          <InfoItem
            label="Geht mit"
            value={
              <span className="text-sm text-gray-500">
                Laufgemeinschaft konnte nicht geladen werden
              </span>
            }
          />
        )}
        {!companionsUnavailable && companions.length > 0 && (
          <InfoItem
            label="Geht mit"
            value={
              <span className="space-y-0.5">
                {companions.map((companion) => (
                  <span
                    key={companion.companion_student_id}
                    className="block text-sm"
                  >
                    {companionDisplayName(companion)}
                    <span className="text-gray-500">
                      {" "}
                      ({formatCompanionWeekdays(companion.weekdays)})
                    </span>
                  </span>
                ))}
              </span>
            }
          />
        )}
        {student.departure_companion_note && (
          <InfoItem
            label="Geht außerdem mit"
            value={
              <span className="flex items-start gap-1.5">
                <span className="min-w-0 flex-1">
                  {student.departure_companion_note}
                </span>
                <FieldHistoryInfo
                  studentId={student.id}
                  fields={["departure_companion_note"]}
                />
              </span>
            }
          />
        )}
        {student.health_info && (
          <InfoItem
            label={
              <ParentVisibleLabel
                label="Gesundheitsinformationen"
                hint={PARENT_VISIBLE_HINTS.healthInfo}
              />
            }
            value={
              <span className="flex items-start gap-1.5">
                <span className="min-w-0 flex-1">{student.health_info}</span>
                <FieldHistoryInfo
                  studentId={student.id}
                  fields={["health_info"]}
                />
              </span>
            }
          />
        )}
        {student.supervisor_notes && (
          <InfoItem
            label="Betreuernotizen"
            value={
              <span className="flex items-start gap-1.5">
                <span className="min-w-0 flex-1">
                  {student.supervisor_notes}
                </span>
                <FieldHistoryInfo
                  studentId={student.id}
                  fields={["supervisor_notes"]}
                />
              </span>
            }
          />
        )}
        {student.extra_info && (
          <InfoItem
            label="Elternnotizen"
            value={
              <span className="flex items-start gap-1.5">
                <span className="min-w-0 flex-1">{student.extra_info}</span>
                <FieldHistoryInfo
                  studentId={student.id}
                  fields={["extra_info"]}
                />
              </span>
            }
          />
        )}
        <EnrollmentExtraInfoItems groups={enrollmentExtraGroups} />
      </div>
    </div>
  );
}

/** An InfoItem label carrying the "sichtbar für Eltern" marker (#2163). */
function ParentVisibleLabel({
  label,
  hint,
}: Readonly<{ label: string; hint: string }>) {
  return (
    <span className="inline-flex items-center gap-1">
      {label}
      <ParentVisibleBadge compact hint={hint} />
    </span>
  );
}

function formatStudentAddress(student: ExtendedStudent): string | null {
  const street = student.address_street?.trim();
  const postalCode = student.address_postal_code?.trim();
  const city = student.address_city?.trim();
  const locality = [postalCode, city].filter(Boolean).join(" ");
  const lines = [street, locality].filter(Boolean);
  return lines.length > 0 ? lines.join(", ") : null;
}

function EnrollmentExtraInfoItems({
  groups,
}: Readonly<{ groups: StudentEnrollmentExtraFieldGroup[] }>) {
  if (groups.length === 0) return null;
  const prefixWithPhase = groups.length > 1;

  return (
    <>
      {groups.flatMap((group) =>
        group.fields.map((field) => {
          const value = formatCustomValue(field.value, field);
          if (value === null) return null;
          const label =
            prefixWithPhase && group.phase_name
              ? `${group.phase_name} · ${field.label}`
              : field.label;
          return (
            <InfoItem
              key={`${group.request_id}-${field.key}`}
              label={label}
              value={value}
            />
          );
        }),
      )}
    </>
  );
}

// =============================================================================
// HISTORY SECTION
// =============================================================================

interface HistoryButtonProps {
  concept: MotoConceptKey;
  title: string;
  description: string;
  disabled?: boolean;
  onClick?: () => void;
}

function HistoryButton({
  concept,
  title,
  description,
  disabled = false,
  onClick,
}: Readonly<HistoryButtonProps>) {
  const baseClasses = "flex items-center justify-between rounded-lg border p-3";
  const stateClasses = disabled
    ? "cursor-not-allowed border-gray-100 bg-gray-50 opacity-60"
    : "cursor-pointer border-gray-200 bg-white transition-colors hover:bg-gray-50";

  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onClick}
      className={`${baseClasses} ${stateClasses}`}
    >
      <div className="flex items-center gap-3">
        <ConceptIconTile concept={concept} variant="section" />
        <div className="min-w-0 flex-1 text-left">
          <p
            className={`text-sm font-medium sm:text-base ${
              disabled ? "text-gray-400" : "text-gray-900"
            }`}
          >
            {title}
          </p>
          <p className="text-xs text-gray-400">{description}</p>
        </div>
      </div>
      <ChevronRightIcon className="h-4 w-4 flex-shrink-0 text-gray-300" />
    </button>
  );
}

interface StudentHistorySectionProps {
  studentId: string;
  attendanceLogEnabled: boolean;
  feedbackEnabled: boolean;
  readOnly?: boolean;
  canViewChangeHistory?: boolean;
  onNavigate: (path: string) => void;
}

/**
 * History section on the student detail page. The Anwesenheitsprotokoll button
 * is gated by the tenant's `gdpr.attendance_log_enabled` setting. The
 * Feedbackhistorie button is gated by the tenant's `feedback.enabled` setting.
 * Mensa history remains a disabled placeholder (future feature).
 */
export function StudentHistorySection({
  studentId,
  attendanceLogEnabled,
  feedbackEnabled,
  readOnly = false,
  canViewChangeHistory = true,
  onNavigate,
}: Readonly<StudentHistorySectionProps>) {
  const attendanceDisabled = readOnly || !attendanceLogEnabled;
  const feedbackDisabled = readOnly || !feedbackEnabled;
  const changeHistoryDisabled = readOnly || !canViewChangeHistory;

  return (
    <InfoCard
      title="Historien"
      icon={
        <MotoDuotoneIcon
          icon={MOTO_CONCEPTS.changeHistory.icon}
          tone={MOTO_CONCEPTS.changeHistory.tone}
          size={20}
        />
      }
    >
      <div className="grid grid-cols-1 gap-2">
        <HistoryButton
          concept="rooms"
          title="Anwesenheitsprotokoll"
          description={
            readOnly
              ? "Nur für Gruppenbetreuer"
              : attendanceLogEnabled
                ? "Anwesenheit je Betreuungsangebot und besuchte Räume"
                : "Für Ihre Schule deaktiviert"
          }
          disabled={attendanceDisabled}
          onClick={
            !attendanceDisabled
              ? () => onNavigate(`/students/${studentId}/room-history`)
              : undefined
          }
        />
        <HistoryButton
          concept="feedback"
          title="Feedbackhistorie"
          description={
            readOnly
              ? "Nur für Gruppenbetreuer"
              : feedbackEnabled
                ? "Feedback und Bewertungen"
                : "Für Ihre Schule deaktiviert"
          }
          disabled={feedbackDisabled}
          onClick={
            !feedbackDisabled
              ? () => onNavigate(`/students/${studentId}/feedback-history`)
              : undefined
          }
        />
        <HistoryButton
          concept="mealPlan"
          title="Mensaverlauf"
          description="Mahlzeiten und Bestellungen"
          disabled
        />
        <HistoryButton
          concept="changeHistory"
          title="Änderungsverlauf"
          description={
            changeHistoryDisabled
              ? "Nur für Gruppenbetreuer"
              : "Wer hat was geändert"
          }
          disabled={changeHistoryDisabled}
          onClick={
            !changeHistoryDisabled
              ? () => onNavigate(`/students/${studentId}/change-history`)
              : undefined
          }
        />
      </div>
    </InfoCard>
  );
}
