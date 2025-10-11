"use client";

import { useState, useEffect } from "react";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { useSession } from "next-auth/react";

// Room interface
interface Room {
  id: string;
  name: string;
  building?: string;
  floor: number;
  capacity: number;
  category: string;
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
  floor: number;
  capacity: number;
  category: string;
  is_occupied: boolean;
  group_name?: string;
  activity_name?: string;
  supervisor_name?: string;
  device_id?: string;
  student_count?: number;
  color?: string;
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
  "Gruppenraum": "#4F46E5",
  "Lernen": "#10B981",
  "Spielen": "#F59E0B",
  "Bewegen/Ruhe": "#EC4899",
  "Hauswirtschaft": "#EF4444",
  "Natur": "#22C55E",
  "Kreatives/Musik": "#8B5CF6",
  "NW/Technik": "#06B6D4",
  "Klassenzimmer": "#4F46E5",
};

// Helper functions
function mapBackendToFrontendRoom(backendRoom: BackendRoom): Room {
  return {
    id: String(backendRoom.id),
    name: backendRoom.name ?? backendRoom.room_name ?? "",
    building: backendRoom.building,
    floor: backendRoom.floor,
    capacity: backendRoom.capacity,
    category: backendRoom.category,
    isOccupied: backendRoom.is_occupied,
    groupName: backendRoom.group_name,
    activityName: backendRoom.activity_name,
    supervisorName: backendRoom.supervisor_name,
    deviceId: backendRoom.device_id,
    studentCount: backendRoom.student_count,
    color: backendRoom.color ?? categoryColors[backendRoom.category] ?? "#6B7280"
  };
}

function mapBackendToFrontendHistoryEntry(backendEntry: BackendRoomHistoryEntry): RoomHistoryEntry {
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
    reason: backendEntry.reason
  };
}

// Status Badge Component
function StatusBadge({ isOccupied }: { isOccupied: boolean }) {
  return (
    <span
      className={`inline-flex items-center px-3 py-1.5 rounded-full text-xs font-semibold ${
        isOccupied
          ? "bg-red-100 text-red-700"
          : "bg-green-100 text-green-700"
      }`}
    >
      <span className={`w-2 h-2 rounded-full mr-2 ${
        isOccupied ? "bg-red-500 animate-pulse" : "bg-green-500"
      }`} />
      {isOccupied ? "Belegt" : "Frei"}
    </span>
  );
}

// InfoCard Component
function InfoCard({ title, children, icon }: { title: string; children: React.ReactNode; icon: React.ReactNode }) {
  return (
    <div className="bg-white/50 backdrop-blur-sm rounded-2xl border border-gray-100 p-4 sm:p-6">
      <div className="flex items-center gap-3 mb-4">
        <div className="h-9 w-9 sm:h-10 sm:w-10 rounded-lg bg-gray-100 flex items-center justify-center text-gray-600 flex-shrink-0">
          {icon}
        </div>
        <h2 className="text-base sm:text-lg font-semibold text-gray-900">{title}</h2>
      </div>
      <div className="space-y-3">{children}</div>
    </div>
  );
}

// InfoItem Component
function InfoItem({ label, value }: { label: string; value: string | React.ReactNode }) {
  return (
    <div className="flex items-start gap-3">
      <div className="flex-1 min-w-0">
        <p className="text-xs text-gray-500 mb-1">{label}</p>
        <div className="text-sm text-gray-900 font-medium">{value}</div>
      </div>
    </div>
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
            ...authHeaders
          }
        });

        if (!roomResponse.ok) {
          throw new Error("Fehler beim Laden der Raumdaten");
        }

        const roomResponseData = await roomResponse.json() as { data?: BackendRoom } & BackendRoom;
        const roomData = roomResponseData.data ?? roomResponseData;
        const frontendRoom = mapBackendToFrontendRoom(roomData);
        setRoom(frontendRoom);

        // Fetch room history
        const historyResponse = await fetch(`/api/rooms/${roomId}/history`, {
          credentials: "include",
          headers: {
            "Content-Type": "application/json",
            ...authHeaders
          }
        });

        if (historyResponse.ok) {
          const historyResponseData = await historyResponse.json() as BackendRoomHistoryEntry[] | { data: BackendRoomHistoryEntry[] };
          const backendHistoryEntries = Array.isArray(historyResponseData)
            ? historyResponseData
            : (historyResponseData?.data && Array.isArray(historyResponseData.data))
              ? historyResponseData.data
              : [];

          const frontendHistoryEntries = backendHistoryEntries.map(
            (entry: BackendRoomHistoryEntry) => mapBackendToFrontendHistoryEntry(entry)
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
    return date.toLocaleDateString('de-DE', {
      weekday: 'long',
      year: 'numeric',
      month: 'long',
      day: 'numeric'
    });
  };

  const formatTime = (dateString: string): string => {
    const date = new Date(dateString);
    return date.toLocaleTimeString('de-DE', {
      hour: '2-digit',
      minute: '2-digit'
    });
  };

  const calculateDuration = (entry: string, exit: string | null): number | null => {
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
      return `${hours} Std. ${mins > 0 ? `${mins} Min.` : ''}`;
    } else {
      return `${mins} Min.`;
    }
  };

  // Group room history entries into activities
  const groupHistoryByActivity = (history: RoomHistoryEntry[]): Activity[] => {
    const grouped: Activity[] = [];
    const entriesMap: Record<string, RoomHistoryEntry> = {};

    const sortedHistory = [...history].sort(
      (a, b) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime()
    );

    sortedHistory.forEach(entry => {
      if (entry.entry_type === "entry") {
        const key = `${entry.groupName}-${entry.activityName}`;
        entriesMap[key] = entry;
      }
    });

    sortedHistory.forEach(exit => {
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
            reason: entry.reason
          });

          delete entriesMap[key];
        }
      }
    });

    Object.values(entriesMap).forEach(entry => {
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
        reason: entry.reason
      });
    });

    return grouped;
  };

  const groupByDate = (activities: Activity[]): DateGroup[] => {
    const groups: Record<string, Activity[]> = {};

    activities.forEach(activity => {
      const date = new Date(activity.entryTimestamp).toLocaleDateString('de-DE');
      groups[date] ??= [];
      groups[date].push(activity);
    });

    return Object.keys(groups)
      .sort((a, b) => new Date(b).getTime() - new Date(a).getTime())
      .map(date => ({
        date,
        entries: (groups[date] ?? []).sort((a, b) =>
          new Date(a.entryTimestamp).getTime() - new Date(b.entryTimestamp).getTime()
        )
      }));
  };

  const activities = groupHistoryByActivity(roomHistory);
  const groupedActivities = groupByDate(activities);

  if (loading) {
    return (
      <ResponsiveLayout>
        <div className="flex min-h-[80vh] items-center justify-center">
          <div className="flex flex-col items-center gap-4">
            <div className="h-12 w-12 animate-spin rounded-full border-b-2 border-t-2 border-blue-500"></div>
            <p className="text-gray-600">Daten werden geladen...</p>
          </div>
        </div>
      </ResponsiveLayout>
    );
  }

  if (error || !room) {
    return (
      <ResponsiveLayout>
        <div className="flex min-h-[80vh] flex-col items-center justify-center">
          <div className="mb-4 p-4 bg-red-50 border border-red-200 rounded-lg text-red-800">
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
    <ResponsiveLayout>
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 pb-6">
        {/* Back button - Mobile optimized */}
        <button
          onClick={() => router.push(referrer)}
          className="flex items-center gap-2 mb-4 text-gray-600 hover:text-gray-900 transition-colors py-2 -ml-1 pl-1"
        >
          <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
          </svg>
          <span className="text-sm font-medium">Zurück</span>
        </button>

        {/* Room Header - Mobile optimized */}
        <div className="bg-white/50 backdrop-blur-sm rounded-2xl border border-gray-100 p-4 sm:p-6 mb-6">
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
            <div className="flex-1 min-w-0">
              <h1 className="text-xl sm:text-2xl font-bold text-gray-900 truncate">
                {room.name}
              </h1>
              <div className="flex flex-wrap items-center gap-2 sm:gap-4 mt-2 text-sm text-gray-600">
                <span>{room.building ?? "Unbekannt"} · Etage {room.floor}</span>
                <span className="hidden sm:inline">•</span>
                <span className="truncate">{room.category}</span>
              </div>
            </div>
            <div className="flex-shrink-0">
              <StatusBadge isOccupied={room.isOccupied} />
            </div>
          </div>
        </div>

        <div className="space-y-4 sm:space-y-6">
          {/* Room Information */}
          <InfoCard
            title="Rauminformationen"
            icon={
              <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
              </svg>
            }
          >
            <InfoItem label="Raumname" value={room.name} />
            <InfoItem label="Gebäude" value={room.building ?? "Nicht angegeben"} />
            <InfoItem label="Etage" value={`Etage ${room.floor}`} />
            <InfoItem label="Kategorie" value={
              <div className="flex items-center gap-2">
                <span
                  className="inline-block h-2.5 w-2.5 rounded-full"
                  style={{ backgroundColor: room.color }}
                ></span>
                <span>{room.category}</span>
              </div>
            } />
            <InfoItem label="Kapazität" value={`${room.capacity} Personen`} />
            <InfoItem label="Status" value={room.isOccupied ? "Belegt" : "Frei"} />
          </InfoCard>

          {/* Current Occupation */}
          {room.isOccupied && (
            <InfoCard
              title="Aktuelle Belegung"
              icon={
                <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
                </svg>
              }
            >
              {room.groupName && <InfoItem label="Gruppe" value={room.groupName} />}
              {room.activityName && <InfoItem label="Aktivität" value={room.activityName} />}
              {room.supervisorName && <InfoItem label="Aufsichtsperson" value={room.supervisorName} />}
              {room.studentCount !== undefined && (
                <div>
                  <p className="text-xs text-gray-500 mb-2">Belegung</p>
                  <div className="flex items-center justify-between mb-1">
                    <span className="text-sm text-gray-900 font-medium">
                      {room.studentCount} / {room.capacity} Personen
                    </span>
                    <span className="text-xs text-gray-500">
                      {Math.round((room.studentCount / room.capacity) * 100)}%
                    </span>
                  </div>
                  <div className="h-2 w-full overflow-hidden rounded-full bg-gray-200">
                    <div
                      className="h-full rounded-full transition-all duration-300"
                      style={{
                        width: `${Math.min((room.studentCount / room.capacity) * 100, 100)}%`,
                        backgroundColor: room.color
                      }}
                    ></div>
                  </div>
                </div>
              )}
            </InfoCard>
          )}

          {/* Room History */}
          <InfoCard
            title="Belegungshistorie"
            icon={
              <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            }
          >
            {groupedActivities.length === 0 ? (
              <div className="py-8 text-center text-gray-500">
                Keine Belegungshistorie verfügbar.
              </div>
            ) : (
              <div className="space-y-6">
                {groupedActivities.map(dateGroup => (
                  <div key={dateGroup.date}>
                    <h3 className="text-sm font-semibold text-gray-700 mb-3">
                      {dateGroup.entries[0]?.entryTimestamp ? formatDate(dateGroup.entries[0].entryTimestamp) : ''}
                    </h3>

                    <div className="space-y-3">
                      {dateGroup.entries.map(activity => {
                        const actualDuration = calculateDuration(
                          activity.entryTimestamp,
                          activity.exitTimestamp
                        );
                        const duration = activity.duration_minutes ?? actualDuration;
                        const categoryColor = activity.category
                          ? categoryColors[activity.category] ?? "#6B7280"
                          : "#6B7280";

                        return (
                          <div
                            key={activity.id}
                            className="bg-white border border-gray-100 rounded-lg p-4 hover:shadow-md transition-shadow"
                          >
                            {/* Color indicator */}
                            <div
                              className="h-1 rounded-full mb-3 -mx-4 -mt-4"
                              style={{ backgroundColor: categoryColor }}
                            ></div>

                            <div className="flex justify-between items-start mb-2">
                              <h4 className="font-medium text-gray-900">{activity.activityName}</h4>
                              <span className="text-xs text-gray-500">{formatDuration(duration)}</span>
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

                            <div className="mt-3 pt-3 border-t border-gray-100 flex justify-between text-xs text-gray-500">
                              <span>Beginn: {formatTime(activity.entryTimestamp)}</span>
                              {activity.exitTimestamp ? (
                                <span>Ende: {formatTime(activity.exitTimestamp)}</span>
                              ) : (
                                <span className="text-blue-600 font-medium">Laufend</span>
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
