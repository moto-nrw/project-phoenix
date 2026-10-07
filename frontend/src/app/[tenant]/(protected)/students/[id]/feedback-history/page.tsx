"use client";

import { Suspense, useEffect, useMemo, useState } from "react";
import dynamic from "next/dynamic";
import { useParams, useSearchParams } from "next/navigation";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { Skeleton } from "~/components/ui/skeleton";
import { useSession } from "next-auth/react";
import { getStartDateForTimeRange, toISODate } from "~/lib/date-helpers";
import { useStudentHistoryBreadcrumb } from "~/lib/breadcrumb-context";
import { useScrollToTop } from "~/lib/hooks/use-scroll-to-top";
import { BackButton } from "~/components/ui/back-button";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { useApiLoadError } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { FeedbackHistorySkeleton } from "./page-skeleton";
import { createLogger } from "~/lib/logger";
import { fetchStudent } from "~/lib/student-api";
import type { Student } from "~/lib/student-helpers";
import { fetchStudentFeedback, type FeedbackEntry } from "~/lib/feedback-api";
import { ChevronDown, ChevronUp } from "lucide-react";
import { MOTO_COLOR_PALETTE } from "~/lib/location-helper";

const logger = createLogger({ component: "StudentFeedbackHistoryPage" });

const FEATURE_DISABLED_DESCRIPTION =
  "Die Feedbackhistorie ist für Ihre Schule ausgeschaltet. Sie lässt sich in den Einstellungen unter Datenschutz einschalten.";

const FEEDBACK_HISTORY_DESCRIPTION =
  "Rückmeldungen dieses Kindes im Zeitverlauf.";

const feedbackTypeLabels: Record<FeedbackEntry["feedback_type"], string> = {
  positive: "Positives Feedback",
  neutral: "Neutrales Feedback",
  negative: "Negatives Feedback",
};

const FeedbackHistoryChart = dynamic(() => import("./feedback-history-chart"), {
  ssr: false,
  loading: () => (
    <Skeleton className="h-[180px] w-full sm:h-[220px]" aria-hidden />
  ),
});

const feedbackToneColors = {
  positive: MOTO_COLOR_PALETTE.green.base,
  neutral: MOTO_COLOR_PALETTE.amber.base,
  negative: MOTO_COLOR_PALETTE.red.base,
} satisfies Record<FeedbackEntry["feedback_type"], string>;

const timeRangeOptions = [
  { value: "all", label: "Alle" },
  { value: "today", label: "Heute" },
  { value: "week", label: "Diese Woche" },
  { value: "7days", label: "Letzte 7 Tage" },
  { value: "month", label: "Diesen Monat" },
] as const;

const DAY_NAMES = ["Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"];

function formatDateShort(date: Date): string {
  return date.toLocaleDateString("de-DE", {
    timeZone: "Europe/Berlin",
    day: "2-digit",
    month: "2-digit",
  });
}

export default function StudentFeedbackHistoryPage() {
  return (
    <Suspense fallback={<FeedbackHistorySkeleton />}>
      <StudentFeedbackHistoryPageContent />
    </Suspense>
  );
}

function StudentFeedbackHistoryPageContent() {
  const params = useParams();
  const searchParams = useSearchParams();
  const studentId = params.id as string;
  const referrer = searchParams.get("from") ?? "/students/search";
  useSession();

  const [student, setStudent] = useState<Student | null>(null);
  // `null` while the history never loaded: no "0 Einträge" next to a load
  // error (#2517).
  const [feedbackHistory, setFeedbackHistory] = useState<
    FeedbackEntry[] | null
  >(null);
  const [loading, setLoading] = useState(true);
  // Set at once when a load fails, so no „Kind nicht gefunden“ flashes
  // while the catalog text is on its way.
  const [failed, setFailed] = useState(false);
  const [notFound, setNotFound] = useState(false);
  const [featureDisabled, setFeatureDisabled] = useState(false);
  const pageLoad = useApiLoadError();
  const showPageError = pageLoad.show;
  const clearPageError = pageLoad.clear;
  // Bumped by „Wiederholen“ to load the page again.
  const [attempt, setAttempt] = useState(0);
  const [timeRange, setTimeRange] =
    useState<(typeof timeRangeOptions)[number]["value"]>("7days");
  const [showDetails, setShowDetails] = useState(false);

  useStudentHistoryBreadcrumb({ studentName: student?.name, referrer });

  // Start at the top instead of inheriting the previous page's scroll position
  useScrollToTop(studentId);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setFailed(false);
    setNotFound(false);
    setFeatureDisabled(false);
    clearPageError();

    async function loadData() {
      const [studentResult, feedbackResult] = await Promise.allSettled([
        fetchStudent(studentId),
        fetchStudentFeedback(studentId),
      ]);
      if (cancelled) return;
      const studentData =
        studentResult.status === "fulfilled" ? studentResult.value : null;
      setStudent(studentData);
      setFeedbackHistory(
        feedbackResult.status === "fulfilled" ? feedbackResult.value : null,
      );
      const failure =
        studentResult.status === "rejected"
          ? (studentResult.reason as unknown)
          : feedbackResult.status === "rejected"
            ? (feedbackResult.reason as unknown)
            : null;
      if (failure !== null) {
        logger.error("failed_to_fetch_feedback_history", {
          error: failure instanceof Error ? failure.message : String(failure),
        });
        // A switched-off feature is a state, not an error. It is read from the
        // switch in the child's record, never from the text of the 403.
        const switchedOff =
          (studentData as { feedback_enabled?: boolean } | null)
            ?.feedback_enabled === false;
        if (switchedOff) {
          setFeatureDisabled(true);
        } else if (failure instanceof ApiError && failure.status === 404) {
          setNotFound(true);
        } else {
          setFailed(true);
          void showPageError(failure, {
            object: "die Feedbackhistorie",
            retry: () => setAttempt((current) => current + 1),
          });
        }
      }
      setLoading(false);
    }

    void loadData();
    return () => {
      cancelled = true;
    };
  }, [attempt, clearPageError, showPageError, studentId]);

  const filteredFeedbackHistory = useMemo(() => {
    if (feedbackHistory === null) return [];
    if (timeRange === "all") return feedbackHistory;
    const now = new Date();
    const startDate = getStartDateForTimeRange(timeRange, now);
    return feedbackHistory.filter((entry) => {
      const entryDate = new Date(entry.timestamp);
      return entryDate >= startDate;
    });
  }, [feedbackHistory, timeRange]);

  // Counts
  const positiveFeedbackCount = filteredFeedbackHistory.filter(
    (e) => e.feedback_type === "positive",
  ).length;
  const neutralFeedbackCount = filteredFeedbackHistory.filter(
    (e) => e.feedback_type === "neutral",
  ).length;
  const negativeFeedbackCount = filteredFeedbackHistory.filter(
    (e) => e.feedback_type === "negative",
  ).length;
  const totalFeedback =
    positiveFeedbackCount + neutralFeedbackCount + negativeFeedbackCount;

  // Chart data: aggregate per day
  const chartData = useMemo(() => {
    const dayMap = new Map<
      string,
      { date: Date; positive: number; neutral: number; negative: number }
    >();

    for (const entry of filteredFeedbackHistory) {
      const d = new Date(entry.timestamp);
      const key = toISODate(d);
      const existing = dayMap.get(key) ?? {
        date: d,
        positive: 0,
        neutral: 0,
        negative: 0,
      };
      existing[entry.feedback_type]++;
      dayMap.set(key, existing);
    }

    return Array.from(dayMap.entries())
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([, v]) => {
        const dayIndex = (v.date.getDay() + 6) % 7;
        return {
          day: DAY_NAMES[dayIndex] ?? "",
          label: `${DAY_NAMES[dayIndex] ?? ""} ${formatDateShort(v.date)}`,
          positive: v.positive,
          neutral: v.neutral,
          negative: v.negative,
        };
      });
  }, [filteredFeedbackHistory]);

  // Group for detail list
  const groupedFeedbackHistory = useMemo(() => {
    const groups: Record<string, FeedbackEntry[]> = {};
    for (const entry of filteredFeedbackHistory) {
      const date = new Date(entry.timestamp).toLocaleDateString("de-DE", {
        timeZone: "Europe/Berlin",
        day: "2-digit",
        month: "2-digit",
        year: "numeric",
      });
      groups[date] ??= [];
      groups[date].push(entry);
    }
    return groups;
  }, [filteredFeedbackHistory]);

  const sortedDates = useMemo(
    () =>
      Object.keys(groupedFeedbackHistory).sort(
        (a, b) => new Date(b).getTime() - new Date(a).getTime(),
      ),
    [groupedFeedbackHistory],
  );

  const formatTime = (dateString: string): string => {
    const date = new Date(dateString);
    return date.toLocaleTimeString("de-DE", {
      timeZone: "Europe/Berlin",
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  // Statuszeile: Klasse, Gruppe und die Zahl der Einträge im gewählten
  // Zeitraum, alles aus den Daten, die die Seite ohnehin geladen hat.
  const feedbackLine =
    feedbackHistory === null
      ? null
      : `${totalFeedback} ${totalFeedback === 1 ? "Eintrag" : "Einträge"}`;
  const studentMeta = student
    ? [
        student.school_class,
        student.group_name ? `Gruppe: ${student.group_name}` : null,
        feedbackLine,
      ]
        .filter(Boolean)
        .join(" · ")
    : "";
  const errorMessage = loading
    ? null
    : featureDisabled
      ? null
      : failed
        ? pageLoad.error
        : notFound || !student
          ? "Kind nicht gefunden"
          : null;
  // Im Fehlerfall führt der Rückweg auf die Liste, sonst auf die Kindakte in
  // den Reiter, aus dem diese Unterseite geöffnet wurde.
  const backReferrer =
    errorMessage !== null || featureDisabled
      ? referrer
      : `/students/${studentId}?from=${referrer}&tab=historie`;

  return (
    <>
      {/* tab=historie returns to the originating tab on the detail page
          (this sub-page lives under Historie, issue #1501); from= still drives
          the detail page's own back button to the list. */}
      <BackButton referrer={backReferrer} />

      {/* Der Entitätskopf ist die Kopfkarte der Seite. Der Zeitraum ist eine
          Wertauswahl und steht deshalb als Aktion in der Titelzeile. */}
      <TenantPage
        leading={<ConceptIconTile concept="feedback" variant="page" />}
        title={student?.name ?? "Feedbackhistorie"}
        stats={studentMeta || feedbackLine}
        statsLoading={loading}
        loading={loading}
        error={errorMessage}
        empty={
          featureDisabled
            ? {
                title: "Feedbackhistorie ist ausgeschaltet",
                description: FEATURE_DISABLED_DESCRIPTION,
              }
            : !loading && errorMessage === null && totalFeedback === 0
              ? {
                  title:
                    "Kein Feedback für den ausgewählten Zeitraum vorhanden.",
                  description: "Wählen Sie einen anderen Zeitraum.",
                }
              : null
        }
        actions={
          <SegmentedControl
            items={timeRangeOptions}
            value={timeRange}
            onChange={setTimeRange}
            ariaLabel="Zeitraum wählen"
          />
        }
      >
        <SectionCard
          title="Feedback-Übersicht"
          description={FEEDBACK_HISTORY_DESCRIPTION}
          leading={<ConceptIconTile concept="feedback" variant="section" />}
        >
          <>
            {/* Proportion bar */}
            <div className="mt-4 flex h-3 overflow-hidden rounded-full">
              {positiveFeedbackCount > 0 && (
                <div
                  className="transition-[width] duration-300"
                  style={{
                    backgroundColor: feedbackToneColors.positive,
                    width: `${(positiveFeedbackCount / totalFeedback) * 100}%`,
                  }}
                />
              )}
              {neutralFeedbackCount > 0 && (
                <div
                  className="transition-[width] duration-300"
                  style={{
                    backgroundColor: feedbackToneColors.neutral,
                    width: `${(neutralFeedbackCount / totalFeedback) * 100}%`,
                  }}
                />
              )}
              {negativeFeedbackCount > 0 && (
                <div
                  className="transition-[width] duration-300"
                  style={{
                    backgroundColor: feedbackToneColors.negative,
                    width: `${(negativeFeedbackCount / totalFeedback) * 100}%`,
                  }}
                />
              )}
            </div>

            {/* Inline stats */}
            <div className="mt-3 flex flex-wrap gap-x-5 gap-y-1 text-sm">
              <span className="flex items-center gap-1.5">
                <span
                  className="inline-block h-2.5 w-2.5 rounded-full"
                  style={{ backgroundColor: feedbackToneColors.positive }}
                />
                <span className="font-medium text-gray-900">
                  {positiveFeedbackCount}
                </span>
                <span className="text-gray-500">Positiv</span>
              </span>
              <span className="flex items-center gap-1.5">
                <span
                  className="inline-block h-2.5 w-2.5 rounded-full"
                  style={{ backgroundColor: feedbackToneColors.neutral }}
                />
                <span className="font-medium text-gray-900">
                  {neutralFeedbackCount}
                </span>
                <span className="text-gray-500">Neutral</span>
              </span>
              <span className="flex items-center gap-1.5">
                <span
                  className="inline-block h-2.5 w-2.5 rounded-full"
                  style={{ backgroundColor: feedbackToneColors.negative }}
                />
                <span className="font-medium text-gray-900">
                  {negativeFeedbackCount}
                </span>
                <span className="text-gray-500">Negativ</span>
              </span>
            </div>

            {/* Stacked bar chart */}
            {chartData.length > 1 && (
              <div className="mt-6">
                <FeedbackHistoryChart data={chartData} />
              </div>
            )}
          </>

          {/* Expandable detail list; sie läuft randlos bis zur Kartenkante. */}
          {totalFeedback > 0 && (
            <div className="-mx-5 mt-4 -mb-5">
              <button
                type="button"
                onClick={() => setShowDetails((prev) => !prev)}
                className="flex w-full items-center justify-center gap-2 border-t border-gray-100 px-4 py-3 text-sm font-medium text-gray-500 transition-colors hover:bg-gray-50 hover:text-gray-700"
              >
                {showDetails ? (
                  <>
                    Einträge ausblenden
                    <ChevronUp className="h-4 w-4" />
                  </>
                ) : (
                  <>
                    Alle Einträge anzeigen ({totalFeedback})
                    <ChevronDown className="h-4 w-4" />
                  </>
                )}
              </button>

              {showDetails && (
                <div className="border-t border-gray-100">
                  {sortedDates.map((dateString) => {
                    const feedbackForDate =
                      groupedFeedbackHistory[dateString] ?? [];
                    const dateObj = new Date(
                      feedbackForDate[0]?.timestamp ?? dateString,
                    );

                    return (
                      <div key={dateString}>
                        <div className="border-b border-gray-50 bg-gray-50/50 px-4 py-2 sm:px-6">
                          <h3 className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                            {dateObj.toLocaleDateString("de-DE", {
                              timeZone: "Europe/Berlin",
                              weekday: "long",
                              day: "numeric",
                              month: "long",
                              year: "numeric",
                            })}
                          </h3>
                        </div>
                        <div className="divide-y divide-gray-50 px-4 sm:px-6">
                          {feedbackForDate.map((feedback) => (
                            <div
                              key={feedback.id}
                              className="flex items-center gap-3 py-2.5"
                              data-testid={`feedback-indicator-${feedback.feedback_type}`}
                            >
                              <span
                                className="inline-block h-2.5 w-2.5 flex-shrink-0 rounded-full"
                                style={{
                                  backgroundColor:
                                    feedbackToneColors[feedback.feedback_type],
                                }}
                              />
                              <span className="text-sm text-gray-900">
                                {feedbackTypeLabels[feedback.feedback_type]}
                              </span>
                              <span className="text-xs text-gray-400">
                                {formatTime(feedback.timestamp)}
                              </span>
                              {feedback.is_mensa_feedback && (
                                <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-500">
                                  Mensa
                                </span>
                              )}
                            </div>
                          ))}
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          )}
        </SectionCard>
      </TenantPage>
    </>
  );
}
