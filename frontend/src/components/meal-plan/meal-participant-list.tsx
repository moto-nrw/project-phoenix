"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Download, FileSpreadsheet, Utensils } from "lucide-react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { ISODatePicker } from "~/components/ui/date-picker";
import { EmptyState } from "~/components/ui/empty-state";
import { useToast } from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import {
  downloadDailyMealParticipants,
  getDailyMealParticipants,
  type DailyMealParticipant,
} from "~/lib/meal-plan-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "MealParticipantList" });

export function MealParticipantList() {
  const today = useBerlinToday();
  const toast = useToast();
  const [date, setDate] = useState(today);
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
        header: "Kind",
        render: (row) => `${row.lastName}, ${row.firstName}`,
        sortValue: (row) => `${row.lastName} ${row.firstName}`,
        stacked: "title",
      },
      {
        key: "class",
        header: "Klasse",
        render: (row) => row.schoolClass || "–",
        sortValue: (row) => row.schoolClass,
        stacked: "meta",
      },
    ],
    [],
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
      toast.error("Die Tagesliste konnte nicht heruntergeladen werden.");
    } finally {
      setExporting(null);
    }
  }

  return (
    <section className="space-y-4" aria-labelledby="meal-participants-title">
      <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
        <div>
          <h2
            id="meal-participants-title"
            className="text-lg font-semibold text-gray-900"
          >
            Tagesliste für die Küche
          </h2>
          <p className="mt-1 text-sm text-gray-600">
            Die Liste zeigt alle Kinder, die an diesem Tag zum Mittagessen
            angemeldet sind.
          </p>
          {cutoffTime ? (
            <p className="mt-1 text-sm font-medium text-gray-700">
              Änderungen für diesen Tag sind bis {cutoffTime} Uhr möglich.
            </p>
          ) : null}
        </div>

        <div className="mt-4 flex flex-col gap-3 border-t border-gray-100 pt-4 sm:flex-row sm:flex-wrap sm:items-center">
          <div className="flex items-center gap-2">
            <span className="text-sm font-medium text-gray-700">Datum</span>
            <ISODatePicker
              id="meal-participant-date"
              value={date}
              onChange={setDate}
              ariaLabel={`Datum: ${formatDate(date)}`}
              calendarLayout="popover"
              controlSize="md"
              hideClearButton
              required
              className="min-w-0 flex-1 sm:w-44 sm:flex-none"
            />
          </div>

          <div
            role="group"
            aria-labelledby="meal-participant-export-label"
            className="flex flex-col items-start gap-2 sm:ms-auto sm:flex-row sm:items-center"
          >
            <span
              id="meal-participant-export-label"
              className="text-sm font-medium text-gray-700"
            >
              Export
            </span>
            <div className="flex gap-2">
              <Button
                type="button"
                variant="primary"
                size="sm"
                className="gap-2"
                aria-busy={exporting === "pdf"}
                onClick={() => void download("pdf")}
                isLoading={exporting === "pdf"}
                loadingText="PDF wird erstellt…"
                disabled={loading || error || exporting !== null}
              >
                <Download className="h-4 w-4" aria-hidden="true" />
                PDF
              </Button>
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="gap-2 bg-white"
                aria-busy={exporting === "xlsx"}
                onClick={() => void download("xlsx")}
                isLoading={exporting === "xlsx"}
                loadingText="Excel wird erstellt…"
                disabled={loading || error || exporting !== null}
              >
                <FileSpreadsheet className="h-4 w-4" aria-hidden="true" />
                Excel
              </Button>
            </div>
            <span className="sr-only" role="status" aria-live="polite">
              {exporting === "pdf"
                ? "PDF wird erstellt."
                : exporting === "xlsx"
                  ? "Excel wird erstellt."
                  : ""}
            </span>
          </div>
        </div>
      </div>

      {error ? (
        <div className="space-y-3">
          <Alert
            type="error"
            title="Tagesliste nicht geladen"
            message="Bitte versuchen Sie es erneut."
          />
          <Button type="button" variant="outline" onClick={() => void load()}>
            Erneut versuchen
          </Button>
        </div>
      ) : (
        <DataTable
          columns={columns}
          rows={participants}
          getRowKey={(row) => row.studentId}
          isLoading={loading}
          stackedOnMobile
          caption={`Mittagessen am ${formatDate(date)}`}
          emptyState={
            <EmptyState
              icon={<Utensils className="h-6 w-6" />}
              title="Keine Anmeldungen für diesen Tag"
              description="Für diesen Tag steht kein Kind auf der Küchenliste."
            />
          }
        />
      )}
    </section>
  );
}
