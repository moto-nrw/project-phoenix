// components/rooms/room-detail-content.tsx
//
// Body of the room detail view , used by both the full subpage
// (/rooms/[id]/page.tsx, kept for deep links) and the responsive modal
// rendered from /rooms (#1374).
//
// Fetching is exposed via the `useRoomDetail` hook so wrappers can react
// to loading/error states with their own layout (e.g. the page swaps in
// a Zurück button on error, the modal does nothing). The content
// component itself is purely presentational.

"use client";

import { useEffect, useRef } from "react";
import { useSession } from "next-auth/react";
import { BuildingsIcon, StackSimpleIcon, TagIcon } from "@phosphor-icons/react";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { RoomStatusBadge } from "./room-status-badge";
import { useSWRAuth } from "~/lib/swr";
import {
  formatDate,
  formatTime,
  calculateDuration,
  formatDuration,
} from "~/lib/date-helpers";
import {
  formatFloor,
  getRoomCategoryColor,
  ROOM_HISTORY_STATUS_FEATURE_DISABLED,
} from "~/lib/room-helpers";
import { createLogger } from "~/lib/logger";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";
import { useStudentPhotosEnabled } from "~/lib/hooks/use-student-photos-enabled";
import { StudentsInRoomSection } from "./students-in-room-section";

const logger = createLogger({ component: "RoomDetailContent" });
const DETAIL_CARD_CLASS =
  "moto-content-surface rounded-2xl border p-5 shadow-sm sm:p-6";

interface Room {
  id: string;
  name: string;
  building?: string;
  floor?: number;
  capacity?: number;
  category?: string;
  isOccupied: boolean;
  groupName?: string;
  activityName?: string;
  supervisorName?: string;
  deviceId?: string;
  studentCount?: number;
  color?: string;
}

// One row in the room occupancy history. Issue #1425 moved this from
// per-student-visit rows to one row per active.groups session — no student
// IDs or names by design. Per-child detail lives behind
// /students/{id}/attendance-history under its own GDPR gates.
interface RoomHistoryEntry {
  sessionId: string;
  startedAt: string; // RFC3339
  endedAt: string | null; // RFC3339 or null while session is open
  durationMinutes: number | null;
  activityName: string;
  supervisorName: string;
  studentCount: number;
}

interface DateGroup {
  date: string;
  entries: RoomHistoryEntry[];
}

interface BackendRoom {
  id: number | string;
  name: string;
  room_name?: string;
  building?: string;
  floor?: number | null;
  capacity?: number | null;
  category?: string | null;
  is_occupied: boolean;
  group_name?: string;
  activity_name?: string;
  supervisor_name?: string;
  supervisor_names?: string;
  device_id?: string;
  student_count?: number;
  color?: string | null;
  created_at?: string;
  updated_at?: string;
}

interface BackendRoomHistoryEntry {
  session_id: number;
  started_at: string;
  ended_at?: string | null;
  duration_minutes?: number | null;
  activity_name: string;
  supervisor_name: string;
  student_count: number;
}

function mapBackendToFrontendRoom(backendRoom: BackendRoom): Room {
  return {
    id: String(backendRoom.id),
    name: backendRoom.name ?? backendRoom.room_name ?? "",
    building: backendRoom.building,
    floor: backendRoom.floor ?? undefined,
    capacity: backendRoom.capacity ?? undefined,
    category: backendRoom.category ?? undefined,
    isOccupied: backendRoom.is_occupied,
    groupName: backendRoom.group_name,
    activityName: backendRoom.activity_name,
    supervisorName: backendRoom.supervisor_names ?? backendRoom.supervisor_name,
    deviceId: backendRoom.device_id,
    studentCount: backendRoom.student_count,
    color: backendRoom.color ?? getRoomCategoryColor(backendRoom.category),
  };
}

function mapBackendToFrontendHistoryEntry(
  backendEntry: BackendRoomHistoryEntry,
): RoomHistoryEntry {
  return {
    sessionId: String(backendEntry.session_id),
    startedAt: backendEntry.started_at,
    endedAt: backendEntry.ended_at ?? null,
    durationMinutes: backendEntry.duration_minutes ?? null,
    activityName: backendEntry.activity_name,
    supervisorName: backendEntry.supervisor_name,
    studentCount: backendEntry.student_count,
  };
}

function groupByDate(entries: readonly RoomHistoryEntry[]): DateGroup[] {
  const groups: Record<string, RoomHistoryEntry[]> = {};

  entries.forEach((entry) => {
    const started = new Date(entry.startedAt);
    const date = [
      started.getFullYear(),
      String(started.getMonth() + 1).padStart(2, "0"),
      String(started.getDate()).padStart(2, "0"),
    ].join("-");
    groups[date] ??= [];
    groups[date].push(entry);
  });

  return Object.keys(groups)
    .sort((a, b) => b.localeCompare(a))
    .map((date) => ({
      date,
      entries: (groups[date] ?? []).sort(
        (a, b) =>
          new Date(b.startedAt).getTime() - new Date(a.startedAt).getTime(),
      ),
    }));
}

interface UseRoomDetailResult {
  room: Room | null;
  history: RoomHistoryEntry[];
  loading: boolean;
  error: string | null;
  // True when the tenant has gdpr.attendance_log_enabled = false. The proxy
  // route (`/api/rooms/[id]/history`) translates the backend's 403 into
  // `status: "feature_disabled"` so the drawer can hide the
  // "Belegungshistorie" section deliberately (issue #1425) rather than
  // letting it collapse incidentally on an empty history array.
  historyDisabled: boolean;
}

function useRoomDetail(roomId: string): UseRoomDetailResult {
  const { data: session } = useSession();
  const token = session?.user?.token;

  // SWR-cached so the global SSE handler can invalidate the header on
  // checkin/checkout/activity events , see use-global-sse.ts. The cache
  // key shape is "room-detail-{id}" (the slug-prefix is added by
  // useSWRAuth); SSE matches via key.includes("room-detail-").
  //
  // NOTE: the cache key intentionally does NOT include start/end range
  // params because today the drawer always fetches the server-side default
  // window. If a future caller starts passing range params, fold them into
  // the cache key (e.g. `room-detail-${roomId}-${start}-${end}`) so
  // distinct windows don't share a cache slot.
  const { data, error, isLoading } = useSWRAuth<{
    room: Room;
    history: RoomHistoryEntry[];
    historyDisabled: boolean;
  }>(`room-detail-${roomId}`, async () => {
    const authHeaders = token
      ? { Authorization: `Bearer ${token}` }
      : undefined;

    const [roomResponse, historyResponse] = await Promise.all([
      fetch(`/api/rooms/${roomId}`, {
        credentials: "include",
        headers: { "Content-Type": "application/json", ...authHeaders },
      }),
      fetch(`/api/rooms/${roomId}/history`, {
        credentials: "include",
        headers: { "Content-Type": "application/json", ...authHeaders },
      }),
    ]);
    if (!roomResponse.ok) {
      throw new Error("Fehler beim Laden der Raumdaten");
    }
    const roomResponseData = (await roomResponse.json()) as {
      data?: BackendRoom;
    } & BackendRoom;
    const roomData = roomResponseData.data ?? roomResponseData;
    const room = mapBackendToFrontendRoom(roomData);

    let history: RoomHistoryEntry[] = [];
    let historyDisabled = false;
    if (!historyResponse.ok) {
      // A non-OK response is NOT the same as "no history". The proxy
      // maps the GDPR feature-disabled path to 200 + status:"feature_disabled"
      // (handled in the OK branch below), so anything reaching here is a
      // genuine failure — proxy crash, backend 500, network timeout. Log
      // it so the empty drawer section isn't indistinguishable from a real
      // outage. We deliberately leave historyDisabled=false: hiding the
      // section on real errors would mask the failure further.
      logger.warn("room_history_fetch_failed", {
        room_id: roomId,
        status: historyResponse.status,
      });
    } else {
      const historyResponseData = (await historyResponse.json()) as
        | BackendRoomHistoryEntry[]
        | { status?: string; data?: BackendRoomHistoryEntry[] | null }
        | null;
      // Four observed response shapes:
      //   - bare array (legacy)
      //   - { data: [...] } wrapped
      //   - { status: "success", data: null, message: "..." } when no history
      //   - { status: "feature_disabled", data: [] } when the tenant has
      //     gdpr.attendance_log_enabled = false (issue #1425). In that
      //     case set historyDisabled so the drawer hides the section
      //     deliberately — distinguishing "off" from "no data" matters for
      //     debugging and for surviving any future UX change that would
      //     otherwise render a placeholder for empty history.
      // Anything that isn't a real array must collapse to []; otherwise
      // .map on the next line throws (#1374 regression).
      if (
        historyResponseData &&
        typeof historyResponseData === "object" &&
        !Array.isArray(historyResponseData) &&
        historyResponseData.status === ROOM_HISTORY_STATUS_FEATURE_DISABLED
      ) {
        historyDisabled = true;
      }
      const backendHistoryEntries: BackendRoomHistoryEntry[] = (() => {
        if (Array.isArray(historyResponseData)) return historyResponseData;
        if (
          historyResponseData &&
          typeof historyResponseData === "object" &&
          "data" in historyResponseData &&
          Array.isArray(historyResponseData.data)
        ) {
          return historyResponseData.data;
        }
        return [];
      })();
      history = backendHistoryEntries.map(mapBackendToFrontendHistoryEntry);
    }

    return { room, history, historyDisabled };
  });

  if (error) {
    logger.error("failed to fetch room data", {
      error: error instanceof Error ? error.message : String(error),
    });
  }

  return {
    room: data?.room ?? null,
    history: data?.history ?? [],
    loading: isLoading,
    error: error ? "Fehler beim Laden der Raumdaten." : null,
    historyDisabled: data?.historyDisabled ?? false,
  };
}

// Icon-row layout for the room-detail card (#1323 review).
// Each row: a muted icon on the left, then a stacked
// label (small, muted) + value (regular, bold), same shape as the
// reference screenshot's DETAILS section. Keeps every field the old
// InfoItem layout had, just denser and with a clearer visual anchor
// per row so staff can scan vertically by icon.
function IconDetailRow({
  icon,
  label,
  value,
}: Readonly<{
  icon: React.ReactNode;
  label: string;
  value: React.ReactNode;
}>) {
  return (
    // items-center vertically centers the icon box against the
    // label+value text block. Earlier `items-start` + mt-0.5 nudge
    // tried to align the icon with the small label, which left it
    // visually too high relative to the bold value below.
    <div className="flex items-center gap-3 py-1">
      <div className="flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-md bg-gray-100 text-gray-500">
        {icon}
      </div>
      <div className="min-w-0 flex-1">
        <p className="text-xs leading-tight text-gray-500">{label}</p>
        <p className="truncate text-sm leading-tight font-medium text-gray-900">
          {value}
        </p>
      </div>
    </div>
  );
}

interface RoomDetailContentProps {
  readonly room: Room;
  readonly history: readonly RoomHistoryEntry[];
  readonly headerAction?: React.ReactNode;
  readonly onSelectionActiveChange?: (active: boolean) => void;
  // When true, the tenant has gdpr.attendance_log_enabled = false and the
  // "Belegungshistorie" section must be hidden deliberately (issue #1425).
  // Without this flag the section already collapses when `history` is
  // empty, but that's incidental — the explicit flag prevents a future
  // empty-state placeholder from accidentally surfacing the section on a
  // tenant that has opted out.
  readonly historyDisabled?: boolean;
}

export function RoomDetailContent({
  room,
  history,
  headerAction,
  onSelectionActiveChange,
  historyDisabled = false,
}: RoomDetailContentProps) {
  const groupedSessions = groupByDate(history);
  const hasHistory = !historyDisabled && groupedSessions.length > 0;
  const isModalContext = Boolean(headerAction);
  const titleRef = useRef<HTMLHeadingElement>(null);

  useEffect(() => {
    if (!isModalContext) return;
    titleRef.current?.focus({ preventScroll: true });
  }, [isModalContext, room.id]);

  return (
    <div>
      <div
        className={`flex items-center gap-2 ${
          headerAction
            ? "sticky top-0 z-20 border-b border-gray-200/70 bg-gray-50/95 px-5 pt-5 pb-4 backdrop-blur supports-[backdrop-filter]:bg-gray-50/85 sm:px-6"
            : "mb-5"
        }`}
      >
        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 items-center gap-2">
            <MotoDuotoneIcon
              icon={MOTO_CONCEPTS.rooms.icon}
              tone={MOTO_CONCEPTS.rooms.tone}
              size={32}
            />
            <h1
              ref={titleRef}
              tabIndex={isModalContext ? -1 : undefined}
              className="min-w-0 truncate text-2xl leading-tight font-bold text-gray-900 md:text-3xl"
            >
              {room.name}
            </h1>
            <div className="flex-shrink-0">
              <RoomStatusBadge isOccupied={room.isOccupied} />
            </div>
          </div>
          {headerAction && (room.building || room.floor !== undefined) ? (
            <p className="mt-1 truncate text-xs font-medium text-gray-500">
              {room.building && room.floor !== undefined
                ? `${room.building} · ${formatFloor(room.floor)}`
                : (room.building ?? formatFloor(room.floor ?? 0))}
            </p>
          ) : null}
        </div>
        {headerAction && (
          <div className="ml-auto flex-shrink-0">{headerAction}</div>
        )}
      </div>

      <div
        className={`space-y-4 sm:space-y-6 ${
          headerAction ? "px-5 pt-5 sm:px-6" : ""
        }`}
      >
        {/* Compact icon-row block , every field that used to live in the
            old "Rauminformationen" InfoItem stack is here, but each row
            is anchored by a brand-tinted icon so the eye can scan
            vertically without re-reading labels. Review feedback
            (#1323): the previous label-on-top / value-below layout
            wasted vertical space in the narrow slide-over. */}
        {/* Quiet section header (option A from #1323 review): small
            uppercase label instead of the bold h2 + tinted icon box.
            Inverts the previous size hierarchy where the heading
            outweighed the data underneath , now the heading is a quiet
            anchor and the IconDetailRow values carry the visual weight.
            Card outline / padding stay so the section is still
            visually grouped. */}
        <div className={DETAIL_CARD_CLASS}>
          <h2 className="mb-3 text-xs font-semibold tracking-wider text-gray-500 uppercase">
            Rauminformationen
          </h2>
          <div className="space-y-1">
            {/* Raumname intentionally omitted , already in the h1
                header above; review feedback (#1323): redundant. */}
            {room.building && (
              <IconDetailRow
                icon={
                  <MotoDuotoneIcon
                    icon={BuildingsIcon}
                    tone="neutral"
                    size={16}
                  />
                }
                label="Gebäude"
                value={room.building}
              />
            )}
            {room.floor !== undefined && (
              <IconDetailRow
                icon={
                  <MotoDuotoneIcon
                    icon={StackSimpleIcon}
                    tone="neutral"
                    size={16}
                  />
                }
                label="Etage"
                value={formatFloor(room.floor)}
              />
            )}
            {room.category && (
              <IconDetailRow
                icon={
                  <MotoDuotoneIcon icon={TagIcon} tone="neutral" size={16} />
                }
                label="Kategorie"
                value={room.category}
              />
            )}
            <IconDetailRow
              icon={
                room.isOccupied ? (
                  <MotoDuotoneIcon
                    icon={MOTO_CONCEPTS.rooms.icon}
                    tone={MOTO_CONCEPTS.rooms.tone}
                    size={16}
                  />
                ) : (
                  <MotoDuotoneIcon
                    icon={MOTO_CONCEPTS.freeRooms.icon}
                    tone={MOTO_CONCEPTS.freeRooms.tone}
                    size={16}
                  />
                )
              }
              label="Status"
              value={room.isOccupied ? "Belegt" : "Frei"}
            />
            {room.isOccupied && room.groupName && (
              <IconDetailRow
                icon={
                  <MotoDuotoneIcon
                    icon={MOTO_CONCEPTS.activities.icon}
                    tone={MOTO_CONCEPTS.activities.tone}
                    size={18}
                  />
                }
                label="Aktuelle Aktivität"
                value={room.groupName}
              />
            )}
            {room.isOccupied &&
              room.studentCount !== undefined &&
              room.studentCount > 0 && (
                <IconDetailRow
                  icon={
                    <MotoDuotoneIcon
                      icon={MOTO_CONCEPTS.children.icon}
                      tone={MOTO_CONCEPTS.children.tone}
                      size={18}
                    />
                  }
                  label="Aktuell anwesend"
                  value={`${room.studentCount} ${
                    room.studentCount === 1 ? "Kind" : "Kinder"
                  }`}
                />
              )}
            {room.isOccupied && room.supervisorName && (
              <IconDetailRow
                icon={
                  <MotoDuotoneIcon
                    icon={MOTO_CONCEPTS.supervision.icon}
                    tone={MOTO_CONCEPTS.supervision.tone}
                    size={18}
                  />
                }
                label="Aktuelle Aufsicht"
                value={room.supervisorName}
              />
            )}
          </div>
        </div>

        <StudentsInRoomSection
          roomId={room.id}
          roomName={room.name}
          onSelectionActiveChange={onSelectionActiveChange}
        />

        {hasHistory ? (
          <div className={DETAIL_CARD_CLASS}>
            <h2 className="mb-3 text-xs font-semibold tracking-wider text-gray-500 uppercase">
              Belegungshistorie
            </h2>
            <div className="space-y-6">
              {groupedSessions.map((dateGroup) => (
                <div key={dateGroup.date}>
                  <h3 className="mb-3 text-sm font-semibold text-gray-700">
                    {dateGroup.entries[0]?.startedAt
                      ? formatDate(dateGroup.entries[0].startedAt, true)
                      : ""}
                  </h3>

                  <div className="space-y-3">
                    {dateGroup.entries.map((session) => {
                      // Running sessions get a "Laufend" marker in the
                      // footer row below; suppress the top-right duration
                      // slot for them so the same state isn't labelled
                      // twice (formatDuration would otherwise render
                      // "Aktiv" next to a "Laufend" pill).
                      const isRunning = session.endedAt === null;
                      const duration = isRunning
                        ? null
                        : (session.durationMinutes ??
                          calculateDuration(
                            session.startedAt,
                            session.endedAt,
                          ));

                      return (
                        <div
                          key={session.sessionId}
                          className="rounded-xl border border-gray-100 bg-white p-4 transition-shadow hover:shadow-md"
                        >
                          <div className="mb-2 flex items-start justify-between gap-2">
                            <h4 className="font-medium text-gray-900">
                              {session.activityName || "Aktivität"}
                            </h4>
                            {duration !== null && (
                              <span className="text-xs text-gray-500">
                                {formatDuration(duration)}
                              </span>
                            )}
                          </div>

                          <div className="space-y-1 text-sm text-gray-600">
                            {session.supervisorName && (
                              <div>Aufsicht: {session.supervisorName}</div>
                            )}
                            <div>Kinder: {session.studentCount}</div>
                          </div>

                          <div className="mt-3 flex justify-between border-t border-gray-100 pt-3 text-xs text-gray-500">
                            <span>Beginn: {formatTime(session.startedAt)}</span>
                            {session.endedAt ? (
                              <span>Ende: {formatTime(session.endedAt)}</span>
                            ) : (
                              <span className="font-medium text-gray-700">
                                Laufend
                              </span>
                            )}
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </div>
              ))}
            </div>
          </div>
        ) : null}
      </div>
    </div>
  );
}

// Content-shaped skeleton for the loading state. Mirrors the three card
// shells of the loaded layout (Rauminformationen / Kinder im Raum /
// Belegungshistorie) so the slide-over body doesn't visibly resize when
// real data arrives , review feedback #1323. Pulse-animated placeholders
// for header text, badge, icon-rows, and child rows. Keeps role="status"
// + aria-label so the previous business assertion ("loading state is
// announced to AT") is preserved.
function SkeletonLine({ className = "" }: { readonly className?: string }) {
  return <div className={`animate-pulse rounded bg-gray-200 ${className}`} />;
}

function SkeletonCardShell({
  heading,
  children,
}: Readonly<{ heading: string; children: React.ReactNode }>) {
  return (
    <div className={DETAIL_CARD_CLASS}>
      <h2 className="mb-3 text-xs font-semibold tracking-wider text-gray-500 uppercase">
        {heading}
      </h2>
      {children}
    </div>
  );
}

function SkeletonIconRow() {
  // Mirrors IconDetailRow: 28×28 icon box, label line, value line.
  return (
    <div className="flex items-center gap-3 py-1">
      <div className="h-7 w-7 flex-shrink-0 animate-pulse rounded-md bg-gray-200" />
      <div className="min-w-0 flex-1 space-y-1.5">
        <SkeletonLine className="h-2.5 w-16" />
        <SkeletonLine className="h-3 w-32" />
      </div>
    </div>
  );
}

function SkeletonStudentRow({ withAvatar }: { withAvatar: boolean }) {
  // Mirrors CompactStudentCard: full-width pill with name + meta line.
  // When the per-tenant photo feature is on, also reserve the avatar
  // slot (sm = 32px) so the populated row's content origin lines up
  // with the skeleton , avoids a horizontal jump when data arrives.
  return (
    <div className="moto-content-surface flex items-center gap-3 rounded-xl border px-4 py-3">
      {withAvatar ? (
        <div className="h-8 w-8 flex-shrink-0 animate-pulse rounded-full bg-gray-200" />
      ) : null}
      <div className="min-w-0 flex-1">
        <SkeletonLine className="h-4 w-40" />
        <SkeletonLine className="mt-2 h-3 w-24" />
      </div>
    </div>
  );
}

function RoomDetailSkeleton({
  headerAction,
}: Readonly<{ headerAction?: React.ReactNode }>) {
  // Read the per-tenant photo flag once at the top of the skeleton so the
  // child-rows below match the populated CompactStudentCard shape exactly
  // (avatar slot reserved when the tenant has photos on; original tighter
  // shape otherwise).
  const { enabled: photosEnabled } = useStudentPhotosEnabled();
  return (
    <output
      aria-label="Raumdetails werden geladen"
      data-testid="room-detail-skeleton"
      className="block"
    >
      {/* Header row mirrors RoomDetailContent's real header (icon + name +
          status pill placeholders), but renders the real headerAction so
          the close control (modal/drawer X) stays available while the
          fetch is in flight instead of disappearing behind the skeleton. */}
      <div
        className={`flex items-center gap-2 ${
          headerAction
            ? "sticky top-0 z-20 border-b border-gray-200/70 bg-gray-50/95 px-5 pt-5 pb-4 backdrop-blur supports-[backdrop-filter]:bg-gray-50/85 sm:px-6"
            : "mb-5"
        }`}
      >
        <div className="flex min-w-0 flex-1 items-center gap-2">
          <MotoDuotoneIcon
            icon={MOTO_CONCEPTS.rooms.icon}
            tone={MOTO_CONCEPTS.rooms.tone}
            size={32}
          />
          <SkeletonLine className="h-7 w-48 md:h-8 md:w-64" />
          <SkeletonLine className="h-6 w-16 flex-shrink-0 rounded-full" />
        </div>
        {headerAction && (
          <div className="ml-auto flex-shrink-0">{headerAction}</div>
        )}
      </div>

      <div
        className={`space-y-4 sm:space-y-6 ${
          headerAction ? "px-5 pt-5 sm:px-6" : ""
        }`}
      >
        <SkeletonCardShell heading="Rauminformationen">
          <div className="space-y-1">
            <SkeletonIconRow />
            <SkeletonIconRow />
            <SkeletonIconRow />
            <SkeletonIconRow />
          </div>
        </SkeletonCardShell>

        <SkeletonCardShell heading="Kinder im Raum">
          {/* Subline (count + Kindersuche button) and three child rows. */}
          <div className="flex items-end justify-between gap-3">
            <SkeletonLine className="h-3 w-40" />
            <SkeletonLine className="h-7 w-32 rounded-lg" />
          </div>
          <div className="mt-4 flex flex-col gap-2">
            <SkeletonStudentRow withAvatar={photosEnabled} />
            <SkeletonStudentRow withAvatar={photosEnabled} />
            <SkeletonStudentRow withAvatar={photosEnabled} />
          </div>
        </SkeletonCardShell>

        <SkeletonCardShell heading="Belegungshistorie">
          <div className="space-y-3">
            <div className="rounded-lg border border-gray-100 bg-white p-4">
              <SkeletonLine className="h-1 w-full -translate-y-2 rounded-full" />
              <SkeletonLine className="h-4 w-32" />
              <SkeletonLine className="mt-2 h-3 w-48" />
            </div>
            <div className="rounded-lg border border-gray-100 bg-white p-4">
              <SkeletonLine className="h-1 w-full -translate-y-2 rounded-full" />
              <SkeletonLine className="h-4 w-40" />
              <SkeletonLine className="mt-2 h-3 w-36" />
            </div>
          </div>
        </SkeletonCardShell>
      </div>
    </output>
  );
}

interface RoomDetailLoaderProps {
  readonly roomId: string;
  readonly emptyAction?: React.ReactNode;
  readonly headerAction?: React.ReactNode;
  readonly onSelectionActiveChange?: (active: boolean) => void;
}

/**
 * Convenience wrapper that fetches via `useRoomDetail` and renders
 * loading / error / success states inline. Use this when the wrapper
 * doesn't need to react to error state with its own layout (e.g. the
 * modal). For wrappers that DO need to swap layouts on error (the page
 * shell adds a Zurück button), call `useRoomDetail` directly and choose.
 */
export function RoomDetailLoader({
  roomId,
  emptyAction,
  headerAction,
  onSelectionActiveChange,
}: RoomDetailLoaderProps) {
  const { room, history, loading, error, historyDisabled } =
    useRoomDetail(roomId);
  const showSkeleton = loading;

  if (showSkeleton) {
    return <RoomDetailSkeleton headerAction={headerAction} />;
  }

  if (error || !room) {
    return (
      <div className="flex flex-col items-center justify-center py-12">
        <div className="border-moto-red/30 bg-moto-red/10 text-moto-red rounded-lg border p-4">
          {error ?? "Raum nicht gefunden"}
        </div>
        {emptyAction}
      </div>
    );
  }

  return (
    <RoomDetailContent
      room={room}
      history={history}
      historyDisabled={historyDisabled}
      headerAction={headerAction}
      onSelectionActiveChange={onSelectionActiveChange}
    />
  );
}
