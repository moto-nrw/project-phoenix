"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { ResponsiveLayout } from "@/components/dashboard";
import { Alert } from "~/components/ui/alert";
import { useSession } from "next-auth/react";

// Student type (reused from student page)
interface Student {
    id: string;
    first_name: string;
    second_name: string;
    name?: string;
    school_class: string;
    group_id: string;
    group_name?: string;
    in_house: boolean;
    wc: boolean;
    school_yard: boolean;
    bus: boolean;
    current_room?: string;
    guardian_name: string;
    guardian_contact: string;
    guardian_phone?: string;
    birthday?: string;
    notes?: string;
    buskind?: boolean;
    attendance_rate?: number;
}

// Mensa History interface
interface MensaEntry {
    id: string;
    date: string;
    has_eaten: boolean;
    feedback_type?: "positive" | "neutral" | "negative";
    comment?: string;
    meal_name?: string;
}

export default function StudentMensaHistoryPage() {
    const router = useRouter();
    const params = useParams();
    const searchParams = useSearchParams();
    const studentId = params.id as string;
    const referrer = searchParams.get("from") ?? "/students/search";
    const { } = useSession();

    const [student, setStudent] = useState<Student | null>(null);
    const [mensaHistory, setMensaHistory] = useState<MensaEntry[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [timeRange, setTimeRange] = useState<string>("7days"); // Default to last 7 days

    // Fetch student data and mensa history
    useEffect(() => {
        setLoading(true);
        setError(null);

        // Simulate API request with timeout
        const timer = setTimeout(() => {
            try {
                // Mock student data (same as in student detail page)
                const mockStudent: Student = {
                    id: studentId,
                    first_name: "Emma",
                    second_name: "Müller",
                    name: "Emma Müller",
                    school_class: "3b",
                    group_id: "g3",
                    group_name: "Eulen",
                    in_house: true,
                    wc: false,
                    school_yard: false,
                    bus: false,
                    current_room: "Raum 1.2",
                    guardian_name: "Maria Müller",
                    guardian_contact: "muellers@example.com",
                    guardian_phone: "+49 176 12345678",
                    birthday: "2016-06-15",
                    notes: "Nimmt an der Musik-AG teil. Liebt Kunst und Lesen.",
                    buskind: true,
                    attendance_rate: 92.5
                };

                // Mock mensa history data
                const mockMensaHistory: MensaEntry[] = [
                    {
                        id: "1",
                        date: "2025-05-14T12:00:00",
                        has_eaten: true,
                        feedback_type: "positive",
                        comment: "Hat alles aufgegessen und mochte besonders das Gemüse",
                        meal_name: "Nudeln mit Tomatensoße und Gemüse"
                    },
                    {
                        id: "2",
                        date: "2025-05-13T12:00:00",
                        has_eaten: true,
                        feedback_type: "neutral",
                        comment: "Hat etwa die Hälfte gegessen",
                        meal_name: "Fisch mit Kartoffeln"
                    },
                    {
                        id: "3",
                        date: "2025-05-12T12:00:00",
                        has_eaten: true,
                        feedback_type: "negative",
                        comment: "Hat kaum etwas gegessen, mochte den Geschmack nicht",
                        meal_name: "Gemüseeintopf mit Brot"
                    },
                    {
                        id: "4",
                        date: "2025-05-11T12:00:00",
                        has_eaten: false,
                        comment: "War an diesem Tag zuhause"
                    },
                    {
                        id: "5",
                        date: "2025-05-10T12:00:00",
                        has_eaten: true,
                        feedback_type: "positive",
                        comment: "Hat alles sehr gerne gegessen",
                        meal_name: "Hähnchen mit Reis und Soße"
                    },
                    {
                        id: "6",
                        date: "2025-05-09T12:00:00",
                        has_eaten: true,
                        feedback_type: "neutral",
                        comment: "Hat nur den Nachtisch komplett gegessen",
                        meal_name: "Kartoffelauflauf mit Salat"
                    },
                    {
                        id: "7",
                        date: "2025-05-08T12:00:00",
                        has_eaten: true,
                        feedback_type: "positive",
                        comment: "Hat sich sogar Nachschlag geholt",
                        meal_name: "Pizza mit Salat"
                    }
                ];

                setStudent(mockStudent);
                setMensaHistory(mockMensaHistory);
                setLoading(false);
            } catch (err) {
                console.error("Error fetching data:", err);
                setError("Fehler beim Laden der Daten.");
                setLoading(false);
            }
        }, 800);

        return () => clearTimeout(timer);
    }, [studentId, timeRange]);

    // Time range filtering implementation
    const getFilteredMensaHistory = (): MensaEntry[] => {
        // Wenn "all" ausgewählt ist, geben wir die gesamte Historie zurück
        if (timeRange === "all") {
            return mensaHistory;
        }

        const now = new Date();
        let startDate: Date;

        switch (timeRange) {
            case "today":
                // Get entries from today
                startDate = new Date(now.getFullYear(), now.getMonth(), now.getDate());
                break;
            case "week":
                // Get entries from this week (starting on Monday)
                const dayOfWeek = now.getDay(); // 0 is Sunday, 1 is Monday, etc.
                const daysFromMonday = dayOfWeek === 0 ? 6 : dayOfWeek - 1; // Adjust for Sunday
                startDate = new Date(now.getFullYear(), now.getMonth(), now.getDate() - daysFromMonday);
                break;
            case "7days":
                // Get entries from the last 7 days
                startDate = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 6);
                break;
            case "month":
                // Get entries from this month
                startDate = new Date(now.getFullYear(), now.getMonth(), 1);
                break;
            default:
                // Default to 7 days
                startDate = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 6);
        }

        // Set to start of day to include all entries for startDate
        startDate.setHours(0, 0, 0, 0);

        return mensaHistory.filter(entry => {
            const entryDate = new Date(entry.date);
            return entryDate >= startDate && entryDate <= now;
        });
    };

    // Apply filtering
    const filteredMensaHistory = getFilteredMensaHistory();

    // Get year from class
    const getYear = (schoolClass: string): number => {
        const yearMatch = /^(\d)/.exec(schoolClass);
        return yearMatch?.[1] ? parseInt(yearMatch[1], 10) : 0;
    };

    // Determine color for year indicator
    const getYearColor = (year: number): string => {
        switch (year) {
            case 1: return "bg-blue-500";
            case 2: return "bg-green-500";
            case 3: return "bg-yellow-500";
            case 4: return "bg-purple-500";
            default: return "bg-gray-400";
        }
    };

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

    // Group mensa history by date
    const groupedMensaHistory = filteredMensaHistory.reduce((groups, entry) => {
        const date = new Date(entry.date).toLocaleDateString('de-DE');
        groups[date] ??= [];
        groups[date].push(entry);
        return groups;
    }, {} as Record<string, MensaEntry[]>);

    // Sort dates in descending order (most recent first)
    const sortedDates = Object.keys(groupedMensaHistory).sort((a, b) => {
        return new Date(b).getTime() - new Date(a).getTime();
    });

    // Render the appropriate emoji based on feedback type
    const renderFeedbackEmoji = (type?: string) => {
        switch (type) {
            case "positive":
                return (
                    <div className="text-2xl bg-green-100 rounded-full h-10 w-10 flex items-center justify-center text-green-500">
                        😊
                    </div>
                );
            case "neutral":
                return (
                    <div className="text-2xl bg-yellow-100 rounded-full h-10 w-10 flex items-center justify-center text-yellow-500">
                        😐
                    </div>
                );
            case "negative":
                return (
                    <div className="text-2xl bg-red-100 rounded-full h-10 w-10 flex items-center justify-center text-red-500">
                        😔
                    </div>
                );
            default:
                return (
                    <div className="text-2xl bg-gray-100 rounded-full h-10 w-10 flex items-center justify-center text-gray-500">
                        ❌
                    </div>
                );
        }
    };

    const year = student ? getYear(student.school_class) : 0;
    const yearColor = getYearColor(year);

    // Count mensa statistics
    const totalDays = filteredMensaHistory.length;
    const eatenDays = filteredMensaHistory.filter(entry => entry.has_eaten).length;
    const notEatenDays = totalDays - eatenDays;

    // Calculate percentages
    const eatenPercentage = totalDays > 0 ? Math.round((eatenDays / totalDays) * 100) : 0;
    const notEatenPercentage = totalDays > 0 ? Math.round((notEatenDays / totalDays) * 100) : 0;

    // Count feedback types
    const positiveFeedback = filteredMensaHistory.filter(entry => entry.feedback_type === "positive").length;
    const neutralFeedback = filteredMensaHistory.filter(entry => entry.feedback_type === "neutral").length;
    const negativeFeedback = filteredMensaHistory.filter(entry => entry.feedback_type === "negative").length;

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

    if (error || !student) {
        return (
            <ResponsiveLayout>
                <div className="flex min-h-[80vh] flex-col items-center justify-center">
                    <Alert
                        type="error"
                        message={error ?? "Schüler nicht gefunden"}
                    />
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
            <div className="max-w-7xl mx-auto">
                            {/* Back Button */}
                            <div className="mb-6">
                                <button
                                    onClick={() => router.push(`/students/${studentId}?from=${referrer}`)}
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
                                    Zurück zum Schülerprofil
                                </button>
                            </div>

                            {/* Student Profile Header with Status */}
                            <div className="relative mb-8 overflow-hidden rounded-xl bg-gradient-to-r from-teal-500 to-blue-600 p-6 text-white shadow-md">
                                <div className="flex items-center">
                                    <div className="mr-6 flex h-24 w-24 items-center justify-center rounded-full bg-white/30 text-4xl font-bold">
                                        {student.first_name[0]}{student.second_name[0]}
                                    </div>
                                    <div>
                                        <h1 className="text-3xl font-bold">{student.name}</h1>
                                        <div className="flex items-center mt-1">
                                            <span className="opacity-90">Klasse {student.school_class}</span>
                                            <span className={`ml-2 inline-block h-3 w-3 rounded-full ${yearColor}`} title={`Jahrgang ${year}`}></span>
                                            <span className="mx-2">•</span>
                                            <span className="opacity-90">Gruppe: {student.group_name}</span>
                                        </div>
                                        <div className="mt-4">
                                            <h2 className="text-2xl font-semibold text-white">Mensaverlauf</h2>
                                        </div>
                                    </div>
                                </div>
                            </div>

                            {/* Filter Controls and Mensa Overview */}
                            <div className="mb-8">
                                <div className="bg-white rounded-lg shadow-sm p-4">
                                    <div className="flex flex-col md:flex-row md:justify-between">
                                        {/* Time Range Filter */}
                                        <div className="mb-4 md:mb-0">
                                            <h2 className="text-lg font-medium text-gray-800 mb-3">Filter</h2>
                                            <div className="flex flex-wrap gap-2">
                                                <button
                                                    className={`px-4 py-2 rounded-lg transition-colors ${timeRange === "all" ? "bg-blue-500 text-white" : "bg-gray-100 hover:bg-gray-200 text-gray-700"}`}
                                                    onClick={() => setTimeRange("all")}
                                                >
                                                    Alle
                                                </button>
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
                                            </div>
                                        </div>

                                        {/* Mensa Overview - now placed beside the filter */}
                                        <div>
                                            <h2 className="text-lg font-medium text-gray-800 mb-3">Mensa-Übersicht</h2>
                                            <div className="flex gap-4">
                                                {/* Eaten Days */}
                                                <div className="bg-green-50 border border-green-100 rounded-lg p-3">
                                                    <div className="flex items-center">
                                                        <div className="text-2xl mr-2">🍽️</div>
                                                        <div>
                                                            <h3 className="text-sm font-medium text-green-800">Gegessen</h3>
                                                            <p className="text-xl font-bold text-green-600">
                                                                {eatenDays} <span className="text-sm font-normal">({eatenPercentage}%)</span>
                                                            </p>
                                                        </div>
                                                    </div>
                                                </div>

                                                {/* Not Eaten Days */}
                                                <div className="bg-gray-50 border border-gray-100 rounded-lg p-3">
                                                    <div className="flex items-center">
                                                        <div className="text-2xl mr-2">❌</div>
                                                        <div>
                                                            <h3 className="text-sm font-medium text-gray-800">Nicht gegessen</h3>
                                                            <p className="text-xl font-bold text-gray-600">
                                                                {notEatenDays} <span className="text-sm font-normal">({notEatenPercentage}%)</span>
                                                            </p>
                                                        </div>
                                                    </div>
                                                </div>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            </div>

                            {/* Feedback Statistics (if eaten) */}
                            {eatenDays > 0 && (
                                <div className="mb-8">
                                    <div className="bg-white rounded-lg shadow-sm p-4">
                                        <h2 className="text-lg font-medium text-gray-800 mb-3">Essensrückmeldungen</h2>
                                        <div className="flex gap-4">
                                            {/* Positive Feedback */}
                                            <div className="bg-green-50 border border-green-100 rounded-lg p-3">
                                                <div className="flex items-center">
                                                    <div className="text-2xl mr-2">😊</div>
                                                    <div>
                                                        <h3 className="text-sm font-medium text-green-800">Positiv</h3>
                                                        <p className="text-xl font-bold text-green-600">
                                                            {positiveFeedback} <span className="text-sm font-normal">({eatenDays > 0 ? Math.round((positiveFeedback / eatenDays) * 100) : 0}%)</span>
                                                        </p>
                                                    </div>
                                                </div>
                                            </div>

                                            {/* Neutral Feedback */}
                                            <div className="bg-yellow-50 border border-yellow-100 rounded-lg p-3">
                                                <div className="flex items-center">
                                                    <div className="text-2xl mr-2">😐</div>
                                                    <div>
                                                        <h3 className="text-sm font-medium text-yellow-800">Neutral</h3>
                                                        <p className="text-xl font-bold text-yellow-600">
                                                            {neutralFeedback} <span className="text-sm font-normal">({eatenDays > 0 ? Math.round((neutralFeedback / eatenDays) * 100) : 0}%)</span>
                                                        </p>
                                                    </div>
                                                </div>
                                            </div>

                                            {/* Negative Feedback */}
                                            <div className="bg-red-50 border border-red-100 rounded-lg p-3">
                                                <div className="flex items-center">
                                                    <div className="text-2xl mr-2">😔</div>
                                                    <div>
                                                        <h3 className="text-sm font-medium text-red-800">Negativ</h3>
                                                        <p className="text-xl font-bold text-red-600">
                                                            {negativeFeedback} <span className="text-sm font-normal">({eatenDays > 0 ? Math.round((negativeFeedback / eatenDays) * 100) : 0}%)</span>
                                                        </p>
                                                    </div>
                                                </div>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            )}

                            {/* Mensa History - MODIFIED SECTION */}
                            <div className="space-y-6">
                                {filteredMensaHistory.length === 0 ? (
                                    <div className="bg-white rounded-lg p-8 text-center shadow-sm">
                                        <p className="text-gray-500">Keine Mensadaten für den ausgewählten Zeitraum verfügbar.</p>
                                    </div>
                                ) : (
                                    <div>
                                        {sortedDates.map(dateString => {
                                            const mensaEntries = groupedMensaHistory[dateString] ?? [];

                                            return (
                                                <div key={dateString} className="bg-white rounded-lg shadow-sm overflow-hidden mb-4">
                                                    <div className="bg-blue-50 px-6 py-3 border-b border-blue-100">
                                                        <h3 className="font-medium text-blue-800">
                                                            {formatDate(mensaEntries[0]?.date ?? dateString)}
                                                        </h3>
                                                    </div>
                                                    <div className="px-6 py-4">
                                                        {mensaEntries.length > 0 ? (
                                                            mensaEntries.map(entry => (
                                                                <div key={entry.id} className="flex items-center gap-3 mb-3 last:mb-0">
                                                                    {renderFeedbackEmoji(entry.has_eaten ? entry.feedback_type : undefined)}
                                                                    <div>
                                                                        <span className="font-medium text-gray-900">
                                                                            {entry.has_eaten ? "Gegessen" : "Nichts gegessen"}
                                                                        </span>
                                                                    </div>
                                                                </div>
                                                            ))
                                                        ) : (
                                                            <p className="text-gray-500 italic">Keine Mensadaten für diesen Tag</p>
                                                        )}
                                                    </div>
                                                </div>
                                            );
                                        })}
                                    </div>
                                )}
                            </div>
            </div>
        </ResponsiveLayout>
    );
}