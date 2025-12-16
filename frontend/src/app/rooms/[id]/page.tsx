"use client";

import { useState, useEffect } from "react";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { Loading } from "~/components/ui/loading";
import { useSession } from "next-auth/react";
import { InfoCard, InfoItem } from "~/components/ui/info-card";

// Room interface
interface Room {
  id: string;
  name: string;
  building?: string;
  floor?: number; // Optional (nullable in DB)
  capacity?: number; // Optional (nullable in DB)
  category?: string; // Optional (nullable in DB)
  isOccupied: boolean;
  groupName?: string;
  activityName?: string;
  supervisorName?: string;
  deviceId?: string;
  studentCount?: number;
  color?: string;
}

// Room History Entry interface
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

// Activity interface (for paired entries)
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

// Date Group interface
interface DateGroup {
  date: string;
  entries: Activity[];
}

// Backend response interfaces
interface BackendRoom {
  id: number | string;
  name: string;
  room_name?: string;
  building?: string;
  floor?: number | null; // Optional (nullable in DB)
  capacity?: number | null; // Optional (nullable in DB)
  category?: string | null; // Optional (nullable in DB)
  is_occupied: boolean;
  group_name?: string;
  activity_name?: string;
  supervisor_name?: string;
  device_id?: string;
  student_count?: number;
  color?: string | null; // Optional (nullable in DB)
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

// Kategorie-zu-Farbe Mapping
const categoryColors: Record<string, string> = {
  "Normaler Raum": "#4F46E5",
  Gruppenraum: "#10B981",
  Themenraum: "#8B5CF6",
  Sport: "#EC4899",
};

// Helper functions
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
    supervisorName: backendRoom.supervisor_name,
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

// Status Badge Component
function StatusBadge({ isOccupied }: { isOccupied: boolean }) {
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

export default function RoomDetailPage() {
  const router = useRouter();
  const params = useParams();
  const searchParams = useSearchParams();
  const roomId = params.id as string;
  const referrer = searchParams.get("from") ?? "/rooms";
  const { data: session } = useSession();

  const [room, setRoom] = useState<Room | null>(null);
  const [roomHistory, setRoomHistory] = useState<RoomHistoryEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Fetch room data and history
  useEffect(() => {
    const fetchRoomData = async () => {
      setLoading(true);
      setError(null);

      try {
        const authHeaders = session?.user?.token
          ? { Authorization: `Bearer ${session.user.token}` }
          : undefined;

        // Fetch room data
        const roomResponse = await fetch(`/api/rooms/${roomId}`, {
          credentials: "include",
          headers: {
            "Content-Type": "application/json",
            ...authHeaders,
          },
        });

        if (!roomResponse.ok) {
          throw new Error("Fehler beim Laden der Raumdaten");
        }

        const roomResponseData = (await roomResponse.json()) as {
          data?: BackendRoom;
        } & BackendRoom;
        const roomData = roomResponseData.data ?? roomResponseData;
        const frontendRoom = mapBackendToFrontendRoom(roomData);
        setRoom(frontendRoom);

        // Fetch room history
        const historyResponse = await fetch(`/api/rooms/${roomId}/history`, {
          credentials: "include",
          headers: {
            "Content-Type": "application/json",
            ...authHeaders,
          },
        });

        if (historyResponse.ok) {
          const historyResponseData = (await historyResponse.json()) as
            | BackendRoomHistoryEntry[]
            | { data: BackendRoomHistoryEntry[] };
          const backendHistoryEntries = Array.isArray(historyResponseData)
            ? historyResponseData
            : historyResponseData?.data &&
                Array.isArray(historyResponseData.data)
              ? historyResponseData.data
              : [];

          const frontendHistoryEntries = backendHistoryEntries.map(
            (entry: BackendRoomHistoryEntry) =>
              mapBackendToFrontendHistoryEntry(entry),
          );

          setRoomHistory(frontendHistoryEntries);
        }
      } catch (err) {
        console.error("Error fetching data:", err);
        setError("Fehler beim Laden der Raumdaten.");
      } finally {
        setLoading(false);
      }
    };

    void fetchRoomData();
  }, [roomId, session?.user?.token]);

  // Format date and time
  const formatDate = (dateString: string): string => {
    const date = new Date(dateString);
    return date.toLocaleDateString("de-DE", {
      weekday: "long",
      year: "numeric",
      month: "long",
      day: "numeric",
    });
  };

  const formatTime = (dateString: string): string => {
    const date = new Date(dateString);
    return date.toLocaleTimeString("de-DE", {
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  const calculateDuration = (
    entry: string,
    exit: string | null,
  ): number | null => {
    if (!exit) return null;
    const entryTime = new Date(entry);
    const exitTime = new Date(exit);
    const durationMs = exitTime.getTime() - entryTime.getTime();
    return Math.round(durationMs / 60000);
  };

  const formatDuration = (minutes: number | null): string => {
    if (minutes === null) return "Aktiv";
    if (minutes <= 0) return "< 1 Min.";

    const hours = Math.floor(minutes / 60);
    const mins = minutes % 60;

    if (hours > 0) {
      return `${hours} Std. ${mins > 0 ? `${mins} Min.` : ""}`;
    } else {
      return `${mins} Min.`;
    }
  };

  // Group room history entries into activities
  const groupHistoryByActivity = (history: RoomHistoryEntry[]): Activity[] => {
    const grouped: Activity[] = [];
    const entriesMap: Record<string, RoomHistoryEntry> = {};

    const sortedHistory = [...history].sort(
      (a, b) =>
        new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime(),
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
  };

  const groupByDate = (activities: Activity[]): DateGroup[] => {
    const groups: Record<string, Activity[]> = {};

    activities.forEach((activity) => {
      const date = new Date(activity.entryTimestamp).toLocaleDateString(
        "de-DE",
      );
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
  };

  const activities = groupHistoryByActivity(roomHistory);
  const groupedActivities = groupByDate(activities);

  if (loading) {
    return (
      <ResponsiveLayout roomName="...">
        <Loading message="Laden..." fullPage={false} />
      </ResponsiveLayout>
    );
  }

  if (error || !room) {
    return (
      <ResponsiveLayout>
        <div className="flex min-h-[80vh] flex-col items-center justify-center">
          <div className="mb-4 rounded-lg border border-red-200 bg-red-50 p-4 text-red-800">
            {error ?? "Raum nicht gefunden"}
          </div>
          <button
            onClick={() => router.push(referrer)}
            className="mt-4 rounded bg-blue-100 px-4 py-2 text-blue-800 transition-colors hover:bg-blue-200"
          >
            Zurück
          </button>
        </div>
      </ResponsiveLayout>
    );
  }

  return (
    <ResponsiveLayout roomName={room.name}>
      <div className="mx-auto max-w-7xl">
        {/* Back button - Mobile only (breadcrumb handles desktop navigation) */}
        <button
          onClick={() => router.push(referrer)}
          className="mb-4 -ml-1 flex items-center gap-2 py-2 pl-1 text-gray-600 transition-colors hover:text-gray-900 md:hidden"
        >
          <svg
            className="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M15 19l-7-7 7-7"
            />
          </svg>
          <span className="text-sm font-medium">Zurück</span>
        </button>

        {/* Room Header - Mobile optimized with underline */}
        <div className="mb-6">
          <div className="flex items-end justify-between gap-4">
            {/* Title with underline */}
            <div className="ml-6 flex-1">
              <div className="relative inline-block pb-3">
                <h1 className="text-2xl font-bold text-gray-900 md:text-3xl">
                  {room.name}
                </h1>
                {/* Underline indicator - matches tab style */}
                <div
                  className="absolute bottom-0 left-0 h-0.5 rounded-full bg-gray-900"
                  style={{ width: "70%" }}
                />
              </div>
              <div className="mt-3 flex flex-wrap items-center gap-2 text-sm text-gray-600 sm:gap-4">
                {(room.building !== undefined || room.floor !== undefined) && (
                  <>
                    <span>
                      {room.building &&
                        room.floor !== undefined &&
                        `${room.building} · Etage ${room.floor}`}
                      {room.building &&
                        room.floor === undefined &&
                        room.building}
                      {!room.building &&
                        room.floor !== undefined &&
                        `Etage ${room.floor}`}
                    </span>
                    {room.category && (
                      <span className="hidden sm:inline">•</span>
                    )}
                  </>
                )}
                {room.category && (
                  <span className="truncate">{room.category}</span>
                )}
              </div>
            </div>

            {/* Status Badge */}
            <div className="mr-4 flex-shrink-0 pb-3">
              <StatusBadge isOccupied={room.isOccupied} />
            </div>
          </div>
        </div>

        <div className="space-y-4 sm:space-y-6">
          {/* Room Information */}
          <InfoCard
            title="Rauminformationen"
            icon={
              <svg
                className="h-5 w-5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"
                />
              </svg>
            }
          >
            <InfoItem label="Raumname" value={room.name} />
            {room.building && (
              <InfoItem label="Gebäude" value={room.building} />
            )}
            {room.floor !== undefined && (
              <InfoItem label="Etage" value={`Etage ${room.floor}`} />
            )}
            {room.category && (
              <InfoItem label="Kategorie" value={room.category} />
            )}
            <InfoItem
              label="Status"
              value={room.isOccupied ? "Belegt" : "Frei"}
            />

            {/* Current Occupation - integrated into room info */}
            {room.isOccupied && room.groupName && (
              <div className="pt-3">
                <InfoItem
                  label="Aktuelle Aktivität"
                  value={
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="inline-flex items-center gap-1.5 rounded-full bg-red-100 px-3 py-1 text-sm font-semibold text-red-800">
                        <svg
                          className="h-4 w-4"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                        >
                          <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth={2}
                            d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
                          />
                        </svg>
                        {room.groupName}
                      </span>
                    </div>
                  }
                />
              </div>
            )}
          </InfoCard>

          {/* Room History */}
          <InfoCard
            title="Belegungshistorie"
            icon={
              <svg
                className="h-5 w-5"
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
            }
          >
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
                        ? formatDate(dateGroup.entries[0].entryTimestamp)
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
                            {/* Color indicator */}
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
          </InfoCard>
        </div>
      </div>
    </ResponsiveLayout>
  );
}
