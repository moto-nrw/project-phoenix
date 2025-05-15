"use client";

import { useState, useEffect } from "react";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { Header } from "~/components/dashboard/header";
import { Sidebar } from "~/components/dashboard/sidebar";
import { Alert } from "~/components/ui/alert";
import { BackgroundWrapper } from "~/components/background-wrapper";

// Room interface (from rooms page)
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
    category?: string; // Added category field
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

// Kategorie-zu-Farbe Mapping (from rooms page)
const categoryColors: Record<string, string> = {
    "Gruppenraum": "#4F46E5", // Blau für Gruppenraum
    "Lernen": "#10B981",      // Grün für Lernen
    "Spielen": "#F59E0B",     // Orange für Spielen
    "Bewegen/Ruhe": "#EC4899", // Pink für Bewegen/Ruhe
    "Hauswirtschaft": "#EF4444", // Rot für Hauswirtschaft
    "Natur": "#22C55E",       // Grün für Natur
    "Kreatives/Musik": "#8B5CF6", // Lila für Kreatives/Musik
    "NW/Technik": "#06B6D4",  // Türkis für NW/Technik
};

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

    // Fetch room data and room history
    useEffect(() => {
        setLoading(true);
        setError(null);

        // Simulate API request with timeout
        const timer = setTimeout(() => {
            try {
                // Mock room data
                const mockRoom: Room = {
                    id: roomId,
                    name: "Klassenraum 101",
                    building: "Hauptgebäude",
                    floor: 1,
                    capacity: 30,
                    category: "Gruppenraum",
                    isOccupied: true,
                    groupName: "Regenbogengruppe",
                    activityName: "Gruppenraum",
                    supervisorName: "Fr. Schmidt",
                    studentCount: 25
                };

                // Mock room history data - adding categories to match room categories
                const mockRoomHistory: RoomHistoryEntry[] = [
                    {
                        id: "1",
                        timestamp: "2025-05-14T08:00:00",
                        groupName: "Zebragruppe",
                        activityName: "Lesen",
                        category: "Lernen",
                        supervisorName: "Fr. Schmidt",
                        studentCount: 25,
                        entry_type: "entry",
                        duration_minutes: 45
                    },
                    {
                        id: "2",
                        timestamp: "2025-05-14T08:45:00",
                        groupName: "Zebragruppe",
                        activityName: "Schach",
                        category: "Lernen",
                        supervisorName: "Fr. Schmidt",
                        entry_type: "exit"
                    },
                    {
                        id: "3",
                        timestamp: "2025-05-14T09:00:00",
                        groupName: "Affengruppe",
                        activityName: "Strategiespiele",
                        category: "Lernen",
                        supervisorName: "Hr. Weber",
                        studentCount: 27,
                        entry_type: "entry",
                        duration_minutes: 45
                    },
                    {
                        id: "4",
                        timestamp: "2025-05-14T09:45:00",
                        groupName: "Affengruppe",
                        activityName: "Hausaufgabenbetreuung",
                        category: "Lernen",
                        supervisorName: "Hr. Weber",
                        entry_type: "exit"
                    },
                    {
                        id: "5",
                        timestamp: "2025-05-14T10:00:00",
                        groupName: "Wengerforpresidentgruppe",
                        activityName: "Hausaufgabenbetreuung",
                        category: "Lernen",
                        supervisorName: "Fr. Klein",
                        studentCount: 24,
                        entry_type: "entry",
                        duration_minutes: 45
                    },
                    {
                        id: "6",
                        timestamp: "2025-05-14T10:45:00",
                        groupName: "Wengerforpresidentgruppe",
                        activityName: "Hausaufgabenbetreuung",
                        category: "Lernen",
                        supervisorName: "Fr. Klein",
                        entry_type: "exit"
                    },
                    {
                        id: "7",
                        timestamp: "2025-05-14T11:00:00",
                        groupName: "Pandagruppe",
                        activityName: "Fahrzeugkunde",
                        category: "NW/Technik",
                        supervisorName: "Hr. Müller",
                        studentCount: 26,
                        entry_type: "entry",
                        duration_minutes: 45
                    },
                    {
                        id: "8",
                        timestamp: "2025-05-14T11:45:00",
                        groupName: "Pandagruppe",
                        activityName: "Fahrzeugkunde",
                        category: "NW/Technik",
                        supervisorName: "Hr. Müller",
                        entry_type: "exit"
                    },
                    {
                        id: "9",
                        timestamp: "2025-05-13T08:00:00",
                        groupName: "Regenbogengruppe",
                        activityName: "Hausaufgabenbetreuung",
                        category: "Lernen",
                        supervisorName: "Fr. Schmidt",
                        studentCount: 25,
                        entry_type: "entry",
                        duration_minutes: 45
                    },
                    {
                        id: "10",
                        timestamp: "2025-05-13T08:45:00",
                        groupName: "Regenbogengruppe",
                        activityName: "Hausaufgabenbetreuung",
                        category: "Lernen",
                        supervisorName: "Fr. Schmidt",
                        entry_type: "exit"
                    },
                    {
                        id: "11",
                        timestamp: "2025-05-13T09:00:00",
                        groupName: "Sonnengruppe",
                        activityName: "Malen",
                        category: "Kreatives/Musik",
                        supervisorName: "Hr. Bauer",
                        studentCount: 15,
                        entry_type: "entry",
                        duration_minutes: 90,
                    },
                    {
                        id: "12",
                        timestamp: "2025-05-13T10:30:00",
                        groupName: "Sonnengruppe",
                        activityName: "Malen",
                        category: "Kreatives/Musik",
                        supervisorName: "Hr. Bauer",
                        entry_type: "exit"
                    }
                ];

                setRoom(mockRoom);
                setRoomHistory(mockRoomHistory);
                setLoading(false);
            } catch (err) {
                console.error("Error fetching data:", err);
                setError("Fehler beim Laden der Daten.");
                setLoading(false);
            }
        }, 800);

        return () => clearTimeout(timer);
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

    if (error || !room) {
        return (
            <BackgroundWrapper>
                <div className="min-h-screen">
                    <Header userName="Benutzer" />
                    <div className="flex">
                        <Sidebar />
                        <main className="flex-1 p-8">
                            <div className="flex min-h-[80vh] flex-col items-center justify-center">
                                <Alert
                                    type="error"
                                    message={error ?? "Raum nicht gefunden"}
                                />
                                <button
                                    onClick={() => router.push(referrer)}
                                    className="mt-4 rounded bg-blue-100 px-4 py-2 text-blue-800 transition-colors hover:bg-blue-200"
                                >
                                    Zurück
                                </button>
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
                                                    const categoryColor = activity.category ? categoryColors[activity.category] : "#6B7280";

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
                                                                    {/* Das reason-Tag wurde entfernt */}
                                                                </div>

                                                                <div className="mt-3 text-sm text-gray-600">
                                                                    <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
                                                                        <div>
                                                                            <span className="font-medium">Aufsicht:</span> {activity.supervisorName}
                                                                        </div>
                                                                        <div>
                                                                            <span className="font-medium">Kategorie:</span> {activity.category}
                                                                        </div>
                                                                        {activity.studentCount && (
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