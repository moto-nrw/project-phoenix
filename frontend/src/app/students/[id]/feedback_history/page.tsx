"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { ResponsiveLayout } from "@/components/dashboard";
import { Alert } from "~/components/ui/alert";
import { useSession } from "next-auth/react";
import { getStartDateForTimeRange } from "~/lib/date-helpers";

import { Loading } from "~/components/ui/loading";
// Student type (reused from student page)
interface Student {
  id: string;
  first_name: string;
  second_name: string;
  name?: string;
  school_class: string;
  group_id: string;
  group_name?: string;
  bus: boolean;
  current_room?: string;
  current_location?: string;
  guardian_name: string;
  guardian_contact: string;
  guardian_phone?: string;
  birthday?: string;
  notes?: string;
  buskind?: boolean;
  attendance_rate?: number;
}

// Feedback History interface
interface FeedbackEntry {
  id: string;
  timestamp: string;
  feedback_type: "positive" | "neutral" | "negative";
  is_mensa_feedback: boolean;
  comment?: string;
  is_valid?: boolean; // Neue Eigenschaft für die Validität
}

// Feedback type display labels
const feedbackTypeLabels: Record<FeedbackEntry["feedback_type"], string> = {
  positive: "Positives Feedback",
  neutral: "Neutrales Feedback",
  negative: "Negatives Feedback",
};

export default function StudentFeedbackHistoryPage() {
  const router = useRouter();
  const params = useParams();
  const searchParams = useSearchParams();
  const studentId = params.id as string;
  const referrer = searchParams.get("from") ?? "/students/search";
  useSession(); // Ensure session is active

  const [student, setStudent] = useState<Student | null>(null);
  const [feedbackHistory, setFeedbackHistory] = useState<FeedbackEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [timeRange, setTimeRange] = useState<string>("7days"); // Default to last 7 days

  // Fetch student data and feedback history
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
          bus: false,
          current_room: "Raum 1.2",
          current_location: "Anwesend - Raum 1.2",
          guardian_name: "Maria Müller",
          guardian_contact: "muellers@example.com",
          guardian_phone: "+49 176 12345678",
          birthday: "2016-06-15",
          notes: "Nimmt an der Musik-AG teil. Liebt Kunst und Lesen.",
          buskind: true,
          attendance_rate: 92.5,
        };

        // Mock feedback history data - Jetzt mit aufeinanderfolgenden Daten und den ersten beiden als ungültig markiert
        const mockFeedbackHistory: FeedbackEntry[] = [
          {
            id: "1",
            timestamp: "2025-05-14T15:30:23",
            feedback_type: "positive",
            is_mensa_feedback: false,
            comment: "",
            is_valid: false, // Als ungültig markiert
          },
          {
            id: "2",
            timestamp: "2025-05-13T15:45:12",
            feedback_type: "negative",
            is_mensa_feedback: false,
            comment: "",
            is_valid: false, // Als ungültig markiert
          },
          {
            id: "3",
            timestamp: "2025-05-12T15:25:18",
            feedback_type: "positive",
            is_mensa_feedback: false,
            comment: "",
          },
          {
            id: "4",
            timestamp: "2025-05-11T15:10:09",
            feedback_type: "neutral",
            is_mensa_feedback: false,
            comment: "",
          },
          {
            id: "5",
            timestamp: "2025-05-10T12:30:22",
            feedback_type: "negative",
            is_mensa_feedback: false,
            comment: "",
          },
          {
            id: "6",
            timestamp: "2025-05-09T15:50:37",
            feedback_type: "positive",
            is_mensa_feedback: false,
            comment: "",
          },
          {
            id: "7",
            timestamp: "2025-05-08T16:15:54",
            feedback_type: "neutral",
            is_mensa_feedback: false,
            comment: "",
          },
        ];

        setStudent(mockStudent);
        setFeedbackHistory(mockFeedbackHistory);
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
  const getFilteredFeedbackHistory = (): FeedbackEntry[] => {
    // Wenn "all" ausgewählt ist, geben wir die gesamte Historie zurück
    if (timeRange === "all") {
      return feedbackHistory;
    }

    const now = new Date();
    const startDate = getStartDateForTimeRange(timeRange, now);

    return feedbackHistory.filter((entry) => {
      const entryDate = new Date(entry.timestamp);
      return entryDate >= startDate && entryDate <= now;
    });
  };

  // Apply filtering
  const filteredFeedbackHistory = getFilteredFeedbackHistory();

  // Get year from class
  const getYear = (schoolClass: string): number => {
    const yearMatch = /^(\d)/.exec(schoolClass);
    return yearMatch?.[1] ? Number.parseInt(yearMatch[1], 10) : 0;
  };

  // Determine color for year indicator
  const getYearColor = (year: number): string => {
    switch (year) {
      case 1:
        return "bg-blue-500";
      case 2:
        return "bg-green-500";
      case 3:
        return "bg-yellow-500";
      case 4:
        return "bg-purple-500";
      default:
        return "bg-gray-400";
    }
  };

  // Format date function is actually used in the dateObj.toLocaleDateString call
  // Format time for display
  const formatTime = (dateString: string): string => {
    const date = new Date(dateString);
    return date.toLocaleTimeString("de-DE", {
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  // Group feedback history by date
  const groupedFeedbackHistory = filteredFeedbackHistory.reduce(
    (groups, entry) => {
      const date = new Date(entry.timestamp).toLocaleDateString("de-DE", {
        day: "2-digit",
        month: "2-digit",
        year: "numeric",
      });
      groups[date] ??= [];
      groups[date].push(entry);
      return groups;
    },
    {} as Record<string, FeedbackEntry[]>,
  );

  // Sort dates in descending order (most recent first)
  const sortedDates = Object.keys(groupedFeedbackHistory).sort((a, b) => {
    return new Date(b).getTime() - new Date(a).getTime();
  });

  // Render the appropriate emoji based on feedback type
  const renderFeedbackEmoji = (type: string) => {
    switch (type) {
      case "positive":
        return (
          <div className="flex h-10 w-10 items-center justify-center rounded-full bg-green-100 text-2xl text-green-500">
            😊
          </div>
        );
      case "neutral":
        return (
          <div className="flex h-10 w-10 items-center justify-center rounded-full bg-yellow-100 text-2xl text-yellow-500">
            😐
          </div>
        );
      case "negative":
        return (
          <div className="flex h-10 w-10 items-center justify-center rounded-full bg-red-100 text-2xl text-red-500">
            😔
          </div>
        );
      default:
        return null;
    }
  };

  const year = student ? getYear(student.school_class) : 0;
  const yearColor = getYearColor(year);

  // Count feedback by type (zähle nur gültige Feedbacks)
  const positiveFeedbackCount = filteredFeedbackHistory.filter(
    (entry) => entry.feedback_type === "positive" && entry.is_valid !== false,
  ).length;
  const neutralFeedbackCount = filteredFeedbackHistory.filter(
    (entry) => entry.feedback_type === "neutral" && entry.is_valid !== false,
  ).length;
  const negativeFeedbackCount = filteredFeedbackHistory.filter(
    (entry) => entry.feedback_type === "negative" && entry.is_valid !== false,
  ).length;

  // Calculate percentages
  const totalFeedback =
    positiveFeedbackCount + neutralFeedbackCount + negativeFeedbackCount;
  const positivePercentage =
    totalFeedback > 0
      ? Math.round((positiveFeedbackCount / totalFeedback) * 100)
      : 0;
  const neutralPercentage =
    totalFeedback > 0
      ? Math.round((neutralFeedbackCount / totalFeedback) * 100)
      : 0;
  const negativePercentage =
    totalFeedback > 0
      ? Math.round((negativeFeedbackCount / totalFeedback) * 100)
      : 0;

  if (loading) {
    return (
      <ResponsiveLayout studentName="..." referrerPage={referrer}>
        <Loading fullPage={false} />
      </ResponsiveLayout>
    );
  }

  if (error || !student) {
    return (
      <ResponsiveLayout referrerPage={referrer}>
        <div className="flex min-h-[80vh] flex-col items-center justify-center">
          <Alert type="error" message={error ?? "Schüler nicht gefunden"} />
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
    <ResponsiveLayout studentName={student?.name} referrerPage={referrer}>
      <div className="mx-auto max-w-7xl">
        {/* Back Button */}
        <div className="mb-6">
          <button
            onClick={() =>
              router.push(`/students/${studentId}?from=${referrer}`)
            }
            className="flex items-center text-gray-600 transition-colors hover:text-blue-600"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              className="mr-1 h-5 w-5"
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
              {student.first_name[0]}
              {student.second_name[0]}
            </div>
            <div>
              <h1 className="text-3xl font-bold">{student.name}</h1>
              <div className="mt-1 flex items-center">
                <span className="opacity-90">
                  Klasse {student.school_class}
                </span>
                <span
                  className={`ml-2 inline-block h-3 w-3 rounded-full ${yearColor}`}
                  title={`Jahrgang ${year}`}
                ></span>
                <span className="mx-2">•</span>
                <span className="opacity-90">Gruppe: {student.group_name}</span>
              </div>
              <div className="mt-4">
                <h2 className="text-2xl font-semibold text-white">
                  Feedbackhistorie
                </h2>
              </div>
            </div>
          </div>
        </div>

        {/* Filter Controls and Feedback Overview */}
        <div className="mb-8">
          <div className="rounded-lg bg-white p-4 shadow-sm">
            <div className="flex flex-col md:flex-row md:justify-between">
              {/* Time Range Filter */}
              <div className="mb-4 md:mb-0">
                <h2 className="mb-3 text-lg font-medium text-gray-800">
                  Filter
                </h2>
                <div className="flex flex-wrap gap-2">
                  <button
                    className={`rounded-lg px-4 py-2 transition-colors ${timeRange === "all" ? "bg-blue-500 text-white" : "bg-gray-100 text-gray-700 hover:bg-gray-200"}`}
                    onClick={() => setTimeRange("all")}
                  >
                    Alle
                  </button>
                  <button
                    className={`rounded-lg px-4 py-2 transition-colors ${timeRange === "today" ? "bg-blue-500 text-white" : "bg-gray-100 text-gray-700 hover:bg-gray-200"}`}
                    onClick={() => setTimeRange("today")}
                  >
                    Heute
                  </button>
                  <button
                    className={`rounded-lg px-4 py-2 transition-colors ${timeRange === "week" ? "bg-blue-500 text-white" : "bg-gray-100 text-gray-700 hover:bg-gray-200"}`}
                    onClick={() => setTimeRange("week")}
                  >
                    Diese Woche
                  </button>
                  <button
                    className={`rounded-lg px-4 py-2 transition-colors ${timeRange === "7days" ? "bg-blue-500 text-white" : "bg-gray-100 text-gray-700 hover:bg-gray-200"}`}
                    onClick={() => setTimeRange("7days")}
                  >
                    Letzte 7 Tage
                  </button>
                  <button
                    className={`rounded-lg px-4 py-2 transition-colors ${timeRange === "month" ? "bg-blue-500 text-white" : "bg-gray-100 text-gray-700 hover:bg-gray-200"}`}
                    onClick={() => setTimeRange("month")}
                  >
                    Diesen Monat
                  </button>
                </div>
              </div>

              {/* Feedback Overview - now placed beside the filter */}
              <div>
                <h2 className="mb-3 text-lg font-medium text-gray-800">
                  Feedback-Übersicht
                </h2>
                <div className="flex gap-4">
                  {/* Positive Feedback */}
                  <div className="rounded-lg border border-green-100 bg-green-50 p-3">
                    <div className="flex items-center">
                      <div className="mr-2 text-2xl">😊</div>
                      <div>
                        <h3 className="text-sm font-medium text-green-800">
                          Positiv
                        </h3>
                        <p className="text-xl font-bold text-green-600">
                          {positiveFeedbackCount}{" "}
                          <span className="text-sm font-normal">
                            ({positivePercentage}%)
                          </span>
                        </p>
                      </div>
                    </div>
                  </div>

                  {/* Neutral Feedback */}
                  <div className="rounded-lg border border-yellow-100 bg-yellow-50 p-3">
                    <div className="flex items-center">
                      <div className="mr-2 text-2xl">😐</div>
                      <div>
                        <h3 className="text-sm font-medium text-yellow-800">
                          Neutral
                        </h3>
                        <p className="text-xl font-bold text-yellow-600">
                          {neutralFeedbackCount}{" "}
                          <span className="text-sm font-normal">
                            ({neutralPercentage}%)
                          </span>
                        </p>
                      </div>
                    </div>
                  </div>

                  {/* Negative Feedback */}
                  <div className="rounded-lg border border-red-100 bg-red-50 p-3">
                    <div className="flex items-center">
                      <div className="mr-2 text-2xl">😔</div>
                      <div>
                        <h3 className="text-sm font-medium text-red-800">
                          Negativ
                        </h3>
                        <p className="text-xl font-bold text-red-600">
                          {negativeFeedbackCount}{" "}
                          <span className="text-sm font-normal">
                            ({negativePercentage}%)
                          </span>
                        </p>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Feedback History */}
        <div className="space-y-6">
          {filteredFeedbackHistory.length === 0 ? (
            <div className="rounded-lg bg-white p-8 text-center shadow-sm">
              <p className="text-gray-500">
                Kein Feedback für den ausgewählten Zeitraum verfügbar.
              </p>
            </div>
          ) : (
            <div>
              {sortedDates.map((dateString) => {
                const feedbackForDate =
                  groupedFeedbackHistory[dateString] ?? [];
                const dateObj = new Date(
                  feedbackForDate[0]?.timestamp ?? dateString,
                );

                return (
                  <div
                    key={dateString}
                    className="mb-4 overflow-hidden rounded-lg bg-white shadow-sm"
                  >
                    <div className="border-b border-blue-100 bg-blue-50 px-6 py-3">
                      <h3 className="font-medium text-blue-800">
                        {dateObj.toLocaleDateString("de-DE", {
                          weekday: "long",
                          year: "numeric",
                          month: "long",
                          day: "numeric",
                        })}
                      </h3>
                    </div>
                    <div className="px-6 py-4">
                      {feedbackForDate.length > 0 ? (
                        feedbackForDate.map((feedback) => (
                          <div
                            key={feedback.id}
                            className="mb-3 flex items-center gap-3 last:mb-0"
                          >
                            {renderFeedbackEmoji(feedback.feedback_type)}
                            <div className="flex flex-col">
                              <span className="font-medium text-gray-900">
                                {feedbackTypeLabels[feedback.feedback_type]}
                                <span className="ml-2 text-sm text-gray-500">
                                  {formatTime(feedback.timestamp)}
                                </span>
                              </span>
                              {feedback.is_valid === false && (
                                <span className="mt-1 text-sm text-red-500">
                                  Ungültiges Feedback
                                </span>
                              )}
                              {feedback.comment && (
                                <p className="mt-1 text-sm text-gray-600">
                                  {feedback.comment}
                                </p>
                              )}
                            </div>
                          </div>
                        ))
                      ) : (
                        <p className="text-gray-500 italic">
                          Kein Feedback an diesem Tag
                        </p>
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
