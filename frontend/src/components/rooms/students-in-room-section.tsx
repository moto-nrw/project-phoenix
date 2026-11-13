// components/rooms/students-in-room-section.tsx
//
// Live "Kinder im Raum" view on the room page /rooms/[id] (#1323, #3115).
//
// Uses the same `/api/students?room_id=` data path as Kindersuche, but
// renders a compact name + class + group row per child (CompactStudentCard)
// because the side panel doesn't have room, and doesn't need, the full
// StudentCard. Every row shown is a real, currently checked-in student;
// the total count is the server's authoritative pagination count.
//
// Live updates: SSE in `use-global-sse` invalidates `room-students-{roomId}`
// on student_checkin / student_checkout / activity_start / activity_end /
// dashboard_counts_changed events.

"use client";

import { Alert } from "~/components/ui/alert";
import { EmptyState } from "~/components/ui/empty-state";
import { me as meKeys } from "~/lib/swr/me";
import { rooms as roomsKeys } from "~/lib/swr/rooms";
import { checkinTargetKeys } from "~/lib/swr/rooms-keys";
import {
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type RefObject,
} from "react";
import { ExternalLink, UserPlus } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { useTenantRouter } from "~/lib/tenant-router";
import { useSWRAuth, useTenantMutateMatching } from "~/lib/swr";
import { Button } from "~/components/ui/button";
import { FormRow } from "~/components/ui/form-row";
import { SelectionBar } from "~/components/ui/selection-bar";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { DatabaseSelect } from "~/components/database/database-select";
import type { FormErrorInput } from "~/components/ui/form-error";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import {
  ChoiceTileListSkeleton,
  SkeletonRegion,
} from "~/components/ui/page-skeletons";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { fetchStudents } from "~/lib/student-api";
import type { Student } from "~/lib/student-helpers";
import {
  activeService,
  summarizeStudentMoveResult,
} from "~/lib/active-service";
import type { ActiveGroup, Supervisor, Visit } from "~/lib/active-helpers";
import { ApiError, wireErrorCode } from "~/lib/api-error";
import type { Room } from "~/lib/room-helpers";
import type { Staff } from "~/lib/usercontext-helpers";
import { CompactStudentCard } from "~/components/students/compact-student-card";
import { useStudentPhotosEnabled } from "~/lib/hooks/use-student-photos-enabled";
import { useRefreshAfterWrite } from "~/lib/hooks/use-refresh-after-write";
import { createLogger } from "~/lib/logger";
import { useAttendanceWebEnabled, usePresenceMode } from "~/lib/tenant-context";
import { canCheckInToRooms } from "~/lib/student-access";
import { RoomCheckinDialog } from "~/components/rooms/room-checkin-dialog";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { Checkbox } from "~/components/ui/checkbox";
import { SectionCard } from "~/components/ui/section-card";
import {
  buildTargetRoomOptions,
  canUseAllMoveTargets,
  type TargetRoomOption,
  type TargetScope,
} from "~/lib/move-targets";

const logger = createLogger({ component: "StudentsInRoomSection" });
const EMPTY_STUDENTS: Student[] = [];

interface StudentsInRoomSectionProps {
  readonly roomId: string;
  readonly roomName: string;
  readonly onSelectionActiveChange?: (active: boolean) => void;
}

export function StudentsInRoomSection({
  roomId,
  roomName,
  onSelectionActiveChange,
}: StudentsInRoomSectionProps) {
  const router = useTenantRouter();
  const { data: session } = useSession();
  const attendanceWebEnabled = useAttendanceWebEnabled();
  // Visibility never grants move rights: only administrators may target any
  // running module; staff remain limited to modules they supervise.
  const showAllTargets = canUseAllMoveTargets(session);
  const { success: toastSuccess } = useToast();
  const refreshRoomConsumers = useTenantMutateMatching([
    "room-students-",
    "room-bulk-source-visits-",
    "room-detail-",
    roomsKeys.family,
    "search-students-",
    "database-students-list",
  ]);
  // Nach dem Bewegen lädt die Liste nach; ein Fehler dabei steht als
  // Ladefehler über der Liste und bewegt nichts erneut (#6652).
  const moveAfterWrite = useRefreshAfterWrite(
    "die Anzeige der Kinder im Raum",
    refreshRoomConsumers,
  );
  const [selectedStudentIds, setSelectedStudentIds] = useState<Set<string>>(
    () => new Set(),
  );
  // The chosen target's option value: a session ID or `room:<id>` for a
  // released room.
  const [targetValue, setTargetValue] = useState("");
  const [bulkMoveState, setBulkMoveState] = useState<
    { type: "idle" } | { type: "loading" }
  >({ type: "idle" });
  // „In Raum setzen“ ist ein kleines Formular: Fehler stehen in seiner Leiste,
  // der Zielraum wird markiert (#2517).
  const moveToolbarRef = useRef<HTMLDivElement>(null);
  const moveErrors = useApiFormError(moveToolbarRef);
  const { clear: clearMoveErrors } = moveErrors;
  // „Wiederholen“ setzt die aktuell gewählten Kinder in den aktuellen Zielraum.
  const retryMoveRef = useRef<() => void>(() => undefined);
  const resetBulkMove = () => {
    setBulkMoveState({ type: "idle" });
    clearMoveErrors();
  };
  const sectionSearchParams = useSearchParams();
  // Drilling into a child must return to the room page the user came from
  // (#3115). Its own query (the `from` back to the grid or the register, the
  // open tab) rides along so the way back stays intact: child → room →
  // collection.
  const fromReferrer = (() => {
    const qs = sectionSearchParams?.toString() ?? "";
    return qs ? `/rooms/${roomId}?${qs}` : `/rooms/${roomId}`;
  })();

  // pageSize must comfortably exceed any realistic room occupancy (combined
  // groups in Sporthalle/Aula/Mensa cap well below 100 in normal usage) so
  // every present child shows up. Backend default is only 50, which would
  // silently truncate larger rooms, see PR #1374 review.
  const {
    data,
    error,
    isLoading,
    mutate: reloadStudents,
  } = useSWRAuth<{
    students: Student[];
    pagination?: { total_records: number };
  }>(`room-students-${roomId}`, async () =>
    fetchStudents({
      roomId,
      pageSize: 200,
    }),
  );
  const { data: activeGroups = [] } = useSWRAuth<ActiveGroup[]>(
    checkinTargetKeys.activeGroups(),
    () => activeService.getActiveGroups({ active: true }),
  );
  const { data: currentStaff } = useSWRAuth<Staff>(
    showAllTargets ? null : meKeys.key(),
    meKeys.fetcher,
  );
  const { data: activeSupervisions = [] } = useSWRAuth<Supervisor[]>(
    showAllTargets || !currentStaff?.id
      ? null
      : checkinTargetKeys.supervisions(currentStaff.id),
    () => activeService.getStaffActiveSupervisions(currentStaff?.id ?? ""),
  );
  const { data: rooms = [], mutate: refreshMoveRooms } = useSWRAuth<Room[]>(
    roomsKeys.key(),
    roomsKeys.fetcher,
  );

  // Ladefehler mit Katalogtext, Wiederholen und Vorgangskennung (#2517).
  const loadError = useSwrLoadError(error, "die Liste der Kinder im Raum", () =>
    reloadStudents(),
  );

  const students = data?.students ?? EMPTY_STUDENTS;
  const visibleStudentIds = useMemo(
    () => new Set(students.map((student) => String(student.id))),
    [students],
  );
  const selectedVisibleCount = [...selectedStudentIds].filter((studentId) =>
    visibleStudentIds.has(studentId),
  ).length;
  const supervisedTargetGroupIds = useMemo(
    () =>
      new Set(
        activeSupervisions
          .filter((supervision) => supervision.isActive)
          .map((supervision) => supervision.activeGroupId),
      ),
    [activeSupervisions],
  );
  // Push-or-pull (#2969), mirroring the backend's
  // MoveStudentsToActiveGroupAuthorized: a staff member may move children
  // when they supervise this room (push into any supervised, unambiguous
  // room) or when they supervise the target (pull into their own room).
  // Admins keep the school-wide view. The backend re-checks all of this
  // against the live supervisions; this only avoids offering a guaranteed
  // 403.
  const sourceGroups = useMemo(
    () =>
      activeGroups.filter((group) => group.isActive && group.roomId === roomId),
    [activeGroups, roomId],
  );
  const sourceGroupIDs = useMemo(
    () => sourceGroups.map((group) => group.id).sort(),
    [sourceGroups],
  );
  const sourceVisitsKey = useMemo(
    () =>
      !showAllTargets && sourceGroupIDs.length > 0
        ? `room-bulk-source-visits-${JSON.stringify(sourceGroupIDs)}`
        : null,
    [showAllTargets, sourceGroupIDs],
  );
  const { data: sourceVisitsByStudentID } = useSWRAuth<Map<string, Visit>>(
    sourceVisitsKey,
    async () => {
      const visits = await activeService.getVisits({
        active: true,
        activeGroupIds: sourceGroupIDs,
      });
      return new Map(
        visits
          .filter((visit) => visit.isActive)
          .map((visit) => [visit.studentId, visit]),
      );
    },
  );
  // „Kinder anmelden“ (#6953): this room as the fixed target of a check-in.
  // A released room takes the independent stay, otherwise the room's one
  // session with supervision; the rules are those of every other target.
  const presenceMode = usePresenceMode();
  const showRoomCheckin =
    attendanceWebEnabled &&
    presenceMode === "detailed" &&
    canCheckInToRooms(session);
  const roomCheckinTarget = useMemo((): TargetRoomOption | null => {
    const here = buildTargetRoomOptions(
      activeGroups,
      rooms,
      "",
      // An arriving child needs a running supervision, also for admins.
      "supervised",
      supervisedTargetGroupIds,
    ).filter((option) => option.roomId === roomId);
    return (
      here.find((option) => option.kind === "openRoom") ??
      (here.length === 1 ? (here[0] ?? null) : null)
    );
  }, [activeGroups, rooms, roomId, supervisedTargetGroupIds]);
  // Pulling present children out of their rooms needs the move rights over
  // them: an administrator, or the supervisor of this room's session.
  const canPullIntoRoom =
    showAllTargets ||
    (roomCheckinTarget?.kind === "session" &&
      supervisedTargetGroupIds.has(roomCheckinTarget.activeGroupId));
  const [roomCheckinOpen, setRoomCheckinOpen] = useState(false);
  const supervisesSourceRoom = sourceGroups.some((group) =>
    supervisedTargetGroupIds.has(group.id),
  );
  const targetScope: TargetScope = showAllTargets
    ? "all"
    : supervisesSourceRoom
      ? "supervised"
      : "own";
  const targetOptions = useMemo(
    () =>
      buildTargetRoomOptions(
        activeGroups,
        rooms,
        roomId,
        targetScope,
        supervisedTargetGroupIds,
      ),
    [activeGroups, rooms, roomId, targetScope, supervisedTargetGroupIds],
  );
  // Source supervisors always get the toolbar (with an explanation when no
  // room qualifies right now); pull-only staff get it as soon as one of
  // their own rooms is a valid target AND this room has a running session
  // to pull out of (children sit in sessions, so a list without one is
  // stale); everyone else sees a plain list.
  const canBulkMove =
    attendanceWebEnabled &&
    (targetScope !== "own" ||
      (sourceGroups.length > 0 && targetOptions.length > 0));
  const selectedTarget = targetOptions.find(
    (option) => option.value === targetValue,
  );
  // A released room is never "own": moving into it keeps the push rule, so
  // only children from groups the caller supervises may be selected.
  const selectedTargetIsOwn =
    selectedTarget?.kind === "session" &&
    supervisedTargetGroupIds.has(selectedTarget.activeGroupId);
  // A supervised target authorizes a pull from every source group. For a
  // colleague's target, however, the backend authorizes only children from
  // groups supervised by the current staff member. Before a target is chosen
  // we use that safer push rule too, so a later colleague target cannot turn
  // an already selected mixed batch into a guaranteed 403.
  const selectableStudentIds = useMemo(() => {
    if (showAllTargets || selectedTargetIsOwn || targetScope === "own") {
      return visibleStudentIds;
    }
    if (!sourceVisitsByStudentID) return new Set<string>();
    return new Set(
      students
        .filter((student) => {
          const visit = sourceVisitsByStudentID.get(String(student.id));
          return (
            visit !== undefined &&
            supervisedTargetGroupIds.has(visit.activeGroupId)
          );
        })
        .map((student) => String(student.id)),
    );
  }, [
    selectedTargetIsOwn,
    showAllTargets,
    sourceVisitsByStudentID,
    students,
    supervisedTargetGroupIds,
    targetScope,
    visibleStudentIds,
  ]);
  // Prefer the server's authoritative count so the badge stays honest even
  // if pagination ever truncates the response. Fall back to length only
  // when pagination metadata is missing.
  const totalCount = data?.pagination?.total_records ?? students.length;
  // If the response is truncated (room has more occupants than pageSize),
  // surface the overflow + offer the Kindersuche escape hatch (#1374).
  // Without this notice, staff see N cards while the badge says >N and
  // have no affordance to open the missing children.
  const isTruncated = totalCount > students.length;
  const hiddenCount = Math.max(0, totalCount - students.length);

  useEffect(() => {
    setSelectedStudentIds(new Set());
    setTargetValue("");
    setBulkMoveState({ type: "idle" });
    clearMoveErrors();
  }, [roomId, clearMoveErrors]);

  useEffect(() => {
    onSelectionActiveChange?.(selectedVisibleCount > 0);
  }, [onSelectionActiveChange, selectedVisibleCount]);

  useEffect(() => {
    if (
      targetValue &&
      !targetOptions.some((option) => option.value === targetValue)
    ) {
      setTargetValue("");
    }
  }, [targetValue, targetOptions]);

  useEffect(() => {
    setSelectedStudentIds((current) => {
      const filtered = new Set(
        [...current].filter((studentId) => selectableStudentIds.has(studentId)),
      );
      return filtered.size === current.size ? current : filtered;
    });
  }, [selectableStudentIds]);

  const openInSearch = () => {
    const qs = new URLSearchParams({
      room_id: roomId,
      room_name: roomName,
    }).toString();
    router.push(`/students/search?${qs}`);
  };

  const clearSelection = () => {
    setSelectedStudentIds(new Set());
    resetBulkMove();
  };

  const toggleStudentSelection = (studentId: string) => {
    if (selectedStudentIds.has(studentId)) {
      setSelectedStudentIds((current) => {
        const next = new Set(current);
        next.delete(studentId);
        return next;
      });
      resetBulkMove();
      return;
    }

    setSelectedStudentIds((current) => {
      const next = new Set(current);
      next.add(studentId);
      return next;
    });
    resetBulkMove();
  };

  const selectAllVisible = () => {
    setSelectedStudentIds(new Set(selectableStudentIds));
    resetBulkMove();
  };

  const changeTarget = (value: string) => {
    setTargetValue(value);
    resetBulkMove();
  };

  const moveSelectedStudents = async () => {
    const target = selectedTarget;
    clearMoveErrors();
    if (!target) {
      moveErrors.invalid("Bitte wählen Sie zuerst einen Zielraum.", {
        "room-bulk-target": "Bitte wählen Sie einen Zielraum.",
      });
      return;
    }

    const studentIds = [...selectedStudentIds].filter((studentId) =>
      visibleStudentIds.has(studentId),
    );
    if (studentIds.length === 0) {
      moveErrors.invalid("Bitte wählen Sie mindestens ein Kind.");
      return;
    }

    setBulkMoveState({ type: "loading" });
    const targetLog =
      target.kind === "openRoom"
        ? { target_room_id: target.roomId }
        : { target_active_group_id: target.activeGroupId };
    try {
      const result =
        target.kind === "openRoom"
          ? await activeService.moveStudentsToOpenRoom(
              studentIds,
              target.roomId,
            )
          : await activeService.moveStudentsToActiveGroup(
              studentIds,
              target.activeGroupId,
            );
      const skipped = result.skipped.length;
      if (skipped > 0) {
        logger.warn("room_bulk_move_partial_failure", {
          selected_count: studentIds.length,
          skipped_count: skipped,
          ...targetLog,
          skipped_reasons: result.skipped.map((item) => item.reason),
        });
        setBulkMoveState({ type: "idle" });
        // Teilerfolg, kein API-Fehler: die übrigen Kinder sind bewegt.
        moveErrors.invalid(
          `${skipped} von ${studentIds.length} Kindern konnten nicht bewegt werden.`,
        );
        await moveAfterWrite.refresh();
        return;
      }

      setSelectedStudentIds(new Set());
      setTargetValue("");
      setBulkMoveState({ type: "idle" });
      const successCount = summarizeStudentMoveResult(result).successCount;
      toastSuccess(
        successCount === 1
          ? `1 Kind ist jetzt in ${target.roomName}.`
          : `${successCount} Kinder sind jetzt in ${target.roomName}.`,
      );
      await moveAfterWrite.refresh();
    } catch (err) {
      logger.warn("room_bulk_move_partial_failure", {
        selected_count: studentIds.length,
        ...targetLog,
        error: err instanceof Error ? err.message : String(err),
      });
      const releaseRemoved =
        err instanceof ApiError &&
        wireErrorCode(err.code) === "rooms.not_released";
      if (releaseRemoved) {
        // The release was removed after the room list loaded: drop the stale
        // choice and reload the rooms, so the list stops offering it.
        setTargetValue("");
        await refreshMoveRooms();
      }
      setBulkMoveState({ type: "idle" });
      // Katalogtext; ein voller Raum oder eine volle Aktivität nennt sich
      // selbst über Code und Details (#3633).
      void moveErrors.show(err, {
        object: "das Verschieben der Kinder",
        retry: releaseRemoved ? undefined : () => retryMoveRef.current(),
      });
      await refreshRoomConsumers();
    }
  };

  useLayoutEffect(() => {
    retryMoveRef.current = () => void moveSelectedStudents();
  });

  return (
    // Quiet section header to match the other slide-over blocks
    // (Rauminformationen / Belegungshistorie). Section element +
    // aria-label keep the named landmark the tests and a11y tooling expect.
    <section aria-label="Kinder im Raum">
      {showRoomCheckin && roomCheckinTarget ? (
        <RoomCheckinDialog
          isOpen={roomCheckinOpen}
          onClose={() => setRoomCheckinOpen(false)}
          roomName={roomName}
          target={roomCheckinTarget}
          canPullPresent={canPullIntoRoom}
          onDone={moveAfterWrite.refresh}
        />
      ) : null}
      <SectionCard
        overflow="visible"
        title="Kinder im Raum"
        inlineActions
        action={
          showRoomCheckin || totalCount > 0 ? (
            <div className="flex shrink-0 flex-wrap items-center justify-end gap-1.5">
              {totalCount > 0 ? (
                <Button
                  type="button"
                  variant="outline"
                  size="compact"
                  onClick={openInSearch}
                  aria-label="Alle Kinder öffnen"
                  title="Alle Kinder öffnen"
                  className="shrink-0"
                >
                  Alle Kinder
                  <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
                </Button>
              ) : null}
              {showRoomCheckin ? (
                <Button
                  type="button"
                  variant="success"
                  size="compact"
                  onClick={() => setRoomCheckinOpen(true)}
                  disabled={roomCheckinTarget === null}
                  disabledReason={`In ${roomName} läuft gerade keine Aufsicht und der Raum ist nicht freigegeben. Kinder melden Sie hier an, sobald eine Aufsicht läuft.`}
                  tooltip={`Meldet Kinder an und setzt sie sofort in ${roomName}.`}
                  className="shrink-0"
                >
                  <UserPlus className="h-3.5 w-3.5" aria-hidden="true" />
                  Kinder anmelden
                </Button>
              ) : null}
            </div>
          ) : undefined
        }
      >
        <p className="text-sm text-gray-600">
          <span className="font-medium text-gray-900">{totalCount}</span>{" "}
          {totalCount === 1 ? "Kind" : "Kinder"} aktuell anwesend
        </p>

        {isTruncated && (
          <Alert
            type="warning"
            announce="polite"
            className="mt-3"
            message={
              <>
                Es werden {students.length} von {totalCount} Kindern angezeigt.{" "}
                {hiddenCount} weitere{" "}
                {hiddenCount === 1 ? "Kind ist" : "Kinder sind"} aktuell nicht
                in dieser Übersicht. Öffnen Sie „Alle Kinder“, um alle zu sehen.
              </>
            }
          />
        )}

        {/* Breathing room between the count + Kindersuche button row and
          the first student card. Without it the button bottom edge sits
          flush with the first card border. Review feedback (#1323). */}
        <div className="mt-4">
          <LoadErrorAlert error={moveAfterWrite.error} className="mb-4" />
          {canBulkMove && students.length > 0 ? (
            <BulkMoveToolbar
              selectedCount={selectedVisibleCount}
              totalCount={selectableStudentIds.size}
              targetValue={targetValue}
              targetOptions={targetOptions}
              targetScope={targetScope}
              state={bulkMoveState}
              error={moveErrors.error}
              containerRef={moveToolbarRef}
              onSelectAll={selectAllVisible}
              onClearSelection={clearSelection}
              onTargetChange={changeTarget}
              onMoveSelected={moveSelectedStudents}
            />
          ) : null}
          <StudentsInRoomBody
            fromReferrer={fromReferrer}
            loading={isLoading}
            loadError={loadError}
            students={students}
            selectable={canBulkMove}
            selectableStudentIds={selectableStudentIds}
            selectedStudentIds={selectedStudentIds}
            onToggleStudentSelection={toggleStudentSelection}
            router={router}
          />
        </div>
      </SectionCard>
    </section>
  );
}

interface BulkMoveToolbarProps {
  readonly selectedCount: number;
  readonly totalCount: number;
  readonly targetValue: string;
  readonly targetOptions: readonly TargetRoomOption[];
  readonly targetScope: TargetScope;
  readonly state: { type: "idle" } | { type: "loading" };
  readonly error: FormErrorInput;
  readonly containerRef: RefObject<HTMLDivElement | null>;
  readonly onSelectAll: () => void;
  readonly onClearSelection: () => void;
  readonly onTargetChange: (value: string) => void;
  readonly onMoveSelected: () => void;
}

function BulkMoveToolbar({
  selectedCount,
  totalCount,
  targetValue,
  targetOptions,
  targetScope,
  state,
  error,
  containerRef,
  onSelectAll,
  onClearSelection,
  onTargetChange,
  onMoveSelected,
}: BulkMoveToolbarProps) {
  const isMoving = state.type === "loading";
  const canMove = selectedCount > 0 && targetValue.length > 0 && !isMoving;
  const allSelected = totalCount > 0 && selectedCount === totalCount;
  const noTargets = targetOptions.length === 0;

  return (
    <SelectionBar
      ref={containerRef}
      aria-label="Kinder in einen anderen Raum setzen"
      placement="bottom"
      className="mb-4 p-3"
      count={selectedCount}
      label={
        selectedCount > 0
          ? `${selectedCount} von ${totalCount} ausgewählt`
          : "Kinder auswählen"
      }
      description={TARGET_SCOPE_HINT[targetScope]}
      busy={isMoving}
      actions={
        <Button
          type="button"
          variant="outline"
          size="compact"
          onClick={allSelected ? onClearSelection : onSelectAll}
          disabled={isMoving}
          className="shrink-0"
        >
          {allSelected ? "Aufheben" : "Alle auswählen"}
        </Button>
      }
    >
      <FormRow>
        <DatabaseSelect
          id="room-bulk-target"
          name="room-bulk-target"
          label="Zielraum"
          value={targetValue}
          onChange={onTargetChange}
          disabled={noTargets || isMoving}
          placeholder={
            noTargets ? "Kein Zielraum verfügbar" : "Zielraum wählen"
          }
          options={targetOptions.map((option) => ({
            value: option.value,
            label: option.label,
          }))}
        />
        <Button
          type="button"
          variant="primary"
          size="md"
          isLoading={isMoving}
          loadingText="Bewege..."
          onClick={onMoveSelected}
          disabled={!canMove}
          className="w-full sm:w-auto"
        >
          In Raum setzen
        </Button>
      </FormRow>

      {noTargets ? (
        <p role="status" className="mt-2 text-sm text-gray-600">
          {NO_TARGET_HINT[targetScope]}
        </p>
      ) : null}

      <FormErrorAlert message={error} className="mt-2" />
    </SelectionBar>
  );
}

/** One line under the toolbar heading: what may be chosen as a target. */
const TARGET_SCOPE_HINT: Record<TargetScope, string> = {
  all: "Zielraum wählen und gemeinsam verschieben",
  supervised:
    "Zur Auswahl stehen Räume mit laufender Aufsicht und offene Räume.",
  own: "Zur Auswahl stehen nur Räume, die Sie selbst beaufsichtigen.",
};

/** Shown instead of a dead disabled select when no room qualifies. */
const NO_TARGET_HINT: Record<TargetScope, string> = {
  all: "Zurzeit läuft in keinem anderen Raum ein Angebot.",
  supervised: "Zurzeit hat kein anderer Raum eine laufende Aufsicht.",
  own: "Zurzeit beaufsichtigen Sie keinen anderen Raum.",
};

interface StudentsInRoomBodyProps {
  readonly fromReferrer: string;
  readonly loading: boolean;
  readonly loadError: FormErrorInput;
  readonly students: readonly Student[];
  readonly selectable: boolean;
  readonly selectableStudentIds: ReadonlySet<string>;
  readonly selectedStudentIds: ReadonlySet<string>;
  readonly onToggleStudentSelection: (studentId: string) => void;
  readonly router: ReturnType<typeof useTenantRouter>;
}

function StudentsInRoomBody({
  fromReferrer,
  loading,
  loadError,
  students,
  selectable,
  selectableStudentIds,
  selectedStudentIds,
  onToggleStudentSelection,
  router,
}: StudentsInRoomBodyProps) {
  const { enabled: photosEnabled } = useStudentPhotosEnabled();
  if (loadError) {
    return <LoadErrorAlert error={loadError} />;
  }

  // All three states (loading / empty / loaded) occupy similar vertical
  // space so the section's footprint stays roughly stable across them and
  // doesn't visibly jump when real data arrives (#1323 review). Shared kit
  // primitive: SkeletonRegion is the one announcing wrapper per region, its
  // testid/label are the AT + test contract other code queries.
  if (loading && students.length === 0) {
    return (
      <SkeletonRegion
        label="Kinderliste wird geladen"
        testId="students-in-room-skeleton"
      >
        <ChoiceTileListSkeleton rows={4} avatar={photosEnabled} />
      </SkeletonRegion>
    );
  }

  if (students.length === 0) {
    // Empty state styled as a single card-shaped row so it occupies
    // similar vertical space to a populated row instead of collapsing
    // to a single text line. Same outer wrapper as the populated list
    // → no layout shift between empty and loaded states for the same
    // room.
    return (
      <div className="flex flex-col gap-2">
        <EmptyState variant="dashed" title="Aktuell keine Kinder im Raum." />
      </div>
    );
  }

  return (
    // Single column inside the slide-over: the panel is ~512px, so a
    // multi-column grid would either truncate names or shrink the rows
    // below useful tap-target size on touch.
    <div className="flex flex-col gap-2">
      {students.map((student) => (
        <SelectableStudentRow
          key={student.id}
          student={student}
          selectable={
            selectable && selectableStudentIds.has(String(student.id))
          }
          isSelected={selectedStudentIds.has(String(student.id))}
          onToggleSelection={() => onToggleStudentSelection(String(student.id))}
          onOpen={() =>
            router.push(
              `/students/${student.id}?from=${encodeURIComponent(fromReferrer)}`,
            )
          }
        />
      ))}
    </div>
  );
}

function SelectableStudentRow({
  student,
  selectable,
  isSelected,
  onToggleSelection,
  onOpen,
}: {
  readonly student: Student;
  readonly selectable: boolean;
  readonly isSelected: boolean;
  readonly onToggleSelection: () => void;
  readonly onOpen: () => void;
}) {
  const fullName =
    `${student.first_name ?? ""} ${student.second_name ?? ""}`.trim() || "Kind";

  const studentCard = (
    <CompactStudentCard
      studentId={student.id}
      firstName={student.first_name}
      lastName={student.second_name}
      schoolClass={student.school_class}
      groupName={student.group_name ?? undefined}
      photoUrl={student.photo_url ?? null}
      chrome="plain"
    />
  );

  return (
    <ChoiceTile as="div" selected={isSelected} className="gap-2">
      {selectable ? (
        <label
          htmlFor={`room-select-${student.id}`}
          className="flex min-w-0 flex-1 cursor-pointer items-center gap-3 text-left"
        >
          <Checkbox
            id={`room-select-${student.id}`}
            checked={isSelected}
            aria-label={`${fullName} auswählen`}
            onChange={onToggleSelection}
          />
          {studentCard}
        </label>
      ) : (
        <div className="flex min-w-0 flex-1 items-center gap-3">
          {studentCard}
        </div>
      )}
      <Button
        type="button"
        variant="outline"
        size="compact"
        onClick={onOpen}
        aria-label={`${fullName} Profil öffnen`}
        title="Profil öffnen"
        className="shrink-0"
      >
        Profil
        <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
      </Button>
    </ChoiceTile>
  );
}
