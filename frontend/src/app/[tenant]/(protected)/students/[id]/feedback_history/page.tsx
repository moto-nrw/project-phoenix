"use client";

import { useEffect, useState } from "react";
import { useParams, useSearchParams } from "next/navigation";
import { useTenantRouter } from "~/lib/tenant-router";
import { Alert } from "~/components/ui/alert";
import { useSession } from "next-auth/react";
import { getStartDateForTimeRange } from "~/lib/date-helpers";
import { useStudentHistoryBreadcrumb } from "~/lib/breadcrumb-context";
import { InfoCard } from "~/components/ui/info-card";

import { Loading } from "~/components/ui/loading";
import { createLogger } from "~/lib/logger";
import { fetchStudent } from "~/lib/student-api";
import type { Student } from "~/lib/student-helpers";
import { fetchStudentFeedback, type FeedbackEntry } from "~/lib/feedback-api";

const logger = createLogger({ component: "StudentFeedbackHistoryPage" });

// Feedback type display labels
const feedbackTypeLabels: Record<FeedbackEntry["feedback_type"], string> = {
  positive: "Positives Feedback",
  neutral: "Neutrales Feedback",
  negative: "Negatives Feedback",
};

// Feedback type indicator config (no emojis)
const feedbackTypeConfig: Record<
  FeedbackEntry["feedback_type"],
  {
    bgColor: string;
    iconColor: string;
    icon: React.ReactNode;
  }
> = {
  positive: {
    bgColor: "bg-green-100",
    iconColor: "text-green-600",
    icon: (
      <svg
        className="h-5 w-5"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        strokeWidth={2}
      >
        <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
      </svg>
    ),
  },
  neutral: {
    bgColor: "bg-yellow-100",
    iconColor: "text-yellow-600",
    icon: (
      <svg
        className="h-5 w-5"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        strokeWidth={2}
      >
        <path strokeLinecap="round" strokeLinejoin="round" d="M5 12h14" />
      </svg>
    ),
  },
  negative: {
    bgColor: "bg-red-100",
    iconColor: "text-red-600",
    icon: (
      <svg
        className="h-5 w-5"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        strokeWidth={2}
      >
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          d="M6 18L18 6M6 6l12 12"
        />
      </svg>
    ),
  },
};

export default function StudentFeedbackHistoryPage() {
  const router = useTenantRouter();
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

  useStudentHistoryBreadcrumb({ studentName: student?.name, referrer });

  // Fetch student data and feedback history
  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);

    async function loadData() {
      try {
        const [studentData, feedbackData] = await Promise.all([
          fetchStudent(studentId),
          fetchStudentFeedback(studentId),
        ]);

        if (cancelled) return;
        setStudent(studentData);
        setFeedbackHistory(feedbackData);
      } catch (err) {
        if (cancelled) return;
        logger.error("failed_to_fetch_feedback_history", {
          error: err instanceof Error ? err.message : String(err),
        });
        setError("Fehler beim Laden der Daten.");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    void loadData();
    return () => {
      cancelled = true;
    };
  }, [studentId]);

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

  const year = student ? getYear(student.school_class) : 0;
  const yearColor = getYearColor(year);

  // Count feedback by type
  const positiveFeedbackCount = filteredFeedbackHistory.filter(
    (entry) => entry.feedback_type === "positive",
  ).length;
  const neutralFeedbackCount = filteredFeedbackHistory.filter(
    (entry) => entry.feedback_type === "neutral",
  ).length;
  const negativeFeedbackCount = filteredFeedbackHistory.filter(
    (entry) => entry.feedback_type === "negative",
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
    return <Loading fullPage={false} />;
  }

  if (error || !student) {
    return (
      <div className="flex min-h-[80vh] flex-col items-center justify-center">
        <Alert type="error" message={error ?? "Schüler nicht gefunden"} />
        <button
          onClick={() => router.push(referrer)}
          className="mt-4 rounded bg-blue-100 px-4 py-2 text-blue-800 transition-colors hover:bg-blue-200"
        >
          Zurück
        </button>
      </div>
    );
  }

  const timeRangeOptions = [
    { key: "all", label: "Alle" },
    { key: "today", label: "Heute" },
    { key: "week", label: "Diese Woche" },
    { key: "7days", label: "Letzte 7 Tage" },
    { key: "month", label: "Diesen Monat" },
  ];

  return (
    <div className="mx-auto max-w-7xl">
      {/* Back Button */}
      <div className="mb-6">
        <button
          onClick={() => router.push(`/students/${studentId}?from=${referrer}`)}
          className="flex items-center text-gray-600 transition-colors hover:text-blue-600"
        >
          <svg
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

      {/* Student Profile Header */}
      <div className="relative mb-8 overflow-hidden rounded-xl bg-gradient-to-r from-teal-500 to-blue-600 p-6 text-white shadow-md">
        <div className="flex items-center">
          <div className="mr-6 flex h-24 w-24 items-center justify-center rounded-full bg-white/30 text-4xl font-bold">
            {student.first_name?.[0] ?? ""}
            {student.second_name?.[0] ?? ""}
          </div>
          <div>
            <h1 className="text-3xl font-bold">{student.name}</h1>
            <div className="mt-1 flex items-center">
              <span className="opacity-90">Klasse {student.school_class}</span>
              <span
                className={`ml-2 inline-block h-3 w-3 rounded-full ${yearColor}`}
                title={`Jahrgang ${year}`}
              ></span>
              <span className="mx-2">·</span>
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
      <div className="mb-8 grid grid-cols-1 gap-6 lg:grid-cols-2">
        {/* Time Range Filter */}
        <div className="rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm sm:p-6">
          <h2 className="mb-3 text-base font-semibold text-gray-900 sm:text-lg">
            Filter
          </h2>
          <div className="flex flex-wrap gap-2">
            {timeRangeOptions.map((option) => (
              <button
                key={option.key}
                className={`rounded-lg px-4 py-2 text-sm font-medium transition-colors ${
                  timeRange === option.key
                    ? "bg-blue-500 text-white"
                    : "bg-gray-100 text-gray-700 hover:bg-gray-200"
                }`}
                onClick={() => setTimeRange(option.key)}
              >
                {option.label}
              </button>
            ))}
          </div>
        </div>

        {/* Feedback Overview */}
        <InfoCard
          title="Feedback-Übersicht"
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
                d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"
              />
            </svg>
          }
        >
          <div className="flex flex-wrap gap-3">
            <StatBlock
              label="Positiv"
              count={positiveFeedbackCount}
              percentage={positivePercentage}
              bgColor="bg-green-50"
              borderColor="border-green-100"
              labelColor="text-green-800"
              valueColor="text-green-600"
              indicator={feedbackTypeConfig.positive}
            />
            <StatBlock
              label="Neutral"
              count={neutralFeedbackCount}
              percentage={neutralPercentage}
              bgColor="bg-yellow-50"
              borderColor="border-yellow-100"
              labelColor="text-yellow-800"
              valueColor="text-yellow-600"
              indicator={feedbackTypeConfig.neutral}
            />
            <StatBlock
              label="Negativ"
              count={negativeFeedbackCount}
              percentage={negativePercentage}
              bgColor="bg-red-50"
              borderColor="border-red-100"
              labelColor="text-red-800"
              valueColor="text-red-600"
              indicator={feedbackTypeConfig.negative}
            />
          </div>
        </InfoCard>
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
              const feedbackForDate = groupedFeedbackHistory[dateString] ?? [];
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
                  <div className="divide-y divide-gray-50 px-6 py-2">
                    {feedbackForDate.map((feedback) => {
                      const config = feedbackTypeConfig[feedback.feedback_type];
                      return (
                        <div
                          key={feedback.id}
                          className="flex items-center gap-3 py-3"
                          data-testid={`feedback-indicator-${feedback.feedback_type}`}
                        >
                          <div
                            className={`flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-full ${config.bgColor} ${config.iconColor}`}
                          >
                            {config.icon}
                          </div>
                          <div className="flex flex-col">
                            <span className="font-medium text-gray-900">
                              {feedbackTypeLabels[feedback.feedback_type]}
                              <span className="ml-2 text-sm text-gray-500">
                                {formatTime(feedback.timestamp)}
                              </span>
                            </span>
                            {feedback.is_mensa_feedback && (
                              <span className="mt-1 text-sm text-blue-500">
                                Mensa-Feedback
                              </span>
                            )}
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}

// Stat block for the feedback overview
function StatBlock({
  label,
  count,
  percentage,
  bgColor,
  borderColor,
  labelColor,
  valueColor,
  indicator,
}: Readonly<{
  label: string;
  count: number;
  percentage: number;
  bgColor: string;
  borderColor: string;
  labelColor: string;
  valueColor: string;
  indicator: { bgColor: string; iconColor: string; icon: React.ReactNode };
}>) {
  return (
    <div className={`flex-1 rounded-lg border ${borderColor} ${bgColor} p-3`}>
      <div className="flex items-center">
        <div
          className={`mr-2 flex h-8 w-8 items-center justify-center rounded-full ${indicator.bgColor} ${indicator.iconColor}`}
        >
          {indicator.icon}
        </div>
        <div>
          <h3 className={`text-sm font-medium ${labelColor}`}>{label}</h3>
          <p className={`text-xl font-bold ${valueColor}`}>
            {count} <span className="text-sm font-normal">({percentage}%)</span>
          </p>
        </div>
      </div>
    </div>
  );
}
