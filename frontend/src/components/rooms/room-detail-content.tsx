// components/rooms/room-detail-content.tsx
//
// Die Übersicht der Raumseite /rooms/[id] (#3115): Rauminformationen, Kinder
// im Raum und die Belegungshistorie. Bis #3115 lag das in einem Slide-over
// auf /rooms; die Kopfkarte (Name, Status) trägt jetzt `TenantPage`, hier
// stehen nur noch die Flächen darunter.
//
// Fetching is exposed via the `useRoomDetail` hook so the page can react to
// loading/error states with its own layout. The content component itself is
// purely presentational.

"use client";

import { useSession } from "next-auth/react";
import { BuildingsIcon, StackSimpleIcon, TagIcon } from "@phosphor-icons/react";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { SectionCard } from "~/components/ui/section-card";
import {
  DataField,
  DataFieldSkeleton,
  DataGrid,
} from "~/components/ui/detail-modal-components";
import { useSWRAuth } from "~/lib/swr";
import {
  formatDate,
  formatTime,
  calculateDuration,
  formatDuration,
} from "~/lib/date-helpers";
import {
  formatFloor,
  mapRoomResponse,
  ROOM_HISTORY_STATUS_FEATURE_DISABLED,
  type BackendRoom,
  type Room,
} from "~/lib/room-helpers";
import { createLogger } from "~/lib/logger";
import { useStudentPhotosEnabled } from "~/lib/hooks/use-student-photos-enabled";
import { StudentsInRoomSection } from "./students-in-room-section";

const logger = createLogger({ component: "RoomDetailContent" });

// One row in the room occupancy history. Issue #1425 moved this from
// per-student-visit rows to one row per active.groups session — no student
// IDs or names by design. Per-child detail lives behind
// /students/{id}/attendance-history under its own GDPR gates.
export interface RoomHistoryEntry {
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

interface BackendRoomHistoryEntry {
  session_id: number;
  started_at: string;
  ended_at?: string | null;
  duration_minutes?: number | null;
  activity_name: string;
  supervisor_name: string;
  student_count: number;
}

/** Ältere Antworten trugen den Namen als `room_name`. */
type RoomPayload = BackendRoom & { room_name?: string };

function mapBackendToFrontendRoom(backendRoom: RoomPayload): Room {
  return mapRoomResponse({
    ...backendRoom,
    name: backendRoom.name ?? backendRoom.room_name ?? "",
  });
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

export interface UseRoomDetailResult {
  room: Room | null;
  history: RoomHistoryEntry[];
  loading: boolean;
  error: string | null;
  // True when the tenant has gdpr.attendance_log_enabled = false. The proxy
  // route (`/api/rooms/[id]/history`) translates the backend's 403 into
  // `status: "feature_disabled"` so the page can hide the
  // "Belegungshistorie" section deliberately (issue #1425) rather than
  // letting it collapse incidentally on an empty history array.
  historyDisabled: boolean;
}

/** SWR-Schlüssel der Raumseite; das globale SSE lädt darüber nach. */
export function roomDetailKey(roomId: string): string {
  return `room-detail-${roomId}`;
}

export function useRoomDetail(roomId: string): UseRoomDetailResult {
  const { data: session } = useSession();
  const token = session?.user?.token;

  // SWR-cached so the global SSE handler can invalidate the header on
  // checkin/checkout/activity events, see use-global-sse.ts. The cache
  // key shape is "room-detail-{id}" (the slug-prefix is added by
  // useSWRAuth); SSE matches via key.includes("room-detail-").
  //
  // NOTE: the cache key intentionally does NOT include start/end range
  // params because today the page always fetches the server-side default
  // window. If a future caller starts passing range params, fold them into
  // the cache key (e.g. `room-detail-${roomId}-${start}-${end}`) so
  // distinct windows don't share a cache slot.
  const { data, error, isLoading } = useSWRAuth<{
    room: Room;
    history: RoomHistoryEntry[];
    historyDisabled: boolean;
  }>(roomDetailKey(roomId), async () => {
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
      data?: RoomPayload;
    } & RoomPayload;
    const roomData = roomResponseData.data ?? roomResponseData;
    const room = mapBackendToFrontendRoom(roomData);

    let history: RoomHistoryEntry[] = [];
    let historyDisabled = false;
    if (!historyResponse.ok) {
      // A non-OK response is NOT the same as "no history". The proxy
      // maps the GDPR feature-disabled path to 200 + status:"feature_disabled"
      // (handled in the OK branch below), so anything reaching here is a
      // genuine failure — proxy crash, backend 500, network timeout. Log
      // it so the empty section isn't indistinguishable from a real
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
      //     case set historyDisabled so the page hides the section
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

interface RoomDetailContentProps {
  readonly room: Room;
  readonly history: readonly RoomHistoryEntry[];
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
  onSelectionActiveChange,
  historyDisabled = false,
}: RoomDetailContentProps) {
  const groupedSessions = groupByDate(history);
  const hasHistory = !historyDisabled && groupedSessions.length > 0;

  return (
    <div className="space-y-4 sm:space-y-6">
      {/* Feldgruppen aus dem Kit (`DataField`/`DataGrid` in einer
          `SectionCard`), statt einer eigenen Icon-Zeile: eine Objektansicht
          zeigt ihre Felder portalweit in derselben Form. */}
      <SectionCard title="Rauminformationen">
        <DataGrid>
          {/* Raumname bewusst ausgelassen: steht schon in der Kopfkarte. */}
          {room.building && (
            <DataField
              label="Gebäude"
              icon={
                <MotoDuotoneIcon
                  icon={BuildingsIcon}
                  tone="neutral"
                  size={14}
                />
              }
            >
              {room.building}
            </DataField>
          )}
          {room.floor !== undefined && (
            <DataField
              label="Etage"
              icon={
                <MotoDuotoneIcon
                  icon={StackSimpleIcon}
                  tone="neutral"
                  size={14}
                />
              }
            >
              {formatFloor(room.floor)}
            </DataField>
          )}
          {room.category && (
            <DataField
              label="Kategorie"
              icon={<MotoDuotoneIcon icon={TagIcon} tone="neutral" size={14} />}
            >
              {room.category}
            </DataField>
          )}
          <DataField label="Status">
            {room.isOccupied ? "Belegt" : "Frei"}
          </DataField>
          {room.isOccupied && room.groupName && (
            <DataField label="Aktuelle Aktivität">{room.groupName}</DataField>
          )}
          {room.isOccupied &&
            room.studentCount !== undefined &&
            room.studentCount > 0 && (
              <DataField label="Aktuell anwesend">
                {`${room.studentCount} ${
                  room.studentCount === 1 ? "Kind" : "Kinder"
                }`}
              </DataField>
            )}
          {room.isOccupied && room.supervisorName && (
            <DataField label="Aktuelle Aufsicht">
              {room.supervisorName}
            </DataField>
          )}
        </DataGrid>
      </SectionCard>

      <StudentsInRoomSection
        roomId={room.id}
        roomName={room.name}
        onSelectionActiveChange={onSelectionActiveChange}
      />

      {hasHistory ? (
        <SectionCard title="Belegungshistorie">
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
                        calculateDuration(session.startedAt, session.endedAt));

                    return (
                      <div
                        key={session.sessionId}
                        className="moto-content-surface rounded-xl border p-4 shadow-sm"
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
        </SectionCard>
      ) : null}
    </div>
  );
}

// Content-shaped skeleton for the loading state. Mirrors the three card
// shells of the loaded layout (Rauminformationen / Kinder im Raum /
// Belegungshistorie) so the page doesn't visibly resize when real data
// arrives.
function SkeletonLine({ className = "" }: { readonly className?: string }) {
  return <div className={`animate-pulse rounded bg-gray-200 ${className}`} />;
}

function SkeletonStudentRow({ withAvatar }: { withAvatar: boolean }) {
  // Mirrors CompactStudentCard: full-width pill with name + meta line.
  // When the per-tenant photo feature is on, also reserve the avatar
  // slot (sm = 32px) so the populated row's content origin lines up
  // with the skeleton, avoids a horizontal jump when data arrives.
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

export function RoomDetailSkeleton() {
  // Read the per-tenant photo flag once at the top of the skeleton so the
  // child-rows below match the populated CompactStudentCard shape exactly
  // (avatar slot reserved when the tenant has photos on; original tighter
  // shape otherwise).
  const { enabled: photosEnabled } = useStudentPhotosEnabled();
  return (
    <output
      aria-label="Raumdetails werden geladen"
      data-testid="room-detail-skeleton"
      className="block space-y-4 sm:space-y-6"
    >
      <SectionCard title="Rauminformationen">
        <DataGrid>
          <DataFieldSkeleton />
          <DataFieldSkeleton />
          <DataFieldSkeleton />
          <DataFieldSkeleton />
        </DataGrid>
      </SectionCard>

      <SectionCard title="Kinder im Raum">
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
      </SectionCard>

      <SectionCard title="Belegungshistorie">
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
      </SectionCard>
    </output>
  );
}
