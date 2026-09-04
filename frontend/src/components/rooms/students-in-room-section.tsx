// components/rooms/students-in-room-section.tsx
//
// Live "Kinder im Raum" view inside the room-detail slide-over (#1323).
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

import { useEffect, useMemo, useState } from "react";
import { Check, ExternalLink } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import type { Session } from "next-auth";
import { useTenantRouter } from "~/lib/tenant-router";
import { useSWRAuth, useTenantMutateMatching } from "~/lib/swr";
import { ROOM_LIST_CACHE_KEYS } from "~/lib/swr/room-derived-caches";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { DatabaseSelect } from "~/components/ui/database/database-select";
import { ListSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { useToast } from "~/contexts/ToastContext";
import { roomService, studentService } from "~/lib/api";
import type { Student } from "~/lib/api";
import {
  activeService,
  summarizeStudentMoveResult,
} from "~/lib/active-service";
import type { ActiveGroup, Supervisor, Visit } from "~/lib/active-helpers";
import type { Room } from "~/lib/room-helpers";
import { userContextService } from "~/lib/usercontext-api";
import type { Staff } from "~/lib/usercontext-helpers";
import { CompactStudentCard } from "~/components/students/compact-student-card";
import { useStudentPhotosEnabled } from "~/lib/hooks/use-student-photos-enabled";
import { createLogger } from "~/lib/logger";
import { useAttendanceWebEnabled } from "~/lib/tenant-context";

const logger = createLogger({ component: "StudentsInRoomSection" });
const EMPTY_STUDENTS: Student[] = [];
const DETAIL_CARD_CLASS =
  "moto-content-surface rounded-2xl border p-5 shadow-sm sm:p-6";

function canUseAllMoveTargets(session: Session | null): boolean {
  const permissions = session?.user?.permissions ?? [];
  return (
    session?.user?.isAdmin === true ||
    session?.user?.roles?.includes("admin") === true ||
    permissions.includes("admin:*") ||
    permissions.includes("*:*")
  );
}

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
    "room-detail-",
    ...ROOM_LIST_CACHE_KEYS,
    "search-students-",
    "database-students-list",
  ]);
  const [selectedStudentIds, setSelectedStudentIds] = useState<Set<string>>(
    () => new Set(),
  );
  const [targetActiveGroupId, setTargetActiveGroupId] = useState("");
  const [bulkMoveState, setBulkMoveState] = useState<
    { type: "idle" } | { type: "loading" } | { type: "error"; message: string }
  >({ type: "idle" });
  const sectionSearchParams = useSearchParams();
  // Drilling into a child must return to the same /rooms grid state the
  // user came from. Preserve the full query string (room=… AND any
  // filter params like ?search=…&building=…&status=…) so the user lands
  // back on the same narrowed view with the slide-over reopened.
  const fromReferrer = (() => {
    const qs = sectionSearchParams?.toString() ?? "";
    return qs ? `/rooms?${qs}` : `/rooms?room=${roomId}`;
  })();

  // pageSize must comfortably exceed any realistic room occupancy (combined
  // groups in Sporthalle/Aula/Mensa cap well below 100 in normal usage) so
  // every present child shows up. Backend default is only 50, which would
  // silently truncate larger rooms, see PR #1374 review.
  const { data, error, isLoading } = useSWRAuth<{
    students: Student[];
    pagination?: { total_records: number };
  }>(`room-students-${roomId}`, async () =>
    studentService.getStudents({
      roomId,
      pageSize: 200,
    }),
  );
  const { data: activeGroups = [] } = useSWRAuth<ActiveGroup[]>(
    "room-bulk-active-groups",
    () => activeService.getActiveGroups({ active: true }),
  );
  const { data: currentStaff } = useSWRAuth<Staff>(
    showAllTargets ? null : "room-bulk-current-staff",
    () => userContextService.getCurrentStaff(),
  );
  const { data: activeSupervisions = [] } = useSWRAuth<Supervisor[]>(
    showAllTargets || !currentStaff?.id
      ? null
      : `room-bulk-active-supervisions-${currentStaff.id}`,
    () => activeService.getStaffActiveSupervisions(currentStaff?.id ?? ""),
  );
  const { data: rooms = [] } = useSWRAuth<Room[]>("room-bulk-rooms", () =>
    roomService.getRooms(),
  );

  if (error) {
    logger.warn("students_in_room_load_failed", {
      room_id: roomId,
      error: error instanceof Error ? error.message : String(error),
    });
  }

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
    () => sourceGroups.map((group) => group.id),
    [sourceGroups],
  );
  const { data: sourceVisitsByStudentID } = useSWRAuth<Map<string, Visit>>(
    !showAllTargets && sourceGroupIDs.length > 0
      ? `room-bulk-source-visits-${sourceGroupIDs.join("-")}`
      : null,
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
    (option) => option.activeGroupId === targetActiveGroupId,
  );
  const selectedTargetIsOwn =
    selectedTarget !== undefined &&
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
    setTargetActiveGroupId("");
    setBulkMoveState({ type: "idle" });
  }, [roomId]);

  useEffect(() => {
    onSelectionActiveChange?.(selectedVisibleCount > 0);
  }, [onSelectionActiveChange, selectedVisibleCount]);

  useEffect(() => {
    if (
      targetActiveGroupId &&
      !targetOptions.some(
        (option) => option.activeGroupId === targetActiveGroupId,
      )
    ) {
      setTargetActiveGroupId("");
    }
  }, [targetActiveGroupId, targetOptions]);

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
    setBulkMoveState({ type: "idle" });
  };

  const toggleStudentSelection = (studentId: string) => {
    if (selectedStudentIds.has(studentId)) {
      setSelectedStudentIds((current) => {
        const next = new Set(current);
        next.delete(studentId);
        return next;
      });
      setBulkMoveState({ type: "idle" });
      return;
    }

    setSelectedStudentIds((current) => {
      const next = new Set(current);
      next.add(studentId);
      return next;
    });
    setBulkMoveState({ type: "idle" });
  };

  const selectAllVisible = () => {
    setSelectedStudentIds(new Set(selectableStudentIds));
    setBulkMoveState({ type: "idle" });
  };

  const changeTarget = (activeGroupId: string) => {
    setTargetActiveGroupId(activeGroupId);
    setBulkMoveState({ type: "idle" });
  };

  const moveSelectedStudents = async () => {
    const target = selectedTarget;
    if (!target) {
      setBulkMoveState({
        type: "error",
        message: "Bitte wähle zuerst einen Zielraum aus.",
      });
      return;
    }

    const studentIds = [...selectedStudentIds].filter((studentId) =>
      visibleStudentIds.has(studentId),
    );
    if (studentIds.length === 0) {
      setBulkMoveState({
        type: "error",
        message: "Bitte wähle mindestens ein Kind aus.",
      });
      return;
    }

    setBulkMoveState({ type: "loading" });
    try {
      const result = await activeService.moveStudentsToActiveGroup(
        studentIds,
        target.activeGroupId,
      );
      const skipped = result.skipped.length;
      if (skipped > 0) {
        logger.warn("room_bulk_move_partial_failure", {
          selected_count: studentIds.length,
          skipped_count: skipped,
          target_active_group_id: target.activeGroupId,
          skipped_reasons: result.skipped.map((item) => item.reason),
        });
        setBulkMoveState({
          type: "error",
          message: `${skipped} von ${studentIds.length} Kindern konnten nicht bewegt werden.`,
        });
        await refreshRoomConsumers();
        return;
      }

      setSelectedStudentIds(new Set());
      setTargetActiveGroupId("");
      setBulkMoveState({ type: "idle" });
      const successCount = summarizeStudentMoveResult(result).successCount;
      toastSuccess(
        `${successCount} ${
          successCount === 1 ? "Kind" : "Kinder"
        } nach ${target.roomName} bewegt.`,
      );
      await refreshRoomConsumers();
    } catch (err) {
      logger.warn("room_bulk_move_partial_failure", {
        selected_count: studentIds.length,
        target_active_group_id: target.activeGroupId,
        error: err instanceof Error ? err.message : String(err),
      });
      setBulkMoveState({
        type: "error",
        message: "Die ausgewählten Kinder konnten nicht bewegt werden.",
      });
      await refreshRoomConsumers();
    }
  };

  return (
    // Quiet section header to match the other slide-over blocks
    // (Rauminformationen / Belegungshistorie). Section element +
    // aria-label preserve the "info-card" landmark contract the tests
    // / a11y tooling expect from the previous InfoCard wrapper.
    <section aria-label="Kinder im Raum" className={DETAIL_CARD_CLASS}>
      <div className="mb-3 flex items-center justify-between gap-3">
        <h2 className="text-xs font-semibold tracking-wider text-gray-500 uppercase">
          Kinder im Raum
        </h2>
        {totalCount > 0 && (
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={openInSearch}
            aria-label="Alle Kinder öffnen"
            title="Alle Kinder öffnen"
            className="h-8 shrink-0 gap-1.5 rounded-full px-2.5 py-0 text-xs font-medium shadow-none"
          >
            Alle Kinder
            <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
          </Button>
        )}
      </div>
      <p className="text-sm text-gray-600">
        <span className="font-medium text-gray-900">{totalCount}</span>{" "}
        {totalCount === 1 ? "Kind" : "Kinder"} aktuell anwesend
      </p>

      {isTruncated && (
        <div
          role="status"
          className="border-moto-amber/30 bg-moto-amber/10 text-moto-amber-strong mt-3 rounded-lg border p-3 text-sm"
        >
          Es werden {students.length} von {totalCount} Kindern angezeigt.{" "}
          {hiddenCount} weitere {hiddenCount === 1 ? "Kind ist" : "Kinder sind"}{" "}
          aktuell nicht in dieser Übersicht. Öffnen Sie „Alle Kinder“, um alle
          zu sehen.
        </div>
      )}

      {/* Breathing room between the count + Kindersuche button row and
          the first student card. Without it the button bottom edge sits
          flush with the first card border. Review feedback (#1323). */}
      <div className="mt-4">
        {canBulkMove && students.length > 0 ? (
          <BulkMoveToolbar
            selectedCount={selectedVisibleCount}
            totalCount={selectableStudentIds.size}
            targetActiveGroupId={targetActiveGroupId}
            targetOptions={targetOptions}
            targetScope={targetScope}
            state={bulkMoveState}
            onSelectAll={selectAllVisible}
            onClearSelection={clearSelection}
            onTargetChange={changeTarget}
            onMoveSelected={moveSelectedStudents}
          />
        ) : null}
        <StudentsInRoomBody
          fromReferrer={fromReferrer}
          loading={isLoading}
          hasError={!!error}
          students={students}
          selectable={canBulkMove}
          selectableStudentIds={selectableStudentIds}
          selectedStudentIds={selectedStudentIds}
          onToggleStudentSelection={toggleStudentSelection}
          router={router}
        />
      </div>
    </section>
  );
}

interface TargetRoomOption {
  readonly activeGroupId: string;
  readonly roomId: string;
  readonly roomName: string;
}

/**
 * Which rooms may be offered as move targets (#2969):
 * - `all`: admins see every room with exactly one running session.
 * - `supervised`: a supervisor of THIS room may push into every room whose
 *   single running session has at least one supervisor (colleagues count),
 *   or pull into their own running session.
 * - `own`: everyone else may only pull into rooms they supervise themselves.
 */
type TargetScope = "all" | "supervised" | "own";

function buildTargetRoomOptions(
  activeGroups: readonly ActiveGroup[],
  rooms: readonly Room[],
  currentRoomId: string,
  scope: TargetScope,
  ownActiveGroupIds: ReadonlySet<string>,
): TargetRoomOption[] {
  const roomsById = new Map(rooms.map((room) => [room.id, room]));
  const groupsByRoomId = new Map<string, ActiveGroup[]>();

  // Group BEFORE filtering by eligibility: a room with one supervised and
  // one unsupervised session is still ambiguous and must not be offered.
  activeGroups.forEach((group) => {
    if (!group.isActive || group.roomId === currentRoomId) return;
    const groupsInRoom = groupsByRoomId.get(group.roomId) ?? [];
    groupsInRoom.push(group);
    groupsByRoomId.set(group.roomId, groupsInRoom);
  });

  return [...groupsByRoomId.entries()]
    .flatMap(([targetRoomId, groups]) => {
      const room = roomsById.get(targetRoomId);
      const ownGroups = groups.filter((group) =>
        ownActiveGroupIds.has(group.id),
      );
      const unambiguousGroup = groups.length === 1 ? groups[0] : undefined;

      const eligibleGroups =
        scope === "all"
          ? unambiguousGroup
            ? [unambiguousGroup]
            : []
          : scope === "own"
            ? ownGroups
            : [
                ...ownGroups,
                ...(unambiguousGroup &&
                !ownActiveGroupIds.has(unambiguousGroup.id) &&
                (unambiguousGroup.supervisorCount ?? 0) > 0
                  ? [unambiguousGroup]
                  : []),
              ];

      return eligibleGroups.map((activeGroup) => ({
        activeGroupId: activeGroup.id,
        roomId: targetRoomId,
        roomName: room?.name ?? `Raum ${targetRoomId}`,
      }));
    })
    .sort((a, b) => a.roomName.localeCompare(b.roomName, "de"));
}

interface BulkMoveToolbarProps {
  readonly selectedCount: number;
  readonly totalCount: number;
  readonly targetActiveGroupId: string;
  readonly targetOptions: readonly TargetRoomOption[];
  readonly targetScope: TargetScope;
  readonly state:
    { type: "idle" } | { type: "loading" } | { type: "error"; message: string };
  readonly onSelectAll: () => void;
  readonly onClearSelection: () => void;
  readonly onTargetChange: (value: string) => void;
  readonly onMoveSelected: () => void;
}

function BulkMoveToolbar({
  selectedCount,
  totalCount,
  targetActiveGroupId,
  targetOptions,
  targetScope,
  state,
  onSelectAll,
  onClearSelection,
  onTargetChange,
  onMoveSelected,
}: BulkMoveToolbarProps) {
  const isMoving = state.type === "loading";
  const canMove =
    selectedCount > 0 && targetActiveGroupId.length > 0 && !isMoving;
  const allSelected = totalCount > 0 && selectedCount === totalCount;
  const hasSelection = selectedCount > 0;
  const noTargets = targetOptions.length === 0;

  return (
    <div
      className={`mb-4 rounded-xl border p-3 transition-shadow ${
        hasSelection
          ? "sticky bottom-3 z-20 border-gray-200 bg-white/95 shadow-sm backdrop-blur"
          : "border-transparent bg-gray-50/80 shadow-none"
      }`}
    >
      <div className="flex items-start justify-between gap-3">
        <p className="min-w-0 text-sm font-semibold text-gray-900">
          <span className="block truncate">
            {selectedCount > 0
              ? `${selectedCount} von ${totalCount} ausgewählt`
              : "Kinder auswählen"}
          </span>
          <span className="block text-xs font-medium text-gray-500">
            {TARGET_SCOPE_HINT[targetScope]}
          </span>
        </p>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={allSelected ? onClearSelection : onSelectAll}
          disabled={isMoving}
          className="h-8 shrink-0 rounded-full px-3 py-0 text-xs shadow-none"
        >
          {allSelected ? "Aufheben" : "Alle auswählen"}
        </Button>
      </div>

      <div className="mt-3 grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
        <DatabaseSelect
          id="room-bulk-target"
          name="room-bulk-target"
          label="Zielraum"
          value={targetActiveGroupId}
          onChange={onTargetChange}
          disabled={noTargets || isMoving}
          placeholder={
            noTargets ? "Kein Zielraum verfügbar" : "Zielraum wählen"
          }
          options={targetOptions.map((option) => ({
            value: option.activeGroupId,
            label: option.roomName,
          }))}
          className="bg-white text-sm md:text-sm"
        />
        <Button
          type="button"
          variant="primary"
          size="sm"
          isLoading={isMoving}
          loadingText="Bewege..."
          onClick={onMoveSelected}
          disabled={!canMove}
          className="h-9 w-full px-3 py-2 text-xs shadow-sm sm:w-auto"
        >
          In Raum setzen
        </Button>
      </div>

      {noTargets ? (
        <p role="status" className="mt-2 text-sm text-gray-600">
          {NO_TARGET_HINT[targetScope]}
        </p>
      ) : null}

      {state.type === "error" ? (
        <p role="alert" className="text-moto-red mt-2 text-sm">
          {state.message}
        </p>
      ) : null}
    </div>
  );
}

/** One line under the toolbar heading: what may be chosen as a target. */
const TARGET_SCOPE_HINT: Record<TargetScope, string> = {
  all: "Zielraum wählen und gemeinsam verschieben",
  supervised: "Zur Auswahl stehen Räume mit laufender Aufsicht.",
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
  readonly hasError: boolean;
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
  hasError,
  students,
  selectable,
  selectableStudentIds,
  selectedStudentIds,
  onToggleStudentSelection,
  router,
}: StudentsInRoomBodyProps) {
  const { enabled: photosEnabled } = useStudentPhotosEnabled();
  if (hasError) {
    return (
      <Alert
        type="error"
        message="Die Liste der Kinder konnte nicht geladen werden."
      />
    );
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
        <ListSkeleton rows={4} avatar={photosEnabled} />
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
        <div className="moto-content-surface rounded-xl border border-dashed px-4 py-6 text-center text-sm text-gray-500 shadow-sm">
          Aktuell keine Kinder im Raum.
        </div>
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
        <button
          type="button"
          role="checkbox"
          aria-checked={isSelected}
          aria-label={`${fullName} auswählen`}
          onClick={onToggleSelection}
          className="flex min-w-0 flex-1 items-center gap-3 rounded-lg text-left focus:outline-none focus-visible:ring-2 focus-visible:ring-gray-300"
        >
          <span
            className={`flex h-5 w-5 shrink-0 items-center justify-center rounded-md border shadow-sm transition-all ${
              isSelected
                ? "border-gray-900 bg-gray-900"
                : "border-gray-300 bg-white"
            }`}
            aria-hidden="true"
          >
            <Check
              className={`h-3.5 w-3.5 text-white transition-opacity ${
                isSelected ? "opacity-100" : "opacity-0"
              }`}
            />
          </span>
          {studentCard}
        </button>
      ) : (
        <div className="flex min-w-0 flex-1 items-center gap-3">
          {studentCard}
        </div>
      )}
      <Button
        type="button"
        variant="outline"
        size="sm"
        onClick={onOpen}
        aria-label={`${fullName} Profil öffnen`}
        title="Profil öffnen"
        className="h-8 shrink-0 gap-1.5 rounded-full px-2.5 py-0 text-xs font-medium shadow-none"
      >
        Profil
        <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
      </Button>
    </ChoiceTile>
  );
}
