"use client";

import { useEffect, useState } from "react";
import { useParams, useSearchParams } from "next/navigation";
import { useTenantRouter } from "~/lib/tenant-router";
import { Alert } from "~/components/ui/alert";
import { useSession } from "next-auth/react";
import { getStartDateForTimeRange } from "~/lib/date-helpers";
import { useStudentHistoryBreadcrumb } from "~/lib/breadcrumb-context";
import { BackButton } from "~/components/ui/back-button";
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

// Feedback type indicator config
const feedbackTypeConfig: Record<
  FeedbackEntry["feedback_type"],
  { bgColor: string; iconColor: string; icon: React.ReactNode }
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

const timeRangeOptions = [
  { key: "all", label: "Alle" },
  { key: "today", label: "Heute" },
  { key: "week", label: "Diese Woche" },
  { key: "7days", label: "Letzte 7 Tage" },
  { key: "month", label: "Diesen Monat" },
];

export default function StudentFeedbackHistoryPage() {
  const router = useTenantRouter();
  const params = useParams();
  const searchParams = useSearchParams();
  const studentId = params.id as string;
  const referrer = searchParams.get("from") ?? "/students/search";
  useSession();

  const [student, setStudent] = useState<Student | null>(null);
  const [feedbackHistory, setFeedbackHistory] = useState<FeedbackEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [timeRange, setTimeRange] = useState<string>("7days");

  useStudentHistoryBreadcrumb({ studentName: student?.name, referrer });

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

  const getFilteredFeedbackHistory = (): FeedbackEntry[] => {
    if (timeRange === "all") return feedbackHistory;

    const now = new Date();
    const startDate = getStartDateForTimeRange(timeRange, now);
    return feedbackHistory.filter((entry) => {
      const entryDate = new Date(entry.timestamp);
      return entryDate >= startDate && entryDate <= now;
    });
  };

  const filteredFeedbackHistory = getFilteredFeedbackHistory();

  const formatTime = (dateString: string): string => {
    const date = new Date(dateString);
    return date.toLocaleTimeString("de-DE", {
      hour: "2-digit",
      minute: "2-digit",
    });
  };

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

  const sortedDates = Object.keys(groupedFeedbackHistory).sort((a, b) => {
    return new Date(b).getTime() - new Date(a).getTime();
  });

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

  return (
    <div className="mx-auto max-w-7xl">
      <BackButton referrer={`/students/${studentId}?from=${referrer}`} />

      {/* Page Header — flat style matching student detail page */}
      <div className="mb-6 ml-6">
        <h1 className="text-2xl font-bold text-gray-900 md:text-3xl">
          {student.name}
        </h1>
        <p className="mt-1 text-sm text-gray-500">Feedbackhistorie</p>
        <div className="mt-2 flex items-center gap-2 text-sm text-gray-600">
          <svg
            className="h-4 w-4 text-gray-400"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
            />
          </svg>
          <span>
            Klasse {student.school_class} · Gruppe: {student.group_name}
          </span>
        </div>
      </div>

      {/* Filter + Overview */}
      <div className="mb-6 space-y-4 sm:space-y-6">
        <InfoCard
          title="Filter"
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
                d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z"
              />
            </svg>
          }
        >
          <div className="flex flex-wrap gap-2">
            {timeRangeOptions.map((option) => (
              <button
                key={option.key}
                className={`rounded-lg px-3 py-1.5 text-sm font-medium transition-colors ${
                  timeRange === option.key
                    ? "bg-[#5080D8] text-white"
                    : "bg-gray-100 text-gray-700 hover:bg-gray-200"
                }`}
                onClick={() => setTimeRange(option.key)}
              >
                {option.label}
              </button>
            ))}
          </div>
        </InfoCard>

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
          <div className="grid grid-cols-3 gap-3">
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

      {/* Feedback Timeline */}
      {filteredFeedbackHistory.length === 0 ? (
        <div className="rounded-2xl border border-gray-100 bg-white/50 p-8 text-center backdrop-blur-sm">
          <p className="text-gray-500">
            Kein Feedback für den ausgewählten Zeitraum verfügbar.
          </p>
        </div>
      ) : (
        <div className="space-y-4">
          {sortedDates.map((dateString) => {
            const feedbackForDate = groupedFeedbackHistory[dateString] ?? [];
            const dateObj = new Date(
              feedbackForDate[0]?.timestamp ?? dateString,
            );

            return (
              <div
                key={dateString}
                className="overflow-hidden rounded-2xl border border-gray-100 bg-white/50 backdrop-blur-sm"
              >
                <div className="border-b border-gray-100 px-4 py-3 sm:px-6">
                  <h3 className="text-sm font-semibold text-gray-900">
                    {dateObj.toLocaleDateString("de-DE", {
                      weekday: "long",
                      year: "numeric",
                      month: "long",
                      day: "numeric",
                    })}
                  </h3>
                </div>
                <div className="divide-y divide-gray-50 px-4 sm:px-6">
                  {feedbackForDate.map((feedback) => {
                    const config = feedbackTypeConfig[feedback.feedback_type];
                    return (
                      <div
                        key={feedback.id}
                        className="flex items-center gap-3 py-3"
                        data-testid={`feedback-indicator-${feedback.feedback_type}`}
                      >
                        <div
                          className={`flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-full ${config.bgColor} ${config.iconColor}`}
                        >
                          {config.icon}
                        </div>
                        <div className="min-w-0 flex-1">
                          <p className="text-sm font-medium text-gray-900">
                            {feedbackTypeLabels[feedback.feedback_type]}
                          </p>
                          <div className="flex items-center gap-2">
                            <span className="text-xs text-gray-500">
                              {formatTime(feedback.timestamp)}
                            </span>
                            {feedback.is_mensa_feedback && (
                              <span className="inline-flex items-center rounded-full bg-[#5080D8]/10 px-2 py-0.5 text-xs font-medium text-[#5080D8]">
                                Mensa-Feedback
                              </span>
                            )}
                          </div>
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
  );
}

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
    <div className={`rounded-lg border ${borderColor} ${bgColor} p-3`}>
      <div className="flex items-center gap-2">
        <div
          className={`flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-full ${indicator.bgColor} ${indicator.iconColor}`}
        >
          {indicator.icon}
        </div>
        <div className="min-w-0">
          <p className={`text-xs font-medium ${labelColor}`}>{label}</p>
          <p className={`text-lg leading-tight font-bold ${valueColor}`}>
            {count}
            <span className="ml-1 text-xs font-normal">({percentage}%)</span>
          </p>
        </div>
      </div>
    </div>
  );
}
