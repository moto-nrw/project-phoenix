"use client";

import { useState, useEffect } from "react";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { Header } from "~/components/dashboard/header";
import { Sidebar } from "~/components/dashboard/sidebar";
import { BackgroundWrapper } from "~/components/background-wrapper";
import { getSession } from "next-auth/react";

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
    "Gruppenraum": "#4F46E5", // Blau für Gruppenraum
    "Lernen": "#10B981",      // Grün für Lernen
    "Spielen": "#F59E0B",     // Orange für Spielen
    "Bewegen/Ruhe": "#EC4899", // Pink für Bewegen/Ruhe
    "Hauswirtschaft": "#EF4444", // Rot für Hauswirtschaft
    "Natur": "#22C55E",       // Grün für Natur
    "Kreatives/Musik": "#8B5CF6", // Lila für Kreatives/Musik
    "NW/Technik": "#06B6D4",  // Türkis für NW/Technik
    "Klassenzimmer": "#4F46E5", // Blau für Klassenzimmer (wie Gruppenraum)
};

// Helper function to convert Backend Room to Frontend Room
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

// Helper function to convert Backend History Entry to Frontend History Entry
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

export default function RoomDetailPage() {
    const router = useRouter();
    const params = useParams();
    const searchParams = useSearchParams();
    const roomId = params.id as string;
    const referrer = searchParams.get("from") ?? "/rooms";

    const [room, setRoom] = useState<Room | null>(null);
    const [roomHistory, setRoomHistory] = useState<RoomHistoryEntry[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [errorStatus, setErrorStatus] = useState<number | null>(null);

    // Fetch room data and room history
    useEffect(() => {
        const fetchRoomData = async () => {
            setLoading(true);
            setError(null);
            setErrorStatus(null);

            try {
                // Get user session
                const session = await getSession();
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
                    const statusCode = roomResponse.status;
                    setErrorStatus(statusCode);

                    if (statusCode === 404) {
                        throw new Error(`Der Raum mit der ID ${roomId} wurde nicht gefunden.`);
                    } else if (statusCode === 401 || statusCode === 403) {
                        throw new Error("Sie haben keine Berechtigung, diesen Raum anzuzeigen.");
                    } else {
                        const errorText = await roomResponse.text();
                        throw new Error(`Fehler beim Abrufen des Raums: ${errorText || statusCode}`);
                    }
                }

                const roomData = await roomResponse.json() as BackendRoom;
                const frontendRoom = mapBackendToFrontendRoom(roomData);
                setRoom(frontendRoom);

                // Fetch room history data
                const historyResponse = await fetch(`/api/rooms/${roomId}/history`, {
                    credentials: "include",
                    headers: {
                        "Content-Type": "application/json",
                        ...authHeaders
                    }
                });

                if (!historyResponse.ok) {
                    const statusCode = historyResponse.status;
                    console.warn(`Warning: Failed to fetch room history: ${statusCode}`);
                    // We'll continue even if history fails - just won't show history
                }
                else {
                    // Parse history data, handling different response formats
                    const historyResponseData = await historyResponse.json() as BackendRoomHistoryEntry[] | { data: BackendRoomHistoryEntry[] };

                    // Handle possible response formats (direct array or data property)
                    const backendHistoryEntries = Array.isArray(historyResponseData)
                        ? historyResponseData
                        : (historyResponseData?.data && Array.isArray(historyResponseData.data))
                            ? historyResponseData.data
                            : [];

                    // Convert backend history entries to frontend format
                    const frontendHistoryEntries = backendHistoryEntries.map(
                        (entry: BackendRoomHistoryEntry) => mapBackendToFrontendHistoryEntry(entry)
                    );

                    setRoomHistory(frontendHistoryEntries);
                }
            } catch (err) {
                console.error("Error fetching data:", err);

                // Extract error message
                const errorMessage = err instanceof Error
                    ? err.message
                    : "Ein unbekannter Fehler ist aufgetreten.";

                setError(errorMessage);
            } finally {
                setLoading(false);
            }
        };

        void fetchRoomData();
    }, [roomId]);

    // Format date for display
    const formatDate = (dateString: string): string => {
        const date = new Date(dateString);
        return date.toLocaleDateString('de-DE', {
            weekday: 'long',
            year: 'numeric',
            month: 'long',
            day: 'numeric'
        });
    };

    // Format time for display
    const formatTime = (dateString: string): string => {
        const date = new Date(dateString);
        return date.toLocaleTimeString('de-DE', {
            hour: '2-digit',
            minute: '2-digit'
        });
    };

    // Calculate duration between two timestamps
    const calculateDuration = (entry: string, exit: string | null): number | null => {
        if (!exit) return null;

        const entryTime = new Date(entry);
        const exitTime = new Date(exit);
        const durationMs = exitTime.getTime() - entryTime.getTime();
        return Math.round(durationMs / 60000); // Minutes
    };

    // Format duration in hours and minutes
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

    // Group room history entries into activities (entry + exit pairs)
    const groupHistoryByActivity = (history: RoomHistoryEntry[]): Activity[] => {
        const grouped: Activity[] = [];
        const entriesMap: Record<string, RoomHistoryEntry> = {};

        // Sort entries by timestamp
        const sortedHistory = [...history].sort(
            (a, b) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime()
        );

        // First collect all entry records
        sortedHistory.forEach(entry => {
            if (entry.entry_type === "entry") {
                const key = `${entry.groupName}-${entry.activityName}`;
                entriesMap[key] = entry;
            }
        });

        // Then find matching exit for each entry
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

                    // Remove entry so it's not used multiple times
                    delete entriesMap[key];
                }
            }
        });

        // Add remaining active entries (without exit)
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

    // Group activities by date
    const groupByDate = (activities: Activity[]): DateGroup[] => {
        const groups: Record<string, Activity[]> = {};

        activities.forEach(activity => {
            const date = new Date(activity.entryTimestamp).toLocaleDateString('de-DE');
            groups[date] ??= [];
            groups[date].push(activity);
        });

        // Sort dates in descending order (newest first)
        return Object.keys(groups)
            .sort((a, b) => new Date(b).getTime() - new Date(a).getTime())
            .map(date => ({
                date,
                entries: (groups[date] ?? []).sort((a, b) =>
                    new Date(a.entryTimestamp).getTime() - new Date(b.entryTimestamp).getTime()
                )
            }));
    };

    // Process room history data
    const activities = groupHistoryByActivity(roomHistory);
    const groupedActivities = groupByDate(activities);

    // Render loading state
    if (loading) {
        return (
            <BackgroundWrapper>
                <div className="min-h-screen">
                    <Header userName="Benutzer" />
                    <div className="flex">
                        <Sidebar />
                        <main className="flex-1 p-8">
                            <div className="flex min-h-[80vh] items-center justify-center">
                                <div className="flex flex-col items-center gap-4">
                                    <div className="h-12 w-12 animate-spin rounded-full border-b-2 border-t-2 border-blue-500"></div>
                                    <p className="text-gray-600">Daten werden geladen...</p>
                                </div>
                            </div>
                        </main>
                    </div>
                </div>
            </BackgroundWrapper>
        );
    }

    // Render error state
    if (error || !room) {
        // Get a user-friendly error message
        let errorTitle = "Fehler";
        let errorMessage = error ?? "Der Raum konnte nicht geladen werden.";

        if (errorStatus === 404 || error?.includes("nicht gefunden")) {
            errorTitle = "Raum nicht gefunden";
            errorMessage = `Der Raum mit der ID ${roomId} existiert nicht oder wurde gelöscht.`;
        } else if (errorStatus === 401 || errorStatus === 403) {
            errorTitle = "Zugriff verweigert";
            errorMessage = "Sie haben keine Berechtigung, diesen Raum anzuzeigen.";
        } else if (errorStatus === 500) {
            errorTitle = "Serverfehler";
            errorMessage = "Es ist ein interner Serverfehler aufgetreten. Bitte versuchen Sie es später erneut.";
        }

        return (
            <BackgroundWrapper>
                <div className="min-h-screen">
                    <Header userName="Benutzer" />
                    <div className="flex">
                        <Sidebar />
                        <main className="flex-1 p-8">
                            <div className="flex min-h-[50vh] flex-col items-center justify-center">
                                <div className="w-full max-w-lg mx-auto p-6 bg-white rounded-lg shadow-sm">
                                    <div className="flex items-center justify-center mb-6">
                                        <div className="h-12 w-12 rounded-full bg-red-100 flex items-center justify-center">
                                            <svg
                                                xmlns="http://www.w3.org/2000/svg"
                                                className="h-6 w-6 text-red-600"
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
                                        </div>
                                    </div>
                                    <h2 className="text-xl font-semibold text-center text-gray-800 mb-2">
                                        {errorTitle}
                                    </h2>
                                    <p className="text-center text-gray-600 mb-6">
                                        {errorMessage}
                                    </p>
                                    <div className="flex justify-center">
                                        <button
                                            onClick={() => router.push(referrer)}
                                            className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white font-medium rounded-md transition-colors"
                                        >
                                            Zurück zur Raumübersicht
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </main>
                    </div>
                </div>
            </BackgroundWrapper>
        );
    }

    return (
        <BackgroundWrapper>
            <div className="min-h-screen">
                {/* Header */}
                <Header userName="Benutzer" />

                <div className="flex">
                    {/* Sidebar */}
                    <Sidebar />

                    {/* Main Content */}
                    <main className="flex-1 p-8">
                        <div className="mx-auto max-w-7xl">
                            {/* Back Button */}
                            <div className="mb-6">
                                <button
                                    onClick={() => router.push(referrer)}
                                    className="flex items-center text-gray-600 hover:text-blue-600 transition-colors"
                                >
                                    <svg
                                        xmlns="http://www.w3.org/2000/svg"
                                        className="h-5 w-5 mr-1"
                                        fill="none"
                                        viewBox="0 0 24 24"
                                        stroke="currentColor"
                                    >
                                        <path
                                            strokeLinecap="round"
                                            strokeLinejoin="round"
                                            strokeWidth={2}
                                            d="M10 19l-7-7m0 0l7-7m-7 7h18"
                                        />
                                    </svg>
                                    Zurück zur Raumübersicht
                                </button>
                            </div>

                            {/* Room Header with Status - Using blue gradient style from student view */}
                            <div className="relative mb-8 overflow-hidden rounded-xl bg-gradient-to-r from-teal-500 to-blue-600 p-6 text-white shadow-md">
                                <div className="flex items-center">
                                    <div className="mr-6 flex h-24 w-24 items-center justify-center rounded-full bg-white/30 text-4xl font-bold">
                                        {room.name.charAt(0)}
                                    </div>
                                    <div>
                                        <h1 className="text-3xl font-bold">{room.name}</h1>
                                        <div className="flex items-center mt-1">
                                            <span className="opacity-90">Gebäude: {room.building ?? "Unbekannt"}</span>
                                            <span className="mx-2">•</span>
                                            <span className="opacity-90">Etage: {room.floor}</span>
                                        </div>
                                        <div className="mt-3 flex items-center">
                                            <span className="text-white font-medium mr-2">Status:</span>
                                            <div className={`rounded-full px-3 py-1 ${
                                                room.isOccupied ? "bg-red-100 text-red-800" : "bg-green-100 text-green-800"
                                            } font-medium flex items-center`}>
                                                <span className={`mr-1.5 inline-block h-2.5 w-2.5 rounded-full ${
                                                    room.isOccupied ? "bg-red-500" : "bg-green-500"
                                                }`}></span>
                                                {room.isOccupied ? "Belegt" : "Frei"}
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            </div>

                            {/* Room Details */}
                            <div className="grid grid-cols-1 gap-6 md:grid-cols-2 mb-8">
                                {/* General Information Card */}
                                <div className="bg-white rounded-lg shadow-sm p-6">
                                    <h2 className="text-xl font-semibold mb-4 text-gray-800">Allgemeine Informationen</h2>
                                    <div className="space-y-3">
                                        <div className="flex justify-between">
                                            <span className="text-gray-600">Name:</span>
                                            <span className="font-medium text-gray-900">{room.name}</span>
                                        </div>
                                        <div className="flex justify-between">
                                            <span className="text-gray-600">Gebäude:</span>
                                            <span className="font-medium text-gray-900">{room.building ?? "Nicht angegeben"}</span>
                                        </div>
                                        <div className="flex justify-between">
                                            <span className="text-gray-600">Etage:</span>
                                            <span className="font-medium text-gray-900">{room.floor}</span>
                                        </div>
                                        <div className="flex justify-between">
                                            <span className="text-gray-600">Kapazität:</span>
                                            <span className="font-medium text-gray-900">{room.capacity} Personen</span>
                                        </div>
                                        <div className="flex justify-between">
                                            <span className="text-gray-600">Status:</span>
                                            <span className={`font-medium ${room.isOccupied ? "text-red-600" : "text-green-600"}`}>
                                                {room.isOccupied ? "Belegt" : "Frei"}
                                            </span>
                                        </div>
                                    </div>
                                </div>

                                {/* Current Occupation Card */}
                                <div className="bg-white rounded-lg shadow-sm p-6">
                                    <h2 className="text-xl font-semibold mb-4 text-gray-800">Aktuelle Belegung</h2>
                                    {room.isOccupied ? (
                                        <div className="space-y-3">
                                            {room.groupName && (
                                                <div className="flex justify-between">
                                                    <span className="text-gray-600">Gruppe:</span>
                                                    <span className="font-medium text-gray-900">{room.groupName}</span>
                                                </div>
                                            )}
                                            {room.activityName && (
                                                <div className="flex justify-between">
                                                    <span className="text-gray-600">Aktivität:</span>
                                                    <span className="font-medium text-gray-900">{room.activityName}</span>
                                                </div>
                                            )}
                                            {room.supervisorName && (
                                                <div className="flex justify-between">
                                                    <span className="text-gray-600">Aufsichtsperson:</span>
                                                    <span className="font-medium text-gray-900">{room.supervisorName}</span>
                                                </div>
                                            )}
                                            {room.studentCount !== undefined && (
                                                <div className="pt-2">
                                                    <div className="flex justify-between mb-1">
                                                        <span className="text-gray-600">Belegung:</span>
                                                        <span className="font-medium text-gray-900">
                                                            {room.studentCount} / {room.capacity} Personen
                                                        </span>
                                                    </div>
                                                    <div className="h-2.5 w-full overflow-hidden rounded-full bg-gray-200">
                                                        <div
                                                            className="h-full rounded-full bg-blue-600"
                                                            style={{
                                                                width: `${Math.min((room.studentCount / room.capacity) * 100, 100)}%`
                                                            }}
                                                        ></div>
                                                    </div>
                                                </div>
                                            )}
                                        </div>
                                    ) : (
                                        <div className="flex items-center justify-center h-32 text-gray-500">
                                            Dieser Raum ist aktuell nicht belegt.
                                        </div>
                                    )}
                                </div>
                            </div>

                            {/* Room History */}
                            <div className="bg-white rounded-lg shadow-sm p-6">
                                <h2 className="text-2xl font-medium text-gray-800 mb-6">Belegungshistorie</h2>

                                {groupedActivities.length === 0 ? (
                                    <div className="bg-white rounded-lg p-8 text-center">
                                        <p className="text-gray-500">Keine Belegungshistorie verfügbar.</p>
                                    </div>
                                ) : (
                                    groupedActivities.map(dateGroup => (
                                        <div key={dateGroup.date} className="mb-8">
                                            <div className="flex items-center mb-4">
                                                <div className="h-6 w-1 bg-blue-600 rounded mr-3"></div>
                                                <h3 className="text-lg font-medium text-blue-800">
                                                    {dateGroup.entries[0]?.entryTimestamp ? formatDate(dateGroup.entries[0].entryTimestamp) : ''}
                                                </h3>
                                            </div>

                                            <div className="space-y-4">
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
                                                            className="bg-white border border-gray-100 rounded-lg shadow-sm overflow-hidden"
                                                        >
                                                            <div
                                                                className="h-1.5"
                                                                style={{ backgroundColor: categoryColor }}
                                                            ></div>

                                                            <div className="p-4">
                                                                <div className="flex justify-between">
                                                                    <h4 className="text-lg font-medium text-gray-800">{activity.activityName}</h4>
                                                                    <div className="flex items-center">
                                                                        <span className="text-sm text-gray-500 mr-2">
                                                                            Dauer: {formatDuration(duration)}
                                                                        </span>
                                                                        <span
                                                                            className="inline-block h-3 w-3 rounded-full"
                                                                            style={{ backgroundColor: categoryColor }}
                                                                        ></span>
                                                                    </div>
                                                                </div>

                                                                <div className="mt-2">
                                                                    {activity.activityName === "Hausaufgabenbetreuung" && activity.groupName && (
                                                                        <span className="inline-flex items-center rounded-full bg-blue-100 px-2.5 py-0.5 text-xs font-medium text-blue-800 mr-2">
                                                                            {activity.groupName}
                                                                        </span>
                                                                    )}
                                                                </div>

                                                                <div className="mt-3 text-sm text-gray-600">
                                                                    <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
                                                                        <div>
                                                                            <span className="font-medium">Aufsicht:</span> {activity.supervisorName}
                                                                        </div>
                                                                        <div>
                                                                            <span className="font-medium">Kategorie:</span> {activity.category}
                                                                        </div>
                                                                        {activity.studentCount !== undefined && (
                                                                            <div>
                                                                                <span className="font-medium">Teilnehmer:</span> {activity.studentCount}
                                                                            </div>
                                                                        )}
                                                                    </div>
                                                                </div>

                                                                <div className="mt-4 flex items-center justify-between border-t border-gray-200 pt-3">
                                                                    <div className="flex items-center text-green-600">
                                                                        <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 9l3 3m0 0l-3 3m3-3H8m13 0a9 9 0 11-18 0 9 9 0 0118 0z" />
                                                                        </svg>
                                                                        <span className="text-sm">Beginn: {formatTime(activity.entryTimestamp)}</span>
                                                                    </div>

                                                                    {activity.exitTimestamp ? (
                                                                        <div className="flex items-center text-red-600">
                                                                            <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
                                                                            </svg>
                                                                            <span className="text-sm">Ende: {formatTime(activity.exitTimestamp)}</span>
                                                                        </div>
                                                                    ) : (
                                                                        <div className="flex items-center text-blue-600">
                                                                            <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                                                                            </svg>
                                                                            <span className="text-sm">Laufend</span>
                                                                        </div>
                                                                    )}
                                                                </div>
                                                            </div>
                                                        </div>
                                                    );
                                                })}
                                            </div>
                                        </div>
                                    ))
                                )}
                            </div>
                        </div>
                    </main>
                </div>
            </div>
        </BackgroundWrapper>
    );
}