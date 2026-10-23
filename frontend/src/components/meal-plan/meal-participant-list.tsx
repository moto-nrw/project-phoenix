"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useLocale, useTranslations } from "next-intl";
import { Utensils } from "lucide-react";

import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { ISODatePicker } from "~/components/ui/date-picker";
import { EmptyState } from "~/components/ui/empty-state";
import { ExportMenu } from "~/components/ui/export-menu";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { useApiErrorDisplay, useApiLoadError } from "~/contexts/ToastContext";
import { formatDate, parseISODate, toISODate } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { useLocalizedDatePicker } from "~/lib/hooks/use-localized-date-picker";
import {
  downloadDailyMealParticipants,
  getDailyMealParticipants,
  type DailyMealParticipant,
} from "~/lib/meal-plan-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "MealParticipantList" });

const isWeekendDay = (date: Date) => date.getDay() === 0 || date.getDay() === 6;

function nextWeekday(date: string): string {
  const candidate = parseISODate(date);
  while (isWeekendDay(candidate)) {
    candidate.setDate(candidate.getDate() + 1);
  }
  return toISODate(candidate);
}

export function MealParticipantList() {
  const t = useTranslations("mealParticipantList");
  const locale = useLocale();
  const datePicker = useLocalizedDatePicker();
  const today = useBerlinToday();
  const { show: showActionError } = useApiErrorDisplay();
  const {
    error: loadError,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();
  // „Wiederholen“ lädt bzw. exportiert mit dem aktuell gewählten Tag.
  const reloadRef = useRef<() => void>(() => undefined);
  const retryDownloadRef = useRef<(format: "pdf" | "xlsx") => void>(
    () => undefined,
  );
  const [selectedDate, setSelectedDate] = useState<string | null>(null);
  const date = selectedDate ?? nextWeekday(today);
  const [cutoffTime, setCutoffTime] = useState("");
  const [participants, setParticipants] = useState<DailyMealParticipant[]>([]);
  const [loading, setLoading] = useState(true);
  const [exporting, setExporting] = useState<"pdf" | "xlsx" | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    clearLoadError();
    try {
      const list = await getDailyMealParticipants(date);
      setParticipants(list.participants);
      setCutoffTime(list.cutoffTime);
    } catch (cause) {
      logger.error("meal_participant_list_load_failed", {
        error: cause instanceof Error ? cause.message : String(cause),
      });
      setParticipants([]);
      void showLoadError(cause, {
        object: t("errorObject"),
        retry: () => reloadRef.current(),
      });
    } finally {
      setLoading(false);
    }
  }, [date, t, showLoadError, clearLoadError]);
  useLayoutEffect(() => {
    reloadRef.current = () => void load();
  });

  useEffect(() => {
    void load();
  }, [load]);

  const columns = useMemo<DataTableColumn<DailyMealParticipant>[]>(
    () => [
      {
        key: "name",
        header: t("child"),
        render: (row) => `${row.lastName}, ${row.firstName}`,
        sortValue: (row) => `${row.lastName} ${row.firstName}`,
        stacked: "title",
      },
      {
        key: "class",
        header: t("class"),
        render: (row) => row.schoolClass || "–",
        sortValue: (row) => row.schoolClass,
        stacked: "meta",
      },
    ],
    [t],
  );

  async function download(format: "pdf" | "xlsx") {
    setExporting(format);
    try {
      await downloadDailyMealParticipants(date, format);
    } catch (downloadError) {
      logger.error("meal_participant_list_export_failed", {
        error:
          downloadError instanceof Error
            ? downloadError.message
            : String(downloadError),
      });
      void showActionError(downloadError, {
        object: t("errorObject"),
        retry: () => retryDownloadRef.current(format),
      });
    } finally {
      setExporting(null);
    }
  }
  useLayoutEffect(() => {
    retryDownloadRef.current = (format) => void download(format);
  });

  return (
    <section className="space-y-4" aria-labelledby="meal-participants-title">
      <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <h2
              id="meal-participants-title"
              className="text-lg font-semibold text-gray-900"
            >
              {t("title")}
            </h2>
            <p className="mt-1 text-sm text-gray-600">{t("description")}</p>
            {cutoffTime ? (
              <p className="mt-1 text-sm font-medium text-gray-700">
                {t("cutoff", { time: cutoffTime })}
              </p>
            ) : null}
          </div>

          <div className="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center sm:justify-end lg:shrink-0">
            <ISODatePicker
              id="meal-participant-date"
              value={date}
              onChange={setSelectedDate}
              ariaLabel={t("dateAria", {
                date: formatDate(date, false, locale),
              })}
              {...datePicker}
              calendarLayout="popover-below"
              controlSize="md"
              disabledDay={isWeekendDay}
              hideClearButton
              required
              className="w-full sm:w-44"
              triggerClassName="w-full min-w-44 flex-none justify-between rounded-lg text-sm font-medium"
            />

            <div className="flex flex-wrap gap-2 sm:justify-end">
              <ExportMenu
                label={t("downloadGroup")}
                options={[
                  { key: "pdf", format: "pdf", label: t("pdf") },
                  { key: "xlsx", format: "xlsx", label: t("excel") },
                ]}
                busyKey={exporting}
                unavailable={loading || Boolean(loadError)}
                onSelect={(option) => {
                  if (option.format === "pdf" || option.format === "xlsx")
                    void download(option.format);
                }}
              />
              <span className="sr-only" role="status" aria-live="polite">
                {exporting === "pdf"
                  ? t("pdfDownloadStatus")
                  : exporting === "xlsx"
                    ? t("excelDownloadStatus")
                    : ""}
              </span>
            </div>
          </div>
        </div>
      </div>

      {loadError ? (
        <LoadErrorAlert error={loadError} />
      ) : (
        <DataTable
          columns={columns}
          rows={participants}
          getRowKey={(row) => row.studentId}
          isLoading={loading}
          stackedOnMobile
          caption={t("caption", {
            date: formatDate(date, false, locale),
          })}
          emptyState={
            <EmptyState
              icon={<Utensils className="h-6 w-6" />}
              title={t("emptyTitle")}
              description={t("emptyDescription")}
            />
          }
        />
      )}
    </section>
  );
}
