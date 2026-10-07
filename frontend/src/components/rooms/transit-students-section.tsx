"use client";

import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { Check, ChevronDown, ExternalLink } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import type { Session } from "next-auth";
import {
  useSWRAuth,
  useTenantMutate,
  useTenantMutateMatching,
} from "~/lib/swr";
import { Button } from "~/components/ui/button";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { DatabaseSelect } from "~/components/ui/database/database-select";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import {
  activeService,
  summarizeStudentMoveResult,
} from "~/lib/active-service";
import type { ActiveGroup, Supervisor } from "~/lib/active-helpers";
import { roomService, studentService } from "~/lib/api";
import type { Student } from "~/lib/api";
import { ApiError, wireErrorCode } from "~/lib/api-error";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import type { Room } from "~/lib/room-helpers";
import { userContextService } from "~/lib/usercontext-api";
import type { Staff } from "~/lib/usercontext-helpers";
import { CompactStudentCard } from "~/components/students/compact-student-card";
import { useTenantRouter } from "~/lib/tenant-router";
import {
  useAttendanceWebEnabled,
  useSchoolWideAttendanceMoves,
} from "~/lib/tenant-context";

const DETAIL_CARD_CLASS =
  "moto-content-surface rounded-2xl border p-5 shadow-sm sm:p-6";
const EMPTY_STUDENTS: Student[] = [];
const EMPTY_ROOMS: Room[] = [];

/**
 * One entry of the target select. A `session` target assigns the children to
 * a running session; an `openRoom` target books an independent stay in a
 * released room (#3066), for example after the child's activity ended.
 */
type TransitTargetOption =
  | {
      readonly kind: "session";
      readonly value: string;
      readonly label: string;
    }
  | {
      readonly kind: "openRoom";
      readonly value: string;
      readonly roomId: string;
      readonly roomName: string;
      readonly label: string;
    };

/**
 * Erfolgsmeldung als ganzer Satz (#2517): „2 Kinder sind jetzt in Mensa.“,
 * übersprungene Kinder als zweiter Satz.
 */
function movedMessage(count: number, skipped: number, where: string): string {
  const moved =
    count === 1 ? `1 Kind ist ${where}.` : `${count} Kinder sind ${where}.`;
  if (skipped === 0) return moved;
  return `${moved} ${
    skipped === 1
      ? "1 Kind wurde übersprungen."
      : `${skipped} Kinder wurden übersprungen.`
  }`;
}

function buildSessionLabel(group: ActiveGroup): string {
  const roomName = group.room?.name ?? `Raum ${group.roomId}`;
  const activityName = group.actualGroup?.name;
  return activityName ? `${roomName} · ${activityName}` : roomName;
}

function canUseAllMoveTargets(session: Session | null): boolean {
  const permissions = session?.user?.permissions ?? [];
  return (
    session?.user?.isAdmin === true ||
    session?.user?.roles?.includes("admin") === true ||
    permissions.includes("admin:*") ||
    permissions.includes("*:*")
  );
}

interface TransitStudentsSectionProps {
  readonly onSelectionActiveChange?: (active: boolean) => void;
  /** Meldet die geladene Zahl für die Statuszeile der eigenständigen Seite. */
  readonly onTotalCountChange?: (totalCount: number | null) => void;
  readonly fromReferrer?: string;
  readonly collapsible?: boolean;
}

export function TransitStudentsSection({
  onSelectionActiveChange,
  onTotalCountChange,
  fromReferrer: fromReferrerOverride,
  collapsible = false,
}: TransitStudentsSectionProps) {
  const router = useTenantRouter();
  const { data: session } = useSession();
  const attendanceWebEnabled = useAttendanceWebEnabled();
  // Visibility never grants move rights: only administrators may target any
  // running module; staff remain limited to modules they supervise.
  const showAllTargets = canUseAllMoveTargets(session);
  // A child without a room belongs to no supervision, so booking it into a
  // released room needs the school-wide move right (#3066): administrators,
  // or every staff member where the school opened attendance edits and the
  // overview to all staff. The server re-checks this for every move.
  const schoolWideMoves = useSchoolWideAttendanceMoves();
  const canTargetOpenRooms = showAllTargets || schoolWideMoves;
  const sectionSearchParams = useSearchParams();
  const [selectedIds, setSelectedIds] = useState<Set<string>>(() => new Set());
  const [collapsibleExpanded, setCollapsibleExpanded] = useState(false);
  const isExpanded = !collapsible || collapsibleExpanded;
  const [targetValue, setTargetValue] = useState("");
  const [submitting, setSubmitting] = useState(false);
  // „In Raum setzen“: Fehler stehen in der Leiste, der Zielraum wird
  // markiert (#2517).
  const assignBarRef = useRef<HTMLDivElement>(null);
  const assignErrors = useApiFormError(assignBarRef);
  const { clear: clearAssignErrors } = assignErrors;
  // „Wiederholen“ weist die aktuell gewählten Kinder dem aktuellen Ziel zu.
  const retryAssignRef = useRef<() => void>(() => undefined);
  const { success: toastSuccess } = useToast();
  const mutateKey = useTenantMutate();
  // "dashboard" (matching the aggregated active-supervision-dashboard key,
  // which carries the room's visits since #2096) and "timetable-roster-"
  // keep the /active-supervisions room grid and roster in sync when this
  // section is embedded there — without them the assigned children only
  // appear once an SSE event happens to arrive.
  const mutateMatching = useTenantMutateMatching([
    "room-students-",
    "room-detail-",
    "rooms-list",
    "dashboard",
    "timetable-roster-",
  ]);

  const {
    data: studentsData,
    error: studentsError,
    isLoading: studentsLoading,
    mutate: mutateStudents,
  } = useSWRAuth<{
    students: Student[];
    pagination?: { total_records: number };
  }>("transit-students", () =>
    studentService.getStudents({ locationState: "transit", pageSize: 200 }),
  );

  const {
    data: activeGroups = [],
    error: groupsError,
    isLoading: groupsLoading,
    mutate: reloadGroups,
  } = useSWRAuth<ActiveGroup[]>("active-groups-for-transit", () =>
    activeService.getActiveGroups({ active: true }),
  );
  const {
    data: currentStaff,
    error: staffError,
    isLoading: staffLoading,
    mutate: reloadStaff,
  } = useSWRAuth<Staff>(
    showAllTargets ? null : "transit-target-current-staff",
    () => userContextService.getCurrentStaff(),
  );
  const {
    data: activeSupervisions = [],
    error: supervisionsError,
    isLoading: supervisionsLoading,
    mutate: reloadSupervisions,
  } = useSWRAuth<Supervisor[]>(
    showAllTargets || !currentStaff?.id
      ? null
      : `transit-target-supervisions-${currentStaff.id}`,
    () => activeService.getStaffActiveSupervisions(currentStaff?.id ?? ""),
  );

  const {
    data: rooms = EMPTY_ROOMS,
    error: roomsError,
    isLoading: roomsLoading,
    mutate: refreshTargetRooms,
  } = useSWRAuth<Room[]>(
    canTargetOpenRooms ? "transit-target-rooms" : null,
    () => roomService.getRooms(),
  );
  // Ladefehler einer der fünf Listen: eine Meldung, „Wiederholen“ lädt alle.
  const loadError = useSwrLoadError(
    studentsError ??
      groupsError ??
      staffError ??
      supervisionsError ??
      roomsError,
    "die Liste der Kinder unterwegs",
    () =>
      Promise.all([
        mutateStudents(),
        reloadGroups(),
        reloadStaff(),
        reloadSupervisions(),
        refreshTargetRooms(),
      ]),
  );

  const supervisedTargetGroupIds = useMemo(
    () =>
      new Set(
        activeSupervisions
          .filter((supervision) => supervision.isActive)
          .map((supervision) => supervision.activeGroupId),
      ),
    [activeSupervisions],
  );

  const targetOptions = useMemo((): TransitTargetOption[] => {
    const sessionOptions = activeGroups
      .filter(
        (group) =>
          group.isActive &&
          (showAllTargets || supervisedTargetGroupIds.has(group.id)),
      )
      .map((group): TransitTargetOption => ({
        kind: "session",
        value: group.id,
        label: buildSessionLabel(group),
      }));
    const openRoomOptions = canTargetOpenRooms
      ? rooms
          .filter((room) => room.isOpenRoom)
          .map((room): TransitTargetOption => ({
            kind: "openRoom",
            value: `room:${room.id}`,
            roomId: room.id,
            roomName: room.name,
            label: `${room.name} (offener Raum)`,
          }))
      : [];
    return [...sessionOptions, ...openRoomOptions].sort((a, b) =>
      a.label.localeCompare(b.label, "de"),
    );
  }, [
    activeGroups,
    canTargetOpenRooms,
    rooms,
    showAllTargets,
    supervisedTargetGroupIds,
  ]);
  const targetsLoading =
    groupsLoading || staffLoading || supervisionsLoading || roomsLoading;

  const students = studentsData?.students ?? EMPTY_STUDENTS;
  const visibleStudentIds = useMemo(
    () => new Set(students.map((student) => String(student.id))),
    [students],
  );
  const selectedVisibleCount = [...selectedIds].filter((studentId) =>
    visibleStudentIds.has(studentId),
  ).length;
  const totalCount = studentsData?.pagination?.total_records ?? students.length;
  const allSelected =
    students.length > 0 && selectedVisibleCount === students.length;
  // Die Kindakte kehrt auf die Seite „Unterwegs" zurück (#3115); ihre
  // Abfrage (etwa der Rückweg zur Übersicht) reist mit.
  const defaultFromReferrer = (() => {
    const qs = sectionSearchParams?.toString() ?? "";
    return qs ? `/rooms/unterwegs?${qs}` : "/rooms/unterwegs";
  })();
  const fromReferrer = fromReferrerOverride ?? defaultFromReferrer;

  useEffect(() => {
    onSelectionActiveChange?.(selectedVisibleCount > 0);
  }, [onSelectionActiveChange, selectedVisibleCount]);

  useEffect(() => {
    onTotalCountChange?.(studentsData ? totalCount : null);
  }, [onTotalCountChange, studentsData, totalCount]);

  useEffect(() => {
    if (
      targetValue &&
      !targetOptions.some((option) => option.value === targetValue)
    ) {
      setTargetValue("");
    }
  }, [targetValue, targetOptions]);

  const toggleSelected = (studentId: string) => {
    setSelectedIds((current) => {
      const next = new Set(current);
      if (next.has(studentId)) next.delete(studentId);
      else next.add(studentId);
      return next;
    });
    clearAssignErrors();
  };

  const selectAllVisible = () => {
    setSelectedIds(new Set(students.map((student) => student.id.toString())));
    clearAssignErrors();
  };

  const clearSelection = () => {
    setSelectedIds(new Set());
    clearAssignErrors();
  };

  const assignSelected = async () => {
    const studentIds = [...selectedIds].filter((studentId) =>
      visibleStudentIds.has(studentId),
    );
    const target = targetOptions.find((option) => option.value === targetValue);
    if (!target || studentIds.length === 0) return;
    setSubmitting(true);
    clearAssignErrors();
    try {
      const message =
        target.kind === "openRoom"
          ? await bookIntoOpenRoom(studentIds, target)
          : await assignToSession(studentIds, target.value);
      setSelectedIds(new Set());
      setTargetValue("");
      await mutateStudents();
      await mutateKey("rooms-list");
      await mutateMatching();
      toastSuccess(message);
    } catch (err) {
      const releaseRemoved =
        target.kind === "openRoom" &&
        err instanceof ApiError &&
        wireErrorCode(err.code) === "rooms.not_released";
      if (releaseRemoved) {
        // The release was removed after the room list loaded: drop the stale
        // choice and reload the rooms, so the list stops offering it.
        setTargetValue("");
        await refreshTargetRooms();
      }
      // Katalogtext; ein voller Raum oder eine volle Aktivität nennt sich
      // selbst über Code und Details (#3633).
      void assignErrors.show(err, {
        object: "das Zuweisen der Kinder",
        retry: releaseRemoved ? undefined : () => retryAssignRef.current(),
      });
    } finally {
      setSubmitting(false);
    }
  };
  useLayoutEffect(() => {
    retryAssignRef.current = () => void assignSelected();
  });

  const assignToSession = async (
    studentIds: string[],
    activeGroupId: string,
  ): Promise<string> => {
    const result = await activeService.assignTransitStudents(
      studentIds,
      activeGroupId,
    );
    return movedMessage(
      result.assigned.length,
      result.skipped.length,
      "zugewiesen",
    );
  };

  // An explicit independent stay in a released room (#3066): the children
  // use the room without joining an activity that runs there.
  const bookIntoOpenRoom = async (
    studentIds: string[],
    target: Extract<TransitTargetOption, { kind: "openRoom" }>,
  ): Promise<string> => {
    const result = await activeService.moveStudentsToOpenRoom(
      studentIds,
      target.roomId,
    );
    const { successCount } = summarizeStudentMoveResult(result);
    return movedMessage(
      successCount,
      result.skipped.length,
      `jetzt in ${target.roomName}`,
    );
  };

  return (
    <section aria-label="Kinder unterwegs" className={DETAIL_CARD_CLASS}>
      {collapsible ? (
        <button
          type="button"
          onClick={() => setCollapsibleExpanded((current) => !current)}
          aria-expanded={isExpanded}
          className="group flex w-full min-w-0 items-center gap-3 text-left focus-visible:ring-2 focus-visible:ring-gray-300 focus-visible:outline-none"
        >
          <span
            className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-gray-100"
            aria-hidden="true"
          >
            <MotoConceptIcon concept="transit" size={18} />
          </span>
          <span className="min-w-0 flex-1">
            <span className="block text-xs font-semibold tracking-wider text-gray-500 uppercase">
              Kinder unterwegs
            </span>
            <span className="block truncate text-sm text-gray-600">
              <span className="font-medium text-gray-900">{totalCount}</span>{" "}
              {totalCount === 1 ? "Kind" : "Kinder"} ohne Raum
            </span>
          </span>
          <ChevronDown
            className={`h-4 w-4 shrink-0 text-gray-400 transition-transform group-hover:text-gray-600 ${
              isExpanded ? "rotate-180" : ""
            }`}
            aria-hidden="true"
          />
        </button>
      ) : (
        <>
          <h2 className="mb-3 text-xs font-semibold tracking-wider text-gray-500 uppercase">
            Kinder unterwegs
          </h2>
          <div className="flex flex-wrap items-end justify-between gap-3">
            <p className="text-sm text-gray-600">
              <span className="font-medium text-gray-900">{totalCount}</span>{" "}
              {totalCount === 1 ? "Kind" : "Kinder"} ohne Raum
            </p>
          </div>
        </>
      )}

      {isExpanded && loadError ? (
        <div className="mt-4">
          <LoadErrorAlert error={loadError} />
        </div>
      ) : null}

      {isExpanded ? (
        <>
          {attendanceWebEnabled ? (
            <div
              ref={assignBarRef}
              className={`mt-4 mb-4 rounded-xl border p-3 transition-shadow ${
                selectedVisibleCount > 0
                  ? "sticky bottom-3 z-20 border-gray-200 bg-white/95 shadow-sm backdrop-blur"
                  : "border-transparent bg-gray-50/80 shadow-none"
              }`}
            >
              <div className="flex items-start justify-between gap-3">
                <p className="min-w-0 text-sm font-semibold text-gray-900">
                  <span className="block truncate">
                    {selectedVisibleCount > 0
                      ? `${selectedVisibleCount} von ${students.length} ausgewählt`
                      : "Kinder auswählen"}
                  </span>
                  <span className="block text-xs font-medium text-gray-500">
                    Zielraum wählen und gemeinsam zuweisen
                  </span>
                </p>
                {students.length > 0 ? (
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={allSelected ? clearSelection : selectAllVisible}
                    className="h-8 shrink-0 rounded-full px-3 py-0 text-xs shadow-none"
                  >
                    {allSelected ? "Aufheben" : "Alle auswählen"}
                  </Button>
                ) : null}
              </div>

              <div className="mt-3 grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
                <DatabaseSelect
                  id="transit-target-room"
                  name="transit-target-room"
                  label="Zielraum"
                  value={targetValue}
                  onChange={(value) => {
                    setTargetValue(value);
                    clearAssignErrors();
                  }}
                  disabled={
                    targetsLoading || targetOptions.length === 0 || submitting
                  }
                  placeholder={
                    targetsLoading
                      ? "Aktive Räume werden geladen..."
                      : targetOptions.length === 0
                        ? "Keine aktiven Räume"
                        : "Zielraum wählen"
                  }
                  options={targetOptions.map((option) => ({
                    value: option.value,
                    label: option.label,
                  }))}
                  className="bg-white text-sm md:text-sm"
                />
                <Button
                  type="button"
                  variant="primary"
                  size="sm"
                  isLoading={submitting}
                  loadingText="Weise zu..."
                  onClick={() => void assignSelected()}
                  disabled={
                    !targetValue || selectedVisibleCount === 0 || submitting
                  }
                  className="h-9 w-full px-3 py-2 text-xs shadow-sm sm:w-auto"
                >
                  In Raum setzen
                </Button>
              </div>
              <FormErrorAlert message={assignErrors.error} className="mt-2" />
            </div>
          ) : null}

          <div className="mt-4 flex flex-col gap-2">
            {studentsLoading && students.length === 0 ? (
              <div className="moto-content-surface rounded-xl border px-4 py-6 text-center text-sm text-gray-500">
                Kinder werden geladen...
              </div>
            ) : studentsError ? null : students.length === 0 ? (
              <div className="moto-content-surface rounded-xl border border-dashed px-4 py-6 text-center text-sm text-gray-500 shadow-sm">
                Aktuell keine Kinder unterwegs.
              </div>
            ) : (
              students.map((student) => {
                const studentId = student.id.toString();
                const checked = selectedIds.has(studentId);
                const fullName =
                  `${student.first_name ?? ""} ${student.second_name ?? ""}`.trim() ||
                  "Kind";
                return (
                  <ChoiceTile
                    as="div"
                    key={student.id}
                    selected={checked}
                    className="gap-2"
                  >
                    {attendanceWebEnabled ? (
                      <button
                        type="button"
                        role="checkbox"
                        aria-checked={checked}
                        aria-label={`${fullName} auswählen`}
                        onClick={() => toggleSelected(studentId)}
                        className="flex min-w-0 flex-1 items-center gap-3 rounded-lg text-left focus:outline-none focus-visible:ring-2 focus-visible:ring-gray-300"
                      >
                        <span
                          className={`flex h-5 w-5 shrink-0 items-center justify-center rounded-md border shadow-sm transition-all ${
                            checked
                              ? "border-gray-900 bg-gray-900"
                              : "border-gray-300 bg-white"
                          }`}
                          aria-hidden="true"
                        >
                          <Check
                            className={`h-3.5 w-3.5 text-white transition-opacity ${
                              checked ? "opacity-100" : "opacity-0"
                            }`}
                          />
                        </span>
                        <CompactStudentCard
                          studentId={student.id}
                          firstName={student.first_name}
                          lastName={student.second_name}
                          schoolClass={student.school_class}
                          groupName={student.group_name ?? undefined}
                          photoUrl={student.photo_url ?? null}
                          chrome="plain"
                        />
                      </button>
                    ) : (
                      <div className="min-w-0 flex-1">
                        <CompactStudentCard
                          studentId={student.id}
                          firstName={student.first_name}
                          lastName={student.second_name}
                          schoolClass={student.school_class}
                          groupName={student.group_name ?? undefined}
                          photoUrl={student.photo_url ?? null}
                          chrome="plain"
                        />
                      </div>
                    )}
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() =>
                        router.push(
                          `/students/${student.id}?from=${encodeURIComponent(
                            fromReferrer,
                          )}`,
                        )
                      }
                      aria-label={`${fullName} Profil öffnen`}
                      title="Profil öffnen"
                      className="h-8 shrink-0 gap-1.5 rounded-full px-2.5 py-0 text-xs font-medium shadow-none"
                    >
                      Profil
                      <ExternalLink
                        className="h-3.5 w-3.5"
                        aria-hidden="true"
                      />
                    </Button>
                  </ChoiceTile>
                );
              })
            )}
          </div>
        </>
      ) : null}
    </section>
  );
}
