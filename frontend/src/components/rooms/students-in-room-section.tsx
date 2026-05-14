// components/rooms/students-in-room-section.tsx
//
// Live "Kinder im Raum" view inside the room-detail slide-over (#1323).
//
// Uses the same `/api/students?room_id=` data path as Kindersuche, but
// renders a compact name + class + group row per child (CompactStudentCard)
// because the side panel doesn't have room — and doesn't need — the full
// StudentCard. Every row shown is a real, currently checked-in student;
// the total count is the server's authoritative pagination count.
//
// Live updates: SSE in `use-global-sse` invalidates `room-students-{roomId}`
// on student_checkin / student_checkout / activity_start / activity_end /
// dashboard_counts_changed events.

"use client";

import { useMemo, useState } from "react";
import { useSearchParams } from "next/navigation";
import { useTenantRouter } from "~/lib/tenant-router";
import { useSWRAuth, useTenantMutateMatching } from "~/lib/swr";
import { ROOM_LIST_CACHE_KEYS } from "~/lib/swr/room-derived-caches";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { DatabaseSelect } from "~/components/ui/database/database-select";
import { roomService, studentService } from "~/lib/api";
import type { Student } from "~/lib/api";
import { activeService } from "~/lib/active-service";
import type { ActiveGroup } from "~/lib/active-helpers";
import type { Room } from "~/lib/room-helpers";
import { CompactStudentCard } from "~/components/students/compact-student-card";
import { useStudentPhotosEnabled } from "~/lib/hooks/use-student-photos-enabled";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StudentsInRoomSection" });
const EMPTY_STUDENTS: Student[] = [];

interface StudentsInRoomSectionProps {
  readonly roomId: string;
  readonly roomName: string;
}

export function StudentsInRoomSection({
  roomId,
  roomName,
}: StudentsInRoomSectionProps) {
  const router = useTenantRouter();
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
    | { type: "idle" }
    | { type: "loading" }
    | { type: "success"; movedCount: number; roomName: string }
    | { type: "error"; message: string }
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
  // silently truncate larger rooms — see PR #1374 review.
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
  const targetOptions = useMemo(
    () => buildTargetRoomOptions(activeGroups, rooms, roomId),
    [activeGroups, rooms, roomId],
  );
  const selectedTarget = targetOptions.find(
    (option) => option.activeGroupId === targetActiveGroupId,
  );
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
    setSelectedStudentIds((current) => {
      const next = new Set(current);
      if (next.has(studentId)) {
        next.delete(studentId);
      } else {
        next.add(studentId);
      }
      return next;
    });
    setBulkMoveState({ type: "idle" });
  };

  const selectAllVisible = () => {
    setSelectedStudentIds(
      new Set(students.map((student) => String(student.id))),
    );
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
    const results = await Promise.allSettled(
      studentIds.map(async (studentId) => {
        const visit = await activeService.getStudentCurrentVisit(studentId);
        if (!visit) {
          throw new Error(`Kind ${studentId} hat keinen aktiven Besuch.`);
        }
        if (visit.activeGroupId === target.activeGroupId) {
          return;
        }
        await activeService.updateVisit(visit.id, {
          ...visit,
          activeGroupId: target.activeGroupId,
        });
      }),
    );
    const failures = results.filter((result) => result.status === "rejected");
    if (failures.length > 0) {
      logger.warn("room_bulk_move_partial_failure", {
        selected_count: studentIds.length,
        failed_count: failures.length,
        target_active_group_id: target.activeGroupId,
      });
      setBulkMoveState({
        type: "error",
        message: `${failures.length} von ${studentIds.length} Kindern konnten nicht bewegt werden.`,
      });
      await refreshRoomConsumers();
      return;
    }

    setSelectedStudentIds(new Set());
    setTargetActiveGroupId("");
    setBulkMoveState({
      type: "success",
      movedCount: studentIds.length,
      roomName: target.roomName,
    });
    await refreshRoomConsumers();
  };

  return (
    // Quiet section header to match the other slide-over blocks
    // (Rauminformationen / Belegungshistorie). Section element +
    // aria-label preserve the "info-card" landmark contract the tests
    // / a11y tooling expect from the previous InfoCard wrapper.
    <section
      aria-label="Kinder im Raum"
      className="rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm sm:p-6"
    >
      <h2 className="mb-3 text-xs font-semibold tracking-wider text-gray-500 uppercase">
        Kinder im Raum
      </h2>
      {/* items-end so the Kindersuche button bottom-aligns with the
          count text bottom. The shared `<Button>` component has fixed
          py-3, which would make the row too tall and push the count
          text down — so this row uses an inline outline-styled button
          with py-1, matching the count text's line height. The button
          stays the same visual style as the elsewhere-used Button
          outline variant, just sized to fit a one-line row.
          Review feedback (#1323). */}
      <div className="flex flex-wrap items-end justify-between gap-3">
        <p className="text-sm text-gray-600">
          <span className="font-medium text-gray-900">{totalCount}</span>{" "}
          {totalCount === 1 ? "Kind" : "Kinder"} aktuell anwesend
        </p>
        {totalCount > 0 && (
          // Tinted brand-blue ghost button. Uses LOCATION_COLORS.OTHER_ROOM
          // (#5080D8) — same hue the IconDetailRow icons in
          // Rauminformationen above are tinted with, so the slide-over
          // has one accent color across all its sections instead of the
          // gray-outline button visually getting lost. CLAUDE.md §0 —
          // brand colors via arbitrary value syntax, never generic
          // tailwind blues.
          <button
            type="button"
            onClick={openInSearch}
            className="inline-flex items-center rounded-lg bg-[#5080D8]/10 px-3 py-1 text-sm font-medium text-[#5080D8] transition-colors hover:bg-[#5080D8]/20 focus:ring-2 focus:ring-[#5080D8]/40 focus:outline-none"
          >
            In Kindersuche öffnen
          </button>
        )}
      </div>

      {isTruncated && (
        <div
          role="status"
          className="mt-3 rounded-lg border border-[#EAB308]/30 bg-[#EAB308]/10 p-3 text-sm text-[#7a5c00]"
        >
          Es werden {students.length} von {totalCount} Kindern angezeigt.{" "}
          {hiddenCount} weitere {hiddenCount === 1 ? "Kind ist" : "Kinder sind"}{" "}
          aktuell nicht in dieser Übersicht — bitte in der Kindersuche öffnen,
          um alle zu sehen.
        </div>
      )}

      {/* Breathing room between the count + Kindersuche button row and
          the first student card. Without it the button bottom edge sits
          flush with the first card border. Review feedback (#1323). */}
      <div className="mt-4">
        {students.length > 0 ? (
          <BulkMoveToolbar
            selectedCount={selectedVisibleCount}
            totalCount={students.length}
            targetActiveGroupId={targetActiveGroupId}
            targetOptions={targetOptions}
            state={bulkMoveState}
            onSelectAll={selectAllVisible}
            onClearSelection={clearSelection}
            onTargetChange={(value) => {
              setTargetActiveGroupId(value);
              setBulkMoveState({ type: "idle" });
            }}
            onMoveSelected={moveSelectedStudents}
          />
        ) : null}
        <StudentsInRoomBody
          fromReferrer={fromReferrer}
          loading={isLoading}
          hasError={!!error}
          students={students}
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

function buildTargetRoomOptions(
  activeGroups: readonly ActiveGroup[],
  rooms: readonly Room[],
  currentRoomId: string,
): TargetRoomOption[] {
  const roomsById = new Map(rooms.map((room) => [room.id, room]));
  const groupsByRoomId = new Map<string, ActiveGroup[]>();

  activeGroups.forEach((group) => {
    if (!group.isActive || group.roomId === currentRoomId) return;
    const groupsInRoom = groupsByRoomId.get(group.roomId) ?? [];
    groupsInRoom.push(group);
    groupsByRoomId.set(group.roomId, groupsInRoom);
  });

  return [...groupsByRoomId.entries()]
    .flatMap(([targetRoomId, groups]) => {
      const activeGroup = groups.length === 1 ? groups[0] : undefined;
      if (!activeGroup) return [];
      const room = roomsById.get(targetRoomId);
      return [
        {
          activeGroupId: activeGroup.id,
          roomId: targetRoomId,
          roomName: room?.name ?? `Raum ${targetRoomId}`,
        },
      ];
    })
    .sort((a, b) => a.roomName.localeCompare(b.roomName, "de"));
}

interface BulkMoveToolbarProps {
  readonly selectedCount: number;
  readonly totalCount: number;
  readonly targetActiveGroupId: string;
  readonly targetOptions: readonly TargetRoomOption[];
  readonly state:
    | { type: "idle" }
    | { type: "loading" }
    | { type: "success"; movedCount: number; roomName: string }
    | { type: "error"; message: string };
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
  state,
  onSelectAll,
  onClearSelection,
  onTargetChange,
  onMoveSelected,
}: BulkMoveToolbarProps) {
  const isMoving = state.type === "loading";
  const canMove =
    selectedCount > 0 && targetActiveGroupId.length > 0 && !isMoving;
  const allSelected = selectedCount === totalCount;

  return (
    <div className="mb-4 rounded-xl border border-gray-200 bg-gray-50/70 p-3">
      <div className="flex items-center justify-between gap-3">
        <p className="min-w-0 text-sm font-semibold text-gray-900">
          <span className="block truncate">
            {selectedCount > 0
              ? `${selectedCount} von ${totalCount} ausgewählt`
              : "Kinder auswählen"}
          </span>
          <span className="block text-xs font-medium text-gray-500">
            Zielraum wählen und gemeinsam verschieben
          </span>
        </p>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={allSelected ? onClearSelection : onSelectAll}
          className="shrink-0 px-3 py-2 text-sm shadow-none"
        >
          {allSelected ? "Aufheben" : "Alle"}
        </Button>
      </div>

      <div className="mt-3 grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
        <DatabaseSelect
          id="room-bulk-target"
          name="room-bulk-target"
          label="Zielraum"
          value={targetActiveGroupId}
          onChange={onTargetChange}
          disabled={targetOptions.length === 0 || isMoving}
          placeholder={
            targetOptions.length === 0
              ? "Kein aktiver Zielraum verfügbar"
              : "Zielraum wählen"
          }
          options={targetOptions.map((option) => ({
            value: option.activeGroupId,
            label: option.roomName,
          }))}
          focusRingColor="focus:ring-[#5080D8]"
          className="bg-white text-sm md:text-sm"
        />
        <Button
          type="button"
          variant="success"
          size="sm"
          isLoading={isMoving}
          loadingText="Bewege..."
          onClick={onMoveSelected}
          disabled={!canMove}
          className="min-h-10 w-full bg-[#83CD2D] px-4 py-2.5 text-sm shadow-sm hover:bg-[#74b827] active:bg-[#669f21] disabled:bg-gray-200 disabled:text-gray-500 sm:w-auto"
        >
          In Raum setzen
        </Button>
      </div>

      {state.type === "success" ? (
        <p role="status" className="mt-2 text-sm text-[#4a7a15]">
          {state.movedCount} {state.movedCount === 1 ? "Kind" : "Kinder"} nach{" "}
          {state.roomName} bewegt.
        </p>
      ) : null}
      {state.type === "error" ? (
        <p role="alert" className="mt-2 text-sm text-[#FF3130]">
          {state.message}
        </p>
      ) : null}
    </div>
  );
}

interface StudentsInRoomBodyProps {
  readonly fromReferrer: string;
  readonly loading: boolean;
  readonly hasError: boolean;
  readonly students: readonly Student[];
  readonly selectedStudentIds: ReadonlySet<string>;
  readonly onToggleStudentSelection: (studentId: string) => void;
  readonly router: ReturnType<typeof useTenantRouter>;
}

// Single student-row skeleton matching CompactStudentCard's outer shell
// (rounded-xl border, px-4 py-3, name line + meta line). The skeleton
// list lives inside the same `flex flex-col gap-2` wrapper the populated
// list uses, so swapping skeleton → real cards causes zero layout shift
// (review feedback #1323). When the per-tenant photo feature is on we
// also reserve the avatar slot (sm = 32px) so the row height + horizontal
// content origin match the populated card; opt-out tenants keep the
// original tighter shape.
function StudentRowSkeleton({ withAvatar }: { withAvatar: boolean }) {
  return (
    <div className="flex items-center gap-3 rounded-xl border border-gray-200 bg-white px-4 py-3">
      {withAvatar ? (
        <div className="h-8 w-8 flex-shrink-0 animate-pulse rounded-full bg-gray-200" />
      ) : null}
      <div className="min-w-0 flex-1">
        <div className="h-4 w-40 animate-pulse rounded bg-gray-200" />
        <div className="mt-2 h-3 w-24 animate-pulse rounded bg-gray-200" />
      </div>
    </div>
  );
}

function StudentsInRoomBody({
  fromReferrer,
  loading,
  hasError,
  students,
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

  // All three states (loading / empty / loaded) live inside the same
  // `flex flex-col gap-2` wrapper so the section's vertical footprint
  // stays stable across them. Without this, the loading placeholder
  // (was: <Loading> with pt-24 pb-12) was a different shape than the
  // populated list, and the section visibly resized when data arrived.
  if (loading && students.length === 0) {
    return (
      <output
        aria-label="Kinderliste wird geladen"
        data-testid="students-in-room-skeleton"
        className="flex flex-col gap-2"
      >
        <StudentRowSkeleton withAvatar={photosEnabled} />
        <StudentRowSkeleton withAvatar={photosEnabled} />
        <StudentRowSkeleton withAvatar={photosEnabled} />
        <StudentRowSkeleton withAvatar={photosEnabled} />
      </output>
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
        <div className="rounded-xl border border-dashed border-gray-200 bg-white px-4 py-6 text-center text-sm text-gray-500">
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
  isSelected,
  onToggleSelection,
  onOpen,
}: {
  readonly student: Student;
  readonly isSelected: boolean;
  readonly onToggleSelection: () => void;
  readonly onOpen: () => void;
}) {
  const fullName =
    `${student.first_name ?? ""} ${student.second_name ?? ""}`.trim() || "Kind";

  return (
    <div
      className={`flex items-center gap-3 rounded-xl border px-3 py-3 transition-colors ${
        isSelected
          ? "border-[#5080D8]/40 bg-[#5080D8]/10"
          : "border-gray-200 bg-white hover:border-gray-300 hover:bg-gray-50"
      }`}
    >
      <input
        type="checkbox"
        checked={isSelected}
        aria-label={`${fullName} auswählen`}
        onChange={onToggleSelection}
        className="h-5 w-5 shrink-0 rounded border-gray-300 text-[#5080D8] accent-[#5080D8] focus:ring-2 focus:ring-[#5080D8]/40 focus:outline-none"
      />
      <div className="min-w-0 flex-1">
        <CompactStudentCard
          studentId={student.id}
          firstName={student.first_name}
          lastName={student.second_name}
          schoolClass={student.school_class}
          groupName={student.group_name ?? undefined}
          photoUrl={student.photo_url ?? null}
          onClick={onOpen}
          chrome="plain"
        />
      </div>
    </div>
  );
}
