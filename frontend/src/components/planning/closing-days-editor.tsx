"use client";

/**
 * ClosingDaysEditor is the admin list for OGS-Schließtage (#1418 3b) —
 * closure ranges (pädagogische Tage, Weihnachtswoche, Sommerschließung) the
 * school defines on top of the gesetzliche Feiertage. Rendered as a second
 * section on the Kalenderzeiträume page.
 */

import { useCallback, useEffect, useMemo, useState } from "react";
import { Plus } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";

import { ClosingDayModal } from "~/components/planning/closing-day-modal";
import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { ConfirmationModal } from "~/components/ui/modal";
import { closingDayService } from "~/lib/closing-day-api";
import {
  type ClosingDay,
  formatClosingDayRange,
} from "~/lib/closing-day-helpers";
import { useToast } from "~/contexts/ToastContext";
import { useInvalidateClosingDays } from "~/lib/hooks/use-closing-days";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ClosingDaysEditor" });

export function ClosingDaysEditor() {
  const [closingDays, setClosingDays] = useState<ClosingDay[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<ClosingDay | null>(null);
  const [deleting, setDeleting] = useState<ClosingDay | null>(null);
  const [deleteLoading, setDeleteLoading] = useState(false);
  const { success: toastSuccess, error: toastError } = useToast();
  const invalidateClosingDays = useInvalidateClosingDays();

  const load = useCallback(async (opts?: { silent?: boolean }) => {
    if (!opts?.silent) setLoading(true);
    setError(null);
    try {
      const data = await closingDayService.list();
      setClosingDays(data);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("closing_days_load_failed", { error: message });
      setError(message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const refreshAfterMutation = useCallback(() => {
    void invalidateClosingDays().catch((err: unknown) => {
      logger.warn("closing_days_cache_invalidation_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
    });
    void load({ silent: true });
  }, [invalidateClosingDays, load]);

  const beginCreate = () => {
    setEditing(null);
    setModalOpen(true);
  };

  const beginEdit = useCallback((day: ClosingDay) => {
    setEditing(day);
    setModalOpen(true);
  }, []);

  const handleDelete = useCallback(async () => {
    if (!deleting) return;
    setDeleteLoading(true);
    try {
      await closingDayService.delete(deleting.id);
      toastSuccess(`Schließtag "${deleting.reason}" gelöscht`);
      setDeleting(null);
      refreshAfterMutation();
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Schließtag konnte nicht gelöscht werden";
      logger.error("closing_day_delete_failed", {
        closing_day_id: deleting.id,
        error: message,
      });
      toastError(message);
    } finally {
      setDeleteLoading(false);
    }
  }, [deleting, refreshAfterMutation, toastSuccess, toastError]);

  const columns = useMemo<DataTableColumn<ClosingDay>[]>(
    () => [
      {
        key: "range",
        header: "Von – Bis",
        className: "hidden sm:table-cell",
        headerClassName: "hidden sm:table-cell",
        sortValue: (day) => day.startDate,
        render: (day) => (
          <span className="text-sm whitespace-nowrap text-gray-600">
            {formatClosingDayRange(day)}
          </span>
        ),
      },
      {
        key: "reason",
        header: "Grund",
        sortValue: (day) => day.reason,
        // Der Zeitraum rutscht auf schmalen Screens als Unterzeile hierher,
        // weil die eigene Spalte dort ausgeblendet ist (#2033).
        render: (day) => (
          // Wie im CalendarPeriodsEditor: kein fester max-w, sondern
          // Restbreite neben der schmalen Aktionsspalte plus umbrechender
          // Text — so passt die Zeile auch auf 320px ohne Seitwärts-Scroll.
          <div className="min-w-0">
            {/* wrap-anywhere statt break-words: nur damit sinkt die
                Mindestbreite der Spalte unter die Länge eines langen
                Wortes wie "Weihnachtsschließung". */}
            <p className="font-medium wrap-anywhere text-gray-900">
              {day.reason}
            </p>
            <p className="mt-0.5 text-xs leading-5 break-words text-gray-500 sm:hidden">
              {formatClosingDayRange(day)}
            </p>
          </div>
        ),
      },
      {
        key: "actions",
        header: "",
        align: "right",
        // w-px: die Aktionsspalte bekommt nur ihre Mindestbreite, die
        // Grund-Spalte den Rest der Tabellenbreite (#2033).
        className: "w-px",
        headerClassName: "w-px",
        render: (day) => (
          // Auf schmalen Screens stapeln sich die beiden Aktionen, sonst
          // passen Grund-Spalte und Buttons nicht nebeneinander (#2033).
          <div className="flex flex-col items-end justify-end gap-1 sm:flex-row sm:items-center">
            <Button
              type="button"
              variant="ghost"
              size="compact"
              onClick={() => beginEdit(day)}
            >
              Bearbeiten
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="compact"
              className="text-moto-red hover:text-moto-red-strong"
              onClick={() => setDeleting(day)}
            >
              Löschen
            </Button>
          </div>
        ),
      },
    ],
    [beginEdit],
  );

  if (loading) {
    return (
      <div className="moto-content-surface rounded-2xl border px-5 py-10 text-center text-sm text-gray-500 shadow-sm">
        Schließtage werden geladen...
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {error && (
        <div
          className="border-moto-red/20 bg-moto-red/10 text-moto-red-strong rounded-2xl border p-4 text-sm"
          role="alert"
          aria-live="polite"
        >
          {error}
        </div>
      )}

      <section className="moto-content-surface rounded-2xl border p-4 shadow-sm backdrop-blur-md">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-sm text-gray-600">
            OGS-Schließtage (z. B. pädagogische Tage, Weihnachtswoche,
            Sommerschließung). An diesen Tagen gilt Soll = 0 wie an gesetzlichen
            Feiertagen.
          </p>
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={beginCreate}
            className="shrink-0 gap-2"
          >
            <Plus className="h-4 w-4" aria-hidden="true" />
            Schließtag anlegen
          </Button>
        </div>
      </section>

      {closingDays.length === 0 ? (
        <section className="moto-content-surface rounded-2xl border px-6 py-12 text-center shadow-sm backdrop-blur-md">
          <div className="bg-moto-green/10 mx-auto flex h-12 w-12 items-center justify-center rounded-2xl text-[#669f21]">
            <MotoConceptIcon concept="closingDays" size={28} />
          </div>
          <h2 className="mt-4 text-base font-semibold text-gray-900">
            Noch keine Schließtage
          </h2>
          <p className="mx-auto mt-2 max-w-md text-sm leading-6 text-gray-600">
            Hinterlege die Schließtage des Schuljahres, damit die Zeiterfassung
            an diesen Tagen kein Soll ansetzt.
          </p>
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={beginCreate}
            className="mt-5 gap-2"
          >
            <Plus className="h-4 w-4" aria-hidden="true" />
            Schließtag anlegen
          </Button>
        </section>
      ) : (
        <DataTable
          columns={columns}
          rows={closingDays}
          getRowKey={(day) => day.id}
          defaultSortKey="range"
          defaultSortDirection="asc"
        />
      )}

      <ClosingDayModal
        isOpen={modalOpen}
        onClose={() => setModalOpen(false)}
        onSaved={refreshAfterMutation}
        initial={editing}
      />

      <ConfirmationModal
        isOpen={deleting !== null}
        onClose={() => setDeleting(null)}
        onConfirm={() => void handleDelete()}
        title="Schließtag löschen"
        confirmText="Löschen"
        isConfirmLoading={deleteLoading}
        confirmButtonClass="bg-moto-red hover:bg-moto-red-strong"
      >
        {deleting && (
          <p>
            Soll der Schließtag &quot;{deleting.reason}&quot; (
            {formatClosingDayRange(deleting)}) wirklich gelöscht werden? Das
            Soll dieser Tage wird danach wieder regulär berechnet.
          </p>
        )}
      </ConfirmationModal>
    </div>
  );
}
