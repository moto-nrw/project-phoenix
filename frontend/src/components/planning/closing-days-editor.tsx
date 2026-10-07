"use client";

/**
 * ClosingDaysEditor is the admin list for OGS-Schließtage (#1418 3b) —
 * closure ranges (pädagogische Tage, Weihnachtswoche, Sommerschließung) the
 * school defines on top of the gesetzliche Feiertage. Rendered as a second
 * section on the Kalenderzeiträume page.
 */

import {
  type ReactNode,
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { CalendarX, Pencil, Plus, Trash2 } from "lucide-react";
import { useSession } from "next-auth/react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";

import { ClosingDayModal } from "~/components/planning/closing-day-modal";
import { BulkCancelAppointmentsModal } from "~/components/timetable/bulk-cancel-appointments-modal";
import { Button } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { formErrorMessage } from "~/components/ui/form-error";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import { closingDayService } from "~/lib/closing-day-api";
import {
  type ClosingDay,
  formatClosingDayRange,
} from "~/lib/closing-day-helpers";
import {
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { hasPermission } from "~/lib/auth-utils";
import { useInvalidateClosingDays } from "~/lib/hooks/use-closing-days";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ClosingDaysEditor" });

export function ClosingDaysEditor({
  onAppointmentsCancelled,
}: {
  /** Nach „Termine absagen“: die Verwendung der Zeiträume neu laden. */
  readonly onAppointmentsCancelled?: () => void;
} = {}) {
  const [closingDays, setClosingDays] = useState<ClosingDay[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadFailed, setLoadFailed] = useState(false);
  const loadError = useApiLoadError();
  const showLoadError = loadError.show;
  const clearLoadError = loadError.clear;
  // „Wiederholen“ lädt bzw. löscht mit dem dann aktuellen Stand.
  const latestLoadRef = useRef<() => void>(() => undefined);
  const latestDeleteRef = useRef<() => void>(() => undefined);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<ClosingDay | null>(null);
  const [deleting, setDeleting] = useState<ClosingDay | null>(null);
  const [deleteLoading, setDeleteLoading] = useState(false);
  // Fehler beim Löschen bleiben im offenen Bestätigungsdialog.
  const deleteErrors = useApiFormError();
  // #3594: Termine eines Schließtags absagen, aus der Zeile oder direkt nach
  // dem Speichern. Nur mit Planungsrecht, wie der Betreuungsplan selbst.
  const [cancelRange, setCancelRange] = useState<{
    startDate: string;
    endDate: string;
    afterSave?: boolean;
  } | null>(null);
  const { data: session } = useSession();
  const canManageSchedules = hasPermission(session, "schedules:manage");
  const { success: toastSuccess } = useToast();
  const invalidateClosingDays = useInvalidateClosingDays();

  const load = useCallback(
    async (opts?: { silent?: boolean }) => {
      if (!opts?.silent) setLoading(true);
      try {
        const data = await closingDayService.list();
        setClosingDays(data);
        setLoadFailed(false);
        clearLoadError();
      } catch (err) {
        logger.error("closing_days_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setLoadFailed(true);
        void showLoadError(err, {
          object: "die Liste der Schließtage",
          retry: () => latestLoadRef.current(),
        });
      } finally {
        setLoading(false);
      }
    },
    [showLoadError, clearLoadError],
  );
  useLayoutEffect(() => {
    latestLoadRef.current = () => void load();
  });

  useEffect(() => {
    void load();
  }, [load]);

  const refreshAfterMutation = useCallback(() => {
    // Bewusst still: gespeichert ist der Schließtag schon. Scheitert nur das
    // Auffrischen der Plan-Zwischenspeicher, laden Dienst- und Betreuungsplan
    // ihn beim nächsten Öffnen ohnehin neu.
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

  const showDeleteError = deleteErrors.show;
  const clearDeleteError = deleteErrors.clear;
  const handleDelete = useCallback(async () => {
    if (!deleting) return;
    setDeleteLoading(true);
    clearDeleteError();
    try {
      await closingDayService.delete(deleting.id);
      toastSuccess(`Der Schließtag „${deleting.reason}“ ist gelöscht.`);
      setDeleting(null);
      refreshAfterMutation();
    } catch (err) {
      logger.error("closing_day_delete_failed", {
        closing_day_id: deleting.id,
        error: err instanceof Error ? err.message : String(err),
      });
      await showDeleteError(err, {
        object: "das Löschen des Schließtags",
        retry: () => latestDeleteRef.current(),
      });
    } finally {
      setDeleteLoading(false);
    }
  }, [
    deleting,
    refreshAfterMutation,
    toastSuccess,
    showDeleteError,
    clearDeleteError,
  ]);
  useLayoutEffect(() => {
    latestDeleteRef.current = () => void handleDelete();
  });

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
          // Zeilenaktionen nur im Kebab (BAUARTEN-SPEC Bauart 1 Regel 4);
          // ein Auslöser passt auch auf 320px neben die Grund-Spalte.
          <div className="flex justify-end">
            <OverflowMenu
              ariaLabel={`Aktionen für ${day.reason}`}
              items={[
                {
                  label: "Bearbeiten",
                  icon: <Pencil className="h-4 w-4" aria-hidden />,
                  onClick: () => beginEdit(day),
                },
                ...(canManageSchedules
                  ? [
                      {
                        label: "Termine absagen",
                        icon: <CalendarX className="h-4 w-4" aria-hidden />,
                        onClick: () =>
                          setCancelRange({
                            startDate: day.startDate,
                            endDate: day.endDate,
                          }),
                      },
                    ]
                  : []),
                { kind: "separator" },
                {
                  label: "Löschen",
                  icon: <Trash2 className="h-4 w-4" aria-hidden />,
                  destructive: true,
                  onClick: () => setDeleting(day),
                },
              ]}
            />
          </div>
        ),
      },
    ],
    [beginEdit, canManageSchedules],
  );

  // Der Katalogtext des Ladefehlers kommt asynchron. Bis dahin steht das
  // Ladeskelett, nie der Leerzustand „Noch keine Schließtage“.
  const awaitingErrorText =
    loadFailed && formErrorMessage(loadError.error) === null;
  // Ohne je geladene Liste steht nur der Fehler da, keine leere Tabelle.
  const failedWithoutData = loadFailed && closingDays.length === 0;
  let body: ReactNode = null;
  if (!loading && !loadFailed && closingDays.length === 0) {
    body = (
      <EmptyState
        icon={
          <span className="flex h-12 w-12 items-center justify-center rounded-2xl bg-gray-100">
            <MotoConceptIcon concept="closingDays" size={28} />
          </span>
        }
        title="Noch keine Schließtage"
        description="Hinterlegen Sie die Schließtage des Schuljahres, damit die Zeiterfassung an diesen Tagen kein Soll ansetzt."
      />
    );
  } else if (!failedWithoutData || awaitingErrorText) {
    body = (
      <DataTable
        columns={columns}
        rows={closingDays}
        getRowKey={(day) => day.id}
        defaultSortKey="range"
        defaultSortDirection="asc"
        isLoading={loading || failedWithoutData}
      />
    );
  }

  return (
    <div className="space-y-4">
      {/* Erklärtext und „Schließtag anlegen“ sitzen im Kartenkopf, statt als
          freier Absatz und eigene Buttonzeile über der Tabelle zu stehen. */}
      <SectionCard
        title="Schließtage"
        description="Tage, an denen die OGS geschlossen hat, z. B. Ferien oder pädagogische Tage. Dann fallen Termine aus und Mitarbeitende haben kein Soll. Ausnahmen: Serien mit „Auch an Schließtagen planen“ und Sonderarbeitszeiten."
        actions={
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
        }
      >
        <LoadErrorAlert
          error={loadError.error}
          className={failedWithoutData ? undefined : "mb-4"}
        />
        {body}
      </SectionCard>

      <ClosingDayModal
        isOpen={modalOpen}
        onClose={() => setModalOpen(false)}
        onSaved={refreshAfterMutation}
        initial={editing}
        onOfferCancel={
          canManageSchedules
            ? (range) => setCancelRange({ ...range, afterSave: true })
            : undefined
        }
      />

      <BulkCancelAppointmentsModal
        isOpen={cancelRange !== null}
        initialFrom={cancelRange?.startDate ?? ""}
        initialTo={cancelRange?.endDate ?? ""}
        afterSave={cancelRange?.afterSave ?? false}
        onClose={() => setCancelRange(null)}
        onCancelled={onAppointmentsCancelled}
      />

      <ConfirmDeleteModal
        isOpen={deleting !== null}
        title="Schließtag löschen"
        description={
          deleting ? (
            <p>
              Der Schließtag „{deleting.reason}“ (
              {formatClosingDayRange(deleting)}) wird gelöscht. Das Soll dieser
              Tage wird danach wieder regulär berechnet.
            </p>
          ) : null
        }
        gate={{ mode: "twoStep" }}
        onConfirm={handleDelete}
        onClose={() => {
          clearDeleteError();
          setDeleting(null);
        }}
        loading={deleteLoading}
        error={deleteErrors.error}
      />
    </div>
  );
}
