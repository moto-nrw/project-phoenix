// app/statistics/page.tsx
"use client";

import { useState, useEffect } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { Alert } from "~/components/ui/alert";


// Type definitions for statistics
interface SchoolStats {
    totalStudents: number;
    presentStudents: number;
    absentStudents: number;
    schoolyard: number;
    bathroom: number;
    bus: number;
    activeGroups: number;
    totalCapacity: number;
    occupancyRate: number;
}

interface AttendanceData {
    date: string;
    present: number;
    absent: number;
}

interface RoomUtilization {
    room: string;
    usagePercent: number;
    category: string;
}

interface ActivityParticipation {
    activity: string;
    students: number;
    totalSlots: number;
    category: string;
}

interface FeedbackSummary {
    positive: number;
    neutral: number;
    negative: number;
    total: number;
}

export default function StatisticsPage() {
    const { data: session, status } = useSession({
        required: true,
        onUnauthenticated() {
            redirect("/");
        },
    });

    // State variables
    const [schoolStats, setSchoolStats] = useState<SchoolStats | null>(null);
    const [attendanceData, setAttendanceData] = useState<AttendanceData[]>([]);
    const [roomUtilization, setRoomUtilization] = useState<RoomUtilization[]>([]);
    const [activityParticipation, setActivityParticipation] = useState<ActivityParticipation[]>([]);
    const [feedbackSummary, setFeedbackSummary] = useState<FeedbackSummary | null>(null);
    const [timeRange, setTimeRange] = useState<string>("7days");
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    // Category-to-color mapping (from rooms page)
    const categoryColors: Record<string, string> = {
        "Gruppenraum": "#4F46E5", // Blue
        "Lernen": "#10B981",      // Green
        "Spielen": "#F59E0B",     // Orange
        "Bewegen/Ruhe": "#EC4899", // Pink
        "Hauswirtschaft": "#EF4444", // Red
        "Natur": "#22C55E",       // Green
        "Kreatives/Musik": "#8B5CF6", // Purple
        "NW/Technik": "#06B6D4",  // Teal
    };

    // Fetch statistics data
    useEffect(() => {
        setLoading(true);
        setError(null);

        // Simulate API request with timeout
        const timer = setTimeout(() => {
            try {
                // Mock school stats
                const mockSchoolStats: SchoolStats = {
                    totalStudents: 150,
                    presentStudents: 127,
                    absentStudents: 23,
                    schoolyard: 32,
                    bathroom: 8,
                    bus: 10,
                    activeGroups: 8,
                    totalCapacity: 180,
                    occupancyRate: 70.5,
                };

                // Mock attendance data for the last 7 days
                const mockAttendanceData: AttendanceData[] = [
                    { date: "2025-05-09", present: 130, absent: 20 },
                    { date: "2025-05-10", present: 125, absent: 25 },
                    { date: "2025-05-11", present: 128, absent: 22 },
                    { date: "2025-05-12", present: 131, absent: 19 },
                    { date: "2025-05-13", present: 124, absent: 26 },
                    { date: "2025-05-14", present: 127, absent: 23 },
                    { date: "2025-05-15", present: 129, absent: 21 },
                ];

                // Mock room utilization data
                const mockRoomUtilization: RoomUtilization[] = [
                    { room: "Klassenraum 101", usagePercent: 85, category: "Gruppenraum" },
                    { room: "Musiksaal", usagePercent: 60, category: "Kreatives/Musik" },
                    { room: "Computerraum 1", usagePercent: 75, category: "NW/Technik" },
                    { room: "Kunstraum", usagePercent: 55, category: "Kreatives/Musik" },
                    { room: "Turnhalle", usagePercent: 90, category: "Bewegen/Ruhe" },
                    { room: "Klassenraum 102", usagePercent: 70, category: "Gruppenraum" },
                    { room: "Klassenraum 201", usagePercent: 80, category: "Lernen" },
                    { room: "Bibliothek", usagePercent: 45, category: "Lernen" },
                ];

                // Mock activity participation data
                const mockActivityParticipation: ActivityParticipation[] = [
                    { activity: "Fußball AG", students: 12, totalSlots: 15, category: "Bewegen/Ruhe" },
                    { activity: "Coding Club", students: 8, totalSlots: 10, category: "NW/Technik" },
                    { activity: "Musikgruppe", students: 14, totalSlots: 15, category: "Kreatives/Musik" },
                    { activity: "Lesezirkel", students: 7, totalSlots: 10, category: "Lernen" },
                    { activity: "Kochgruppe", students: 9, totalSlots: 12, category: "Hauswirtschaft" },
                    { activity: "Naturerkundung", students: 11, totalSlots: 15, category: "Natur" },
                    { activity: "Schach AG", students: 6, totalSlots: 8, category: "Spielen" },
                    { activity: "Kunstwerkstatt", students: 10, totalSlots: 12, category: "Sonstiges" },
                ];

                // Mock feedback summary data
                const mockFeedbackSummary: FeedbackSummary = {
                    positive: 75,
                    neutral: 20,
                    negative: 5,
                    total: 100,
                };

                setSchoolStats(mockSchoolStats);
                setAttendanceData(mockAttendanceData);
                setRoomUtilization(mockRoomUtilization);
                setActivityParticipation(mockActivityParticipation);
                setFeedbackSummary(mockFeedbackSummary);
                setLoading(false);
            } catch (err) {
                console.error("Error fetching statistics:", err);
                setError("Fehler beim Laden der Statistikdaten.");
                setLoading(false);
            }
        }, 800);

        return () => clearTimeout(timer);
    }, [timeRange]);

    // Function to format date
    const formatDate = (dateStr: string): string => {
        const date = new Date(dateStr);
        return date.toLocaleDateString('de-DE', { weekday: 'short', day: 'numeric', month: 'numeric' });
    };

    if (status === "loading" || loading) {
        return (
            <div className="flex min-h-screen items-center justify-center">
                <div className="flex flex-col items-center">
                    <div className="h-12 w-12 rounded-full border-4 border-t-blue-500 border-b-blue-500 border-l-transparent border-r-transparent animate-spin"></div>
                    <p className="mt-4 text-gray-600">Statistiken werden geladen...</p>
                </div>
            </div>
        );
    }

    if (error) {
        return (
            <ResponsiveLayout userName={session?.user?.name ?? "Root"}>
                <div className="flex min-h-[80vh] flex-col items-center justify-center">
                    <Alert type="error" message={error} />
                </div>
            </ResponsiveLayout>
        );
    }

    return (
        <ResponsiveLayout userName={session?.user?.name ?? "Root"}>
            <div className="max-w-7xl mx-auto">
                {/* Page Title */}
                <h1 className="mb-8 text-4xl font-bold text-gray-900">Statistik & Auswertung</h1>

                        {/* Section 1: Aktuell */}
                        <section>
                            <h2 className="mb-6 text-2xl font-bold text-gray-800">Live Übersicht</h2>

                            {/* Key Metrics Overview */}
                            <div className="mb-8">
                                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4 xl:grid-cols-4">
                                    {/* Total Students Card */}
                                    <div className="overflow-hidden rounded-lg bg-white p-4 shadow-sm transition-all duration-200 hover:shadow-md">
                                        <div className="flex items-center">
                                            <div className="mr-4 rounded-full bg-blue-100 p-3">
                                                <svg className="h-6 w-6 text-blue-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
                                                </svg>
                                            </div>
                                            <div>
                                                <p className="text-sm font-medium text-gray-600">Gesamt Schüler</p>
                                                <p className="text-2xl font-bold text-gray-900">{schoolStats?.totalStudents}</p>
                                            </div>
                                        </div>
                                    </div>

                                    {/* Present Students Card */}
                                    <div className="overflow-hidden rounded-lg bg-white p-4 shadow-sm transition-all duration-200 hover:shadow-md">
                                        <div className="flex items-center">
                                            <div className="mr-4 rounded-full bg-green-100 p-3">
                                                <svg className="h-6 w-6 text-green-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                                                </svg>
                                            </div>
                                            <div>
                                                <p className="text-sm font-medium text-gray-600">Anwesend heute</p>
                                                <p className="text-2xl font-bold text-gray-900">{schoolStats?.presentStudents}</p>
                                            </div>
                                        </div>
                                    </div>

                                    {/* Occupancy Rate Card */}
                                    <div className="overflow-hidden rounded-lg bg-white p-4 shadow-sm transition-all duration-200 hover:shadow-md">
                                        <div className="flex items-center">
                                            <div className="mr-4 rounded-full bg-purple-100 p-3">
                                                <svg className="h-6 w-6 text-purple-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
                                                </svg>
                                            </div>
                                            <div>
                                                <p className="text-sm font-medium text-gray-600">Raumauslastung</p>
                                                <p className="text-2xl font-bold text-gray-900">{schoolStats?.occupancyRate.toFixed(1)}%</p>
                                            </div>
                                        </div>
                                    </div>

                                    {/* Active Groups Card */}
                                    <div className="overflow-hidden rounded-lg bg-white p-4 shadow-sm transition-all duration-200 hover:shadow-md">
                                        <div className="flex items-center">
                                            <div className="mr-4 rounded-full bg-amber-100 p-3">
                                                <svg className="h-6 w-6 text-amber-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
                                                </svg>
                                            </div>
                                            <div>
                                                <p className="text-sm font-medium text-gray-600">Aktive Gruppen</p>
                                                <p className="text-2xl font-bold text-gray-900">{schoolStats?.activeGroups}</p>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            </div>

                            {/* Location Distribution Card */}
                            <div className="mb-8 overflow-hidden rounded-xl bg-white p-6 shadow-md">
                                <h2 className="mb-4 text-xl font-bold text-gray-800">Standortverteilung der Schüler</h2>

                                <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-5">
                                    {/* In House Card */}
                                    <div className="rounded-lg bg-green-50 p-4 text-center">
                                        <div className="mx-auto mb-2 flex h-12 w-12 items-center justify-center rounded-full bg-green-100">
                                            <svg className="h-6 w-6 text-green-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
                                            </svg>
                                        </div>
                                        <p className="text-2xl font-bold text-green-800">{schoolStats?.presentStudents}</p>
                                        <p className="text-sm text-green-700">In Gruppenräumen</p>
                                    </div>

                                    {/* Schoolyard Card */}
                                    <div className="rounded-lg bg-blue-50 p-4 text-center">
                                        <div className="mx-auto mb-2 flex h-12 w-12 items-center justify-center rounded-full bg-blue-100">
                                            <svg className="h-6 w-6 text-blue-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 5l-4-4-4 4M8 9v10M16 19V9M12 1v18M3 19h18" />
                                            </svg>
                                        </div>
                                        <p className="text-2xl font-bold text-blue-800">{schoolStats?.schoolyard}</p>
                                        <p className="text-sm text-blue-700">Schulhof</p>
                                    </div>

                                    {/* Bathroom Card */}
                                    <div className="rounded-lg bg-yellow-50 p-4 text-center">
                                        <div className="mx-auto mb-2 flex h-12 w-12 items-center justify-center rounded-full bg-yellow-100">
                                            <span className="text-xl font-bold text-yellow-600">WC</span>
                                        </div>
                                        <p className="text-2xl font-bold text-yellow-800">{schoolStats?.bathroom}</p>
                                        <p className="text-sm text-yellow-700">Toilette</p>
                                    </div>

                                    {/* Bus/Home Card */}
                                    <div className="rounded-lg bg-purple-50 p-4 text-center">
                                        <div className="mx-auto mb-2 flex h-12 w-12 items-center justify-center rounded-full bg-purple-100">
                                            <svg className="h-6 w-6 text-purple-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14 5l7 7m0 0l-7 7m7-7H3" />
                                            </svg>
                                        </div>
                                        <p className="text-2xl font-bold text-purple-800">{schoolStats?.bus}</p>
                                        <p className="text-sm text-purple-700">Unterwegs</p>
                                    </div>

                                    {/* Absent Card */}
                                    <div className="rounded-lg bg-red-50 p-4 text-center">
                                        <div className="mx-auto mb-2 flex h-12 w-12 items-center justify-center rounded-full bg-red-100">
                                            <svg className="h-6 w-6 text-red-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6" />
                                            </svg>
                                        </div>
                                        <p className="text-2xl font-bold text-red-800">{schoolStats?.absentStudents}</p>
                                        <p className="text-sm text-red-700">Abwesend</p>
                                    </div>
                                </div>

                                <div className="mt-6 rounded-lg bg-blue-50 p-4">
                                    <div className="flex items-start">
                                        <svg className="mr-3 h-5 w-5 text-blue-500 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                                        </svg>
                                        <p className="text-sm text-blue-700">
                                            Diese Statistik zeigt die aktuelle Verteilung aller Schüler in Echtzeit. Die Daten werden minütlich aktualisiert und helfen, den Überblick über alle Schüler zu behalten.
                                        </p>
                                    </div>
                                </div>
                            </div>
                        </section>

                        {/* Section 2: Filter */}
                        <section>
                            <h2 className="mb-6 text-2xl font-bold text-gray-800">Historischer Verlauf</h2>

                            {/* Time Range Filter */}
                            <div className="mb-8">
                                <div className="bg-white rounded-lg shadow-sm p-4">
                                    <h2 className="text-lg font-medium text-gray-800 mb-3">Zeitraum auswählen</h2>
                                    <div className="flex flex-wrap gap-2">
                                        <button
                                            className={`px-4 py-2 rounded-lg transition-colors ${timeRange === "today" ? "bg-blue-500 text-white" : "bg-gray-100 hover:bg-gray-200 text-gray-700"}`}
                                            onClick={() => setTimeRange("today")}
                                        >
                                            Heute
                                        </button>
                                        <button
                                            className={`px-4 py-2 rounded-lg transition-colors ${timeRange === "week" ? "bg-blue-500 text-white" : "bg-gray-100 hover:bg-gray-200 text-gray-700"}`}
                                            onClick={() => setTimeRange("week")}
                                        >
                                            Diese Woche
                                        </button>
                                        <button
                                            className={`px-4 py-2 rounded-lg transition-colors ${timeRange === "7days" ? "bg-blue-500 text-white" : "bg-gray-100 hover:bg-gray-200 text-gray-700"}`}
                                            onClick={() => setTimeRange("7days")}
                                        >
                                            Letzte 7 Tage
                                        </button>
                                        <button
                                            className={`px-4 py-2 rounded-lg transition-colors ${timeRange === "month" ? "bg-blue-500 text-white" : "bg-gray-100 hover:bg-gray-200 text-gray-700"}`}
                                            onClick={() => setTimeRange("month")}
                                        >
                                            Diesen Monat
                                        </button>
                                        <button
                                            className={`px-4 py-2 rounded-lg transition-colors ${timeRange === "year" ? "bg-blue-500 text-white" : "bg-gray-100 hover:bg-gray-200 text-gray-700"}`}
                                            onClick={() => setTimeRange("year")}
                                        >
                                            Dieses Jahr
                                        </button>
                                    </div>
                                </div>
                            </div>

                            {/* Attendance Chart */}
                            <div className="mb-8 overflow-hidden rounded-xl bg-white p-6 shadow-md">
                                <h2 className="mb-4 text-xl font-bold text-gray-800">Anwesenheitsstatistik</h2>

                                <div className="h-64">
                                    {/* SVG Attendance Chart */}
                                    <svg width="100%" height="100%" viewBox="0 0 800 250">
                                        {/* X and Y Axes */}
                                        <line x1="50" y1="200" x2="750" y2="200" stroke="#E5E7EB" strokeWidth="2" />
                                        <line x1="50" y1="40" x2="50" y2="200" stroke="#E5E7EB" strokeWidth="2" />

                                        {/* Y-axis labels */}
                                        <text x="30" y="200" textAnchor="end" fill="#6B7280" fontSize="12">0</text>
                                        <text x="30" y="160" textAnchor="end" fill="#6B7280" fontSize="12">50</text>
                                        <text x="30" y="120" textAnchor="end" fill="#6B7280" fontSize="12">100</text>
                                        <text x="30" y="80" textAnchor="end" fill="#6B7280" fontSize="12">150</text>
                                        <text x="30" y="40" textAnchor="end" fill="#6B7280" fontSize="12">200</text>

                                        {/* Gridlines */}
                                        <line x1="50" y1="160" x2="750" y2="160" stroke="#E5E7EB" strokeWidth="1" strokeDasharray="5,5" />
                                        <line x1="50" y1="120" x2="750" y2="120" stroke="#E5E7EB" strokeWidth="1" strokeDasharray="5,5" />
                                        <line x1="50" y1="80" x2="750" y2="80" stroke="#E5E7EB" strokeWidth="1" strokeDasharray="5,5" />
                                        <line x1="50" y1="40" x2="750" y2="40" stroke="#E5E7EB" strokeWidth="1" strokeDasharray="5,5" />

                                        {/* Chart Title */}
                                        <text x="400" y="20" textAnchor="middle" fill="#111827" fontSize="14" fontWeight="bold">Anwesenheitstrend der letzten 7 Tage</text>

                                        {/* Data points and lines */}
                                        {attendanceData.map((entry, index) => {
                                            const x = 100 + index * 100;
                                            const presentY = 200 - (entry.present / 200 * 160);
                                            const absentY = 200 - (entry.absent / 200 * 160);

                                            return (
                                                <g key={index}>
                                                    {/* X-axis label */}
                                                    <text x={x} y="220" textAnchor="middle" fill="#6B7280" fontSize="12">{formatDate(entry.date)}</text>

                                                    {/* Present data points */}
                                                    <circle cx={x} cy={presentY} r="4" fill="#4F46E5" />
                                                    {index > 0 && attendanceData[index - 1]?.present !== undefined && (
                                                        <line
                                                            x1={100 + (index - 1) * 100}
                                                            y1={200 - ((attendanceData[index - 1]?.present ?? 0) / 200 * 160)}
                                                            x2={x}
                                                            y2={presentY}
                                                            stroke="#4F46E5"
                                                            strokeWidth="2"
                                                        />
                                                    )}

                                                    {/* Absent data points */}
                                                    <circle cx={x} cy={absentY} r="4" fill="#EF4444" />
                                                    {index > 0 && attendanceData[index - 1]?.absent !== undefined && (
                                                        <line
                                                            x1={100 + (index - 1) * 100}
                                                            y1={200 - ((attendanceData[index - 1]?.absent ?? 0) / 200 * 160)}
                                                            x2={x}
                                                            y2={absentY}
                                                            stroke="#EF4444"
                                                            strokeWidth="2"
                                                        />
                                                    )}
                                                </g>
                                            );
                                        })}

                                        {/* Legend - KORRIGIERT: Richtiges cy-Attribut statt y und besser positioniert */}
                                        <circle cx="600" cy="30" r="4" fill="#4F46E5" />
                                        <text x="610" y="34" fill="#111827" fontSize="12">Anwesend</text>
                                        <circle cx="680" cy="30" r="4" fill="#EF4444" />
                                        <text x="690" y="34" fill="#111827" fontSize="12">Abwesend</text>
                                    </svg>
                                </div>
                            </div>

                            {/* Room and Activity Stats */}
                            <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
                                {/* Room Utilization */}
                                <div className="overflow-hidden rounded-xl bg-white p-6 shadow-md">
                                    <h2 className="mb-4 text-xl font-bold text-gray-800">Raumauslastung</h2>

                                    <div className="space-y-4">
                                        {roomUtilization.map((room, index) => (
                                            <div key={index} className="relative">
                                                <div className="flex items-center justify-between mb-1">
                                                    <span className="font-medium text-gray-800">{room.room}</span>
                                                    <span className="text-sm text-gray-600">{room.usagePercent}%</span>
                                                </div>
                                                <div className="h-2.5 w-full overflow-hidden rounded-full bg-gray-200">
                                                    <div
                                                        className="h-full rounded-full bg-blue-500"
                                                        style={{
                                                            width: `${room.usagePercent}%`
                                                        }}
                                                    ></div>
                                                </div>
                                            </div>
                                        ))}
                                    </div>

                                    <div className="mt-6 flex justify-end">
                                        <button className="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50">
                                            Vollständige Analyse
                                        </button>
                                    </div>
                                </div>

                                {/* Activity Participation */}
                                <div className="overflow-hidden rounded-xl bg-white p-6 shadow-md">
                                    <h2 className="mb-4 text-xl font-bold text-gray-800">Aktivitäten-Teilnahme</h2>

                                    <div className="space-y-4">
                                        {activityParticipation.map((activity, index) => (
                                            <div key={index} className="relative">
                                                <div className="flex items-center justify-between mb-1">
                                                    <span className="font-medium text-gray-800">{activity.category}</span>
                                                </div>
                                                <div className="h-2.5 w-full overflow-hidden rounded-full bg-gray-200">
                                                    <div
                                                        className="h-full rounded-full"
                                                        style={{
                                                            width: `${(activity.students / activity.totalSlots) * 100}%`,
                                                            backgroundColor: categoryColors[activity.category] ?? '#6B7280',
                                                        }}
                                                    ></div>
                                                </div>
                                            </div>
                                        ))}
                                    </div>

                                    <div className="mt-6 flex justify-end">
                                        <button className="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50">
                                            Vollständige Übersicht
                                        </button>
                                    </div>
                                </div>
                            </div>

                            {/* Feedback Summary */}
                            <div className="mt-8 overflow-hidden rounded-xl bg-white p-6 shadow-md">
                                <h2 className="mb-4 text-xl font-bold text-gray-800">Feedback-Auswertung</h2>

                                <div className="grid grid-cols-1 gap-6 md:grid-cols-2">
                                    {/* Pie Chart */}
                                    <div className="flex items-center justify-center">
                                        <svg width="250" height="250" viewBox="0 0 250 250">
                                            <title>Feedback-Verteilung</title>
                                            {/* SVG Pie Chart */}
                                            {feedbackSummary && (
                                                <g transform="translate(125, 125)">
                                                    {/* Slices */}
                                                    {/* Positive Feedback (Green) */}
                                                    <path
                                                        d={`M 0 0 L ${Math.cos(0) * 100} ${Math.sin(0) * 100} A 100 100 0 ${(feedbackSummary.positive / feedbackSummary.total) * 360 > 180 ? 1 : 0} 1 ${Math.cos((feedbackSummary.positive / feedbackSummary.total) * Math.PI * 2) * 100} ${Math.sin((feedbackSummary.positive / feedbackSummary.total) * Math.PI * 2) * 100} Z`}
                                                        fill="#83CD2D"
                                                    />
                                                    {/* Neutral Feedback (Orange) */}
                                                    <path
                                                        d={`M 0 0 L ${Math.cos((feedbackSummary.positive / feedbackSummary.total) * Math.PI * 2) * 100} ${Math.sin((feedbackSummary.positive / feedbackSummary.total) * Math.PI * 2) * 100} A 100 100 0 ${(feedbackSummary.neutral / feedbackSummary.total) * 360 > 180 ? 1 : 0} 1 ${Math.cos(((feedbackSummary.positive + feedbackSummary.neutral) / feedbackSummary.total) * Math.PI * 2) * 100} ${Math.sin(((feedbackSummary.positive + feedbackSummary.neutral) / feedbackSummary.total) * Math.PI * 2) * 100} Z`}
                                                        fill="#F78C10"
                                                    />
                                                    {/* Negative Feedback (Red) */}
                                                    <path
                                                        d={`M 0 0 L ${Math.cos(((feedbackSummary.positive + feedbackSummary.neutral) / feedbackSummary.total) * Math.PI * 2) * 100} ${Math.sin(((feedbackSummary.positive + feedbackSummary.neutral) / feedbackSummary.total) * Math.PI * 2) * 100} A 100 100 0 ${(feedbackSummary.negative / feedbackSummary.total) * 360 > 180 ? 1 : 0} 1 ${Math.cos(0) * 100} ${Math.sin(0) * 100} Z`}
                                                        fill="#FF3130"
                                                    />
                                                    {/* Center hole */}
                                                    <circle cx="0" cy="0" r="60" fill="white" />
                                                    {/* Inner text */}
                                                    <text x="0" y="0" textAnchor="middle" dominantBaseline="middle" fontSize="24" fontWeight="bold" fill="#111827">
                                                        {feedbackSummary.total}
                                                    </text>
                                                    <text x="0" y="22" textAnchor="middle" dominantBaseline="middle" fontSize="14" fill="#6B7280">
                                                        Gesamt
                                                    </text>
                                                </g>
                                            )}
                                        </svg>
                                    </div>

                                    {/* Stats and Legend */}
                                    <div>
                                        <div className="mb-6 space-y-4">
                                            {/* Positive Feedback */}
                                            <div className="flex items-center">
                                                <div className="w-3 h-3 rounded-full bg-[#83CD2D] mr-2"></div>
                                                <div className="flex-1">
                                                    <div className="flex justify-between">
                                                        <span className="text-sm font-medium text-gray-700">Positiv</span>
                                                        <span className="text-sm text-gray-500">{feedbackSummary?.positive}%</span>
                                                    </div>
                                                    <div className="mt-1 h-1.5 w-full rounded-full bg-gray-200">
                                                        <div className="h-full rounded-full bg-[#83CD2D]" style={{ width: `${feedbackSummary?.positive}%` }}></div>
                                                    </div>
                                                </div>
                                            </div>

                                            {/* Neutral Feedback */}
                                            <div className="flex items-center">
                                                <div className="w-3 h-3 rounded-full bg-[#F78C10] mr-2"></div>
                                                <div className="flex-1">
                                                    <div className="flex justify-between">
                                                        <span className="text-sm font-medium text-gray-700">Neutral</span>
                                                        <span className="text-sm text-gray-500">{feedbackSummary?.neutral}%</span>
                                                    </div>
                                                    <div className="mt-1 h-1.5 w-full rounded-full bg-gray-200">
                                                        <div className="h-full rounded-full bg-[#F78C10]" style={{ width: `${feedbackSummary?.neutral}%` }}></div>
                                                    </div>
                                                </div>
                                            </div>

                                            {/* Negative Feedback */}
                                            <div className="flex items-center">
                                                <div className="w-3 h-3 rounded-full bg-[#FF3130] mr-2"></div>
                                                <div className="flex-1">
                                                    <div className="flex justify-between">
                                                        <span className="text-sm font-medium text-gray-700">Negativ</span>
                                                        <span className="text-sm text-gray-500">{feedbackSummary?.negative}%</span>
                                                    </div>
                                                    <div className="mt-1 h-1.5 w-full rounded-full bg-gray-200">
                                                        <div className="h-full rounded-full bg-[#FF3130]" style={{ width: `${feedbackSummary?.negative}%` }}></div>
                                                    </div>
                                                </div>
                                            </div>
                                        </div>

                                        <div className="border-t border-gray-200 pt-4">
                                            <p className="text-sm text-gray-600">
                                                Basierend auf {feedbackSummary?.total} Feedback-Einträgen im ausgewählten Zeitraum. Die Mehrheit der Rückmeldungen ist positiv, was auf eine hohe Zufriedenheit hindeutet.
                                            </p>
                                            <button className="mt-3 rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50">
                                                Detaillierte Analyse
                                            </button>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </section>

                        {/* Export Options */}
                        <div className="mt-8 flex justify-end space-x-4">
                            <button
                                className="flex items-center rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50"
                            >
                                <svg className="mr-2 h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
                                </svg>
                                Als PDF exportieren
                            </button>
                            <button
                                className="flex items-center rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50"
                            >
                                <svg className="mr-2 h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 17v-2m3 2v-4m3 4v-6m2 10H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                                </svg>
                                Als Excel exportieren
                            </button>
                        </div>
            </div>
        </ResponsiveLayout>
    );
}