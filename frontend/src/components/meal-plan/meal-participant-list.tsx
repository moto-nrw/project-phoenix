"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Download, Utensils } from "lucide-react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { Input } from "~/components/ui/input";
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
        <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
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
          <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
            <Input
              id="meal-participant-date"
              type="date"
              label="Tag"
              value={date}
              onChange={(event) => setDate(event.target.value)}
              controlSize="compact"
            />
            <div className="flex gap-2">
              <Button
                type="button"
                variant="outline"
                onClick={() => void download("pdf")}
                isLoading={exporting === "pdf"}
                disabled={loading || error || exporting !== null}
              >
                <Download className="mr-2 h-4 w-4" />
                PDF
              </Button>
              <Button
                type="button"
                variant="outline"
                onClick={() => void download("xlsx")}
                isLoading={exporting === "xlsx"}
                disabled={loading || error || exporting !== null}
              >
                <Download className="mr-2 h-4 w-4" />
                Excel
              </Button>
            </div>
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
