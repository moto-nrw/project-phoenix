// components/rooms/room-detail-content.tsx
//
// Body of the room detail view — used by both the full subpage
// (/rooms/[id]/page.tsx, kept for deep links) and the responsive modal
// rendered from /rooms (#1374).
//
// Fetching is exposed via the `useRoomDetail` hook so wrappers can react
// to loading/error states with their own layout (e.g. the page swaps in
// a Zurück button on error, the modal does nothing). The content
// component itself is purely presentational.

"use client";

import { useSession } from "next-auth/react";
import {
  Building2,
  CalendarClock,
  CircleDot,
  Layers,
  Tag,
  UserCog,
  Users,
} from "lucide-react";
import { Loading } from "~/components/ui/loading";
import { useSWRAuth } from "~/lib/swr";
import {
  formatDate,
  formatTime,
  calculateDuration,
  formatDuration,
} from "~/lib/date-helpers";
import { formatFloor } from "~/lib/room-helpers";
import { createLogger } from "~/lib/logger";
import { StudentsInRoomSection } from "./students-in-room-section";

const logger = createLogger({ component: "RoomDetailContent" });

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

interface RoomHistoryEntry {
  id: string;
  timestamp: string;
  groupName?: string;
  activityName?: string;
  category?: string;
  supervisorName?: string;
  studentCount?: number;
  duration_minutes?: number;
  entry_type: "entry" | "exit";
  reason?: string;
}

interface Activity {
  id: string;
  entryTimestamp: string;
  exitTimestamp: string | null;
  groupName?: string;
  activityName?: string;
  category?: string;
  supervisorName?: string;
  studentCount?: number;
  duration_minutes?: number;
  reason?: string;
}

interface DateGroup {
  date: string;
  entries: Activity[];
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
  id: number | string;
  room_id: number | string;
  timestamp: string;
  group_name?: string;
  activity_name?: string;
  category?: string;
  supervisor_name?: string;
  student_count?: number;
  duration_minutes?: number;
  entry_type: "entry" | "exit";
  reason?: string;
}

const categoryColors: Record<string, string> = {
  "Normaler Raum": "#4F46E5",
  Gruppenraum: "#10B981",
  Themenraum: "#8B5CF6",
  Sport: "#EC4899",
};

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
    color:
      backendRoom.color ??
      (backendRoom.category
        ? categoryColors[backendRoom.category]
        : undefined) ??
      "#6B7280",
  };
}

function mapBackendToFrontendHistoryEntry(
  backendEntry: BackendRoomHistoryEntry,
): RoomHistoryEntry {
  return {
    id: String(backendEntry.id),
    timestamp: backendEntry.timestamp,
    groupName: backendEntry.group_name,
    activityName: backendEntry.activity_name,
    category: backendEntry.category,
    supervisorName: backendEntry.supervisor_name,
    studentCount: backendEntry.student_count,
    duration_minutes: backendEntry.duration_minutes,
    entry_type: backendEntry.entry_type,
    reason: backendEntry.reason,
  };
}

function StatusBadge({ isOccupied }: Readonly<{ isOccupied: boolean }>) {
  return (
    <span
      className={`inline-flex items-center rounded-full px-3 py-1.5 text-xs font-semibold ${
        isOccupied ? "bg-red-100 text-red-700" : "bg-green-100 text-green-700"
      }`}
    >
      <span
        className={`mr-2 h-2 w-2 rounded-full ${
          isOccupied ? "animate-pulse bg-red-500" : "bg-green-500"
        }`}
      />
      {isOccupied ? "Belegt" : "Frei"}
    </span>
  );
}

function groupHistoryByActivity(history: RoomHistoryEntry[]): Activity[] {
  const grouped: Activity[] = [];
  const entriesMap: Record<string, RoomHistoryEntry> = {};

  const sortedHistory = [...history].sort(
    (a, b) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime(),
  );

  sortedHistory.forEach((entry) => {
    if (entry.entry_type === "entry") {
      const key = `${entry.groupName}-${entry.activityName}`;
      entriesMap[key] = entry;
    }
  });

  sortedHistory.forEach((exit) => {
    if (exit.entry_type === "exit") {
      const key = `${exit.groupName}-${exit.activityName}`;
      const entry = entriesMap[key];

      if (entry) {
        grouped.push({
          id: entry.id,
          entryTimestamp: entry.timestamp,
          exitTimestamp: exit.timestamp,
          groupName: entry.groupName,
          activityName: entry.activityName,
          category: entry.category,
          supervisorName: entry.supervisorName,
          studentCount: entry.studentCount,
          duration_minutes: entry.duration_minutes,
          reason: entry.reason,
        });

        delete entriesMap[key];
      }
    }
  });

  Object.values(entriesMap).forEach((entry) => {
    grouped.push({
      id: entry.id,
      entryTimestamp: entry.timestamp,
      exitTimestamp: null,
      groupName: entry.groupName,
      activityName: entry.activityName,
      category: entry.category,
      supervisorName: entry.supervisorName,
      studentCount: entry.studentCount,
      duration_minutes: entry.duration_minutes,
      reason: entry.reason,
    });
  });

  return grouped;
}

function groupByDate(activities: Activity[]): DateGroup[] {
  const groups: Record<string, Activity[]> = {};

  activities.forEach((activity) => {
    const date = new Date(activity.entryTimestamp).toLocaleDateString("de-DE");
    groups[date] ??= [];
    groups[date].push(activity);
  });

  return Object.keys(groups)
    .sort((a, b) => new Date(b).getTime() - new Date(a).getTime())
    .map((date) => ({
      date,
      entries: (groups[date] ?? []).sort(
        (a, b) =>
          new Date(a.entryTimestamp).getTime() -
          new Date(b.entryTimestamp).getTime(),
      ),
    }));
}

interface UseRoomDetailResult {
  room: Room | null;
  history: RoomHistoryEntry[];
  loading: boolean;
  error: string | null;
}

export function useRoomDetail(roomId: string): UseRoomDetailResult {
  const { data: session } = useSession();
  const token = session?.user?.token;

  // SWR-cached so the global SSE handler can invalidate the header on
  // checkin/checkout/activity events — see use-global-sse.ts. The cache
  // key shape is "room-detail-{id}" (the slug-prefix is added by
  // useSWRAuth); SSE matches via key.includes("room-detail-").
  const { data, error, isLoading } = useSWRAuth<{
    room: Room;
    history: RoomHistoryEntry[];
  }>(`room-detail-${roomId}`, async () => {
    const authHeaders = token
      ? { Authorization: `Bearer ${token}` }
      : undefined;

    const roomResponse = await fetch(`/api/rooms/${roomId}`, {
      credentials: "include",
      headers: { "Content-Type": "application/json", ...authHeaders },
    });
    if (!roomResponse.ok) {
      throw new Error("Fehler beim Laden der Raumdaten");
    }
    const roomResponseData = (await roomResponse.json()) as {
      data?: BackendRoom;
    } & BackendRoom;
    const roomData = roomResponseData.data ?? roomResponseData;
    const room = mapBackendToFrontendRoom(roomData);

    let history: RoomHistoryEntry[] = [];
    const historyResponse = await fetch(`/api/rooms/${roomId}/history`, {
      credentials: "include",
      headers: { "Content-Type": "application/json", ...authHeaders },
    });
    if (historyResponse.ok) {
      const historyResponseData = (await historyResponse.json()) as
        | BackendRoomHistoryEntry[]
        | { data?: BackendRoomHistoryEntry[] | null }
        | null;
      // Three observed response shapes:
      //   - bare array (legacy)
      //   - { data: [...] } wrapped
      //   - { status: "success", data: null, message: "..." } when no history
      // Anything that isn't a real array must collapse to []; otherwise
      // .map on the next line throws (#1374 regression).
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

    return { room, history };
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
  };
}

// Icon-row layout for the room-detail card (#1323 review).
// Each row: a brand-tinted icon on the left, then a stacked
// label (small, muted) + value (regular, bold) — same shape as the
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
      <div className="flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-md bg-[#5080D8]/10 text-[#5080D8]">
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
  // Optional slot rendered in the header row next to the StatusBadge.
  // Used by the slide-over to drop its X close button alongside the
  // room name and badge so they share one clean line. The master-detail
  // database page leaves it undefined — its container owns close
  // affordances elsewhere.
  readonly headerAction?: React.ReactNode;
}

export function RoomDetailContent({
  room,
  history,
  headerAction,
}: RoomDetailContentProps) {
  const activities = groupHistoryByActivity([...history]);
  const groupedActivities = groupByDate(activities);

  return (
    <div>
      {/* Room Header — name, status badge, and (in the slide-over) the
          X close button on a single line (#1323 review).
          Layout choices:
            • Badge sits tight to the heading (no flex-1 on h1) so the
              status reads as part of the title, not a far-right tag.
            • X gets ml-auto, so it always anchors to the far right.
            • leading-tight on h1 collapses the bold heading's line-box
              so it visually centers with the smaller badge + icon
              button instead of floating above them. */}
      <div className="mb-6 flex items-center gap-2">
        <h1 className="min-w-0 truncate text-2xl leading-tight font-bold text-gray-900 md:text-3xl">
          {room.name}
        </h1>
        <div className="flex-shrink-0">
          <StatusBadge isOccupied={room.isOccupied} />
        </div>
        {headerAction && (
          <div className="ml-auto flex-shrink-0">{headerAction}</div>
        )}
      </div>

      <div className="space-y-4 sm:space-y-6">
        {/* Compact icon-row block — every field that used to live in the
            old "Rauminformationen" InfoItem stack is here, but each row
            is anchored by a brand-tinted icon so the eye can scan
            vertically without re-reading labels. Review feedback
            (#1323): the previous label-on-top / value-below layout
            wasted vertical space in the narrow slide-over. */}
        {/* Quiet section header (option A from #1323 review): small
            uppercase label instead of the bold h2 + tinted icon box.
            Inverts the previous size hierarchy where the heading
            outweighed the data underneath — now the heading is a quiet
            anchor and the IconDetailRow values carry the visual weight.
            Card outline / padding stay so the section is still
            visually grouped. */}
        <div className="rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm sm:p-6">
          <h2 className="mb-3 text-xs font-semibold tracking-wider text-gray-500 uppercase">
            Rauminformationen
          </h2>
          <div className="space-y-1">
            {/* Raumname intentionally omitted — already in the h1
                header above; review feedback (#1323): redundant. */}
            {room.building && (
              <IconDetailRow
                icon={<Building2 className="h-4 w-4" />}
                label="Gebäude"
                value={room.building}
              />
            )}
            {room.floor !== undefined && (
              <IconDetailRow
                icon={<Layers className="h-4 w-4" />}
                label="Etage"
                value={formatFloor(room.floor)}
              />
            )}
            {room.category && (
              <IconDetailRow
                icon={<Tag className="h-4 w-4" />}
                label="Kategorie"
                value={room.category}
              />
            )}
            <IconDetailRow
              icon={<CircleDot className="h-4 w-4" />}
              label="Status"
              value={room.isOccupied ? "Belegt" : "Frei"}
            />
            {room.isOccupied && room.groupName && (
              <IconDetailRow
                icon={<CalendarClock className="h-4 w-4" />}
                label="Aktuelle Aktivität"
                value={room.groupName}
              />
            )}
            {room.isOccupied &&
              room.studentCount !== undefined &&
              room.studentCount > 0 && (
                <IconDetailRow
                  icon={<Users className="h-4 w-4" />}
                  label="Aktuell anwesend"
                  value={`${room.studentCount} ${
                    room.studentCount === 1 ? "Kind" : "Kinder"
                  }`}
                />
              )}
            {room.isOccupied && room.supervisorName && (
              <IconDetailRow
                icon={<UserCog className="h-4 w-4" />}
                label="Aktuelle Aufsicht"
                value={room.supervisorName}
              />
            )}
          </div>
        </div>

        <StudentsInRoomSection roomId={room.id} roomName={room.name} />

        {/* Quiet section header to match the Rauminformationen block
            above (#1323). */}
        <div className="rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm sm:p-6">
          <h2 className="mb-3 text-xs font-semibold tracking-wider text-gray-500 uppercase">
            Belegungshistorie
          </h2>
          {groupedActivities.length === 0 ? (
            <div className="py-8 text-center text-gray-500">
              Keine Belegungshistorie verfügbar.
            </div>
          ) : (
            <div className="space-y-6">
              {groupedActivities.map((dateGroup) => (
                <div key={dateGroup.date}>
                  <h3 className="mb-3 text-sm font-semibold text-gray-700">
                    {dateGroup.entries[0]?.entryTimestamp
                      ? formatDate(dateGroup.entries[0].entryTimestamp, true)
                      : ""}
                  </h3>

                  <div className="space-y-3">
                    {dateGroup.entries.map((activity) => {
                      const actualDuration = calculateDuration(
                        activity.entryTimestamp,
                        activity.exitTimestamp,
                      );
                      const duration =
                        activity.duration_minutes ?? actualDuration;
                      const categoryColor = activity.category
                        ? (categoryColors[activity.category] ?? "#6B7280")
                        : "#6B7280";

                      return (
                        <div
                          key={activity.id}
                          className="rounded-lg border border-gray-100 bg-white p-4 transition-shadow hover:shadow-md"
                        >
                          <div
                            className="-mx-4 -mt-4 mb-3 h-1 rounded-full"
                            style={{ backgroundColor: categoryColor }}
                          ></div>

                          <div className="mb-2 flex items-start justify-between">
                            <h4 className="font-medium text-gray-900">
                              {activity.activityName}
                            </h4>
                            <span className="text-xs text-gray-500">
                              {formatDuration(duration)}
                            </span>
                          </div>

                          <div className="space-y-1 text-sm text-gray-600">
                            {activity.supervisorName && (
                              <div>Aufsicht: {activity.supervisorName}</div>
                            )}
                            {activity.category && (
                              <div className="flex items-center gap-2">
                                <span
                                  className="inline-block h-2 w-2 rounded-full"
                                  style={{ backgroundColor: categoryColor }}
                                ></span>
                                {activity.category}
                              </div>
                            )}
                            {activity.studentCount !== undefined && (
                              <div>Teilnehmer: {activity.studentCount}</div>
                            )}
                          </div>

                          <div className="mt-3 flex justify-between border-t border-gray-100 pt-3 text-xs text-gray-500">
                            <span>
                              Beginn: {formatTime(activity.entryTimestamp)}
                            </span>
                            {activity.exitTimestamp ? (
                              <span>
                                Ende: {formatTime(activity.exitTimestamp)}
                              </span>
                            ) : (
                              <span className="font-medium text-blue-600">
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
          )}
        </div>
      </div>
    </div>
  );
}

interface RoomDetailLoaderProps {
  readonly roomId: string;
  readonly emptyAction?: React.ReactNode;
  readonly headerAction?: React.ReactNode;
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
}: RoomDetailLoaderProps) {
  const { room, history, loading, error } = useRoomDetail(roomId);

  if (loading) {
    return <Loading message="Laden..." fullPage={false} />;
  }

  if (error || !room) {
    return (
      <div className="flex flex-col items-center justify-center py-12">
        <div className="rounded-lg border border-red-200 bg-red-50 p-4 text-red-800">
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
      headerAction={headerAction}
    />
  );
}
