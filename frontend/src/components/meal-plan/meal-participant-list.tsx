"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { Download, FileSpreadsheet, Utensils } from "lucide-react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { ISODatePicker } from "~/components/ui/date-picker";
import { EmptyState } from "~/components/ui/empty-state";
import { useToast } from "~/contexts/ToastContext";
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
  const toast = useToast();
  const [selectedDate, setSelectedDate] = useState<string | null>(null);
  const date = selectedDate ?? nextWeekday(today);
  const [cutoffTime, setCutoffTime] = useState("");
  const [participants, setParticipants] = useState<DailyMealParticipant[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [exporting, setExporting] = useState<"pdf" | "xlsx" | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(false);
    try {
      const list = await getDailyMealParticipants(date);
      setParticipants(list.participants);
      setCutoffTime(list.cutoffTime);
    } catch (loadError) {
      logger.error("meal_participant_list_load_failed", {
        error:
          loadError instanceof Error ? loadError.message : String(loadError),
      });
      setParticipants([]);
      setError(true);
    } finally {
      setLoading(false);
    }
  }, [date]);

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
      toast.error(t("downloadError"));
    } finally {
      setExporting(null);
    }
  }

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

            <div
              role="group"
              aria-label={t("downloadGroup")}
              className="flex flex-wrap gap-2 sm:justify-end"
            >
              <Button
                type="button"
                variant="outline"
                size="md"
                className="gap-2 bg-white"
                aria-label={t("downloadPdf")}
                aria-busy={exporting === "pdf"}
                onClick={() => void download("pdf")}
                isLoading={exporting === "pdf"}
                loadingText={t("downloadLoading")}
                disabled={loading || error || exporting !== null}
              >
                <Download className="h-4 w-4" aria-hidden="true" />
                {t("pdf")}
              </Button>
              <Button
                type="button"
                variant="outline"
                size="md"
                className="gap-2 bg-white"
                aria-label={t("downloadExcel")}
                aria-busy={exporting === "xlsx"}
                onClick={() => void download("xlsx")}
                isLoading={exporting === "xlsx"}
                loadingText={t("downloadLoading")}
                disabled={loading || error || exporting !== null}
              >
                <FileSpreadsheet className="h-4 w-4" aria-hidden="true" />
                {t("excel")}
              </Button>
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

      {error ? (
        <div className="space-y-3">
          <Alert
            type="error"
            title={t("loadErrorTitle")}
            message={t("loadErrorMessage")}
          />
          <Button type="button" variant="outline" onClick={() => void load()}>
            {t("retry")}
          </Button>
        </div>
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
