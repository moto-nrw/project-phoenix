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
import { Download, FileSpreadsheet, Utensils } from "lucide-react";

import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { ISODatePicker } from "~/components/ui/date-picker";
import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { MealRegistrationChangeModal } from "~/components/meal-plan/meal-registration-change-modal";
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

/**
 * Die Tagesliste für die Küche. Mit `canRegister` (meals:register) lädt sie
 * auch die Kinder, die nicht essen, und bietet „Anmeldung ändern“ an.
 */
export function MealParticipantList({
  canRegister,
  canExport = true,
}: Readonly<{ canRegister: boolean; canExport?: boolean }>) {
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
  const [deadline, setDeadline] = useState("");
  const [closed, setClosed] = useState(false);
  const [participants, setParticipants] = useState<DailyMealParticipant[]>([]);
  // Die übrigen Kinder des Tages, für Änderungen durch die Schule (#3873).
  const [others, setOthers] = useState<DailyMealParticipant[]>([]);
  const [changing, setChanging] = useState(false);
  const [loading, setLoading] = useState(true);
  const [exporting, setExporting] = useState<"pdf" | "xlsx" | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    clearLoadError();
    try {
      const list = await getDailyMealParticipants(date, canRegister);
      setParticipants(list.participants);
      setOthers(list.others);
      setDeadline(list.deadline);
      setClosed(list.closed);
    } catch (cause) {
      logger.error("meal_participant_list_load_failed", {
        error: cause instanceof Error ? cause.message : String(cause),
      });
      setParticipants([]);
      setOthers([]);
      void showLoadError(cause, {
        object: t("errorObject"),
        retry: () => reloadRef.current(),
      });
    } finally {
      setLoading(false);
    }
  }, [canRegister, date, t, showLoadError, clearLoadError]);
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

  const cutoffLine = closed
    ? t("closedDay")
    : deadline
      ? t("cutoff", {
          date: new Date(deadline).toLocaleString(locale, {
            weekday: "short",
            day: "2-digit",
            month: "2-digit",
            hour: "2-digit",
            minute: "2-digit",
            timeZone: "Europe/Berlin",
          }),
        })
      : null;

  // Eine Karte wie „Bestellung“ darüber: Titel, Frist und Aktionen im Kopf,
  // die Liste der Kinder als Körper (#3873).
  return (
    <SectionCard
      title={t("title")}
      description={
        <>
          <span className="block">{t("description")}</span>
          {cutoffLine ? (
            <span className="mt-1 block font-medium text-gray-700">
              {cutoffLine}
            </span>
          ) : null}
        </>
      }
      actions={
        <div className="flex w-full flex-col gap-2 sm:w-auto sm:flex-row sm:flex-wrap sm:items-center sm:justify-end">
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

          {canRegister ? (
            <Button
              type="button"
              variant="outline"
              size="md"
              className="bg-white"
              onClick={() => setChanging(true)}
              disabled={loading || Boolean(loadError) || closed || date < today}
            >
              {t("change")}
            </Button>
          ) : null}

          {canExport ? (
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
                disabled={loading || Boolean(loadError) || exporting !== null}
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
                disabled={loading || Boolean(loadError) || exporting !== null}
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
          ) : null}
        </div>
      }
    >
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
      <MealRegistrationChangeModal
        isOpen={canRegister && changing}
        onClose={() => setChanging(false)}
        onSaved={() => void load()}
        date={date}
        participants={participants}
        others={others}
      />
    </SectionCard>
  );
}
