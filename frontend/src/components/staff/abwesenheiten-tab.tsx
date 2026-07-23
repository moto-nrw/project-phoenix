"use client";

import {
  type ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";
import { useSWRConfig } from "swr";
import { Thermometer, Trash2 } from "lucide-react";

import {
  DenyAbsenceModal,
  QuestionAbsenceModal,
} from "~/components/staff/absence-decision-modals";
import {
  SickReportModal,
  type SickReportStaff,
} from "~/components/staff/sick-report-modal";
import { AbsenceRequestRow } from "~/components/staff/absence-request-row";
import {
  ABSENCE_TYPE_HEX,
  ABSENCE_TYPE_LABEL,
  absenceStatusMeta,
  absenceTypeNoun as absenceTypeLabel,
  dayCountFor as sharedDayCountFor,
  dispatchAbsencesRefresh,
  formatAbsenceRange,
  formatDayCount,
} from "~/lib/absence-helpers";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { Button } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { Loading } from "~/components/ui/loading";
import { Modal } from "~/components/ui/modal";
import { StatusDotBadge } from "~/components/ui/status-dot-badge";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { todayISO } from "~/lib/date-helpers";
import {
  staffAbsenceService,
  type StaffAbsenceRow,
  type StaffVacationQuotaSummary,
} from "~/lib/staff-api";
import { useTenantMutateMatching } from "~/lib/swr";
import {
  VERTRETUNG_GAPS_KEY_PREFIX,
  VERTRETUNG_WEEK_KEY_PREFIX,
} from "~/lib/timetable-helpers";

const logger = createLogger({ component: "AbwesenheitenTab" });

const VACATION_WORKFLOW_STATUSES = new Set([
  "requested",
  "question",
  "approved",
  "declined",
  "canceled",
]);

function formatRange(start: string, end: string): string {
  return formatAbsenceRange(start, end);
}

function dayCountFor(row: StaffAbsenceRow): number {
  return sharedDayCountFor({
    workingDays: row.working_days,
    dateStart: row.date_start,
    dateEnd: row.date_end,
    halfDay: row.half_day,
    startHalfDay: row.start_half_day,
    endHalfDay: row.end_half_day,
    hasBoundaryFields:
      row.start_half_day !== undefined || row.end_half_day !== undefined,
  });
}

function statusMeta(status: string) {
  return absenceStatusMeta(status);
}

function pendingAbsences(absences: StaffAbsenceRow[]): StaffAbsenceRow[] {
  return absences.filter(
    (absence) =>
      absence.status === "requested" || absence.status === "question",
  );
}

function upcomingAbsences(absences: StaffAbsenceRow[]): StaffAbsenceRow[] {
  const today = todayISO();
  return absences.filter(
    (absence) =>
      (absence.status === "approved" || absence.status === "reported") &&
      absence.date_end >= today,
  );
}

function historicalAbsences(absences: StaffAbsenceRow[]): StaffAbsenceRow[] {
  const today = todayISO();
  return absences
    .filter(
      (absence) =>
        absence.status === "declined" ||
        absence.status === "canceled" ||
        ((absence.status === "approved" || absence.status === "reported") &&
          absence.date_end < today),
    )
    .sort((left, right) => (left.date_start < right.date_start ? 1 : -1));
}

export function isVacationWorkflowAbsence(row: StaffAbsenceRow): boolean {
  return (
    row.absence_type === "vacation" &&
    VACATION_WORKFLOW_STATUSES.has(row.status)
  );
}

function TabLoadingBoundary({
  loading,
  children,
}: {
  readonly loading: boolean;
  readonly children: ReactNode;
}) {
  if (loading) return <Loading fullPage={false} />;
  return children;
}

export function AbwesenheitenTab({
  staffId,
  canEdit,
  canManageSickReports,
  staff,
}: {
  readonly staffId: string;
  readonly canEdit: boolean;
  readonly canManageSickReports: boolean;
  // Passed in from the staff detail page so the "Krank melden" modal has the
  // person's name. Optional so the tab still renders (without the action)
  // where the staff object is not available (#1843).
  readonly staff?: SickReportStaff;
}) {
  const toast = useToast();
  const year = useMemo(() => new Date().getFullYear(), []);
  const [quota, setQuota] = useState<StaffVacationQuotaSummary | null>(null);
  const [absences, setAbsences] = useState<StaffAbsenceRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [pendingActionId, setPendingActionId] = useState<number | null>(null);
  const [denyModal, setDenyModal] = useState<StaffAbsenceRow | null>(null);
  const [questionModal, setQuestionModal] = useState<StaffAbsenceRow | null>(
    null,
  );
  const [quotaModal, setQuotaModal] = useState(false);
  // Snapshot of the staff identity taken when the modal opens: the live
  // `staff` prop can flip to undefined while onCreated's cache invalidation
  // refetches the page data, and a conditional `{staff && <Modal/>}` render
  // would then unmount the modal mid-success-state (#1843).
  const [sickStaff, setSickStaff] = useState<SickReportStaff | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<StaffAbsenceRow | null>(
    null,
  );
  const [deleteLoading, setDeleteLoading] = useState(false);
  const [deleteError, setDeleteError] = useState("");
  const { mutate: swrMutate } = useSWRConfig();

  // A sick report / its deletion cascades into the Dienst- and
  // Vertretungsplan, so revalidate those caches alongside the local reload.
  const refreshPlanCaches = useTenantMutateMatching([
    "dienstplan-overview-",
    "dienstplan-shifts-",
    VERTRETUNG_WEEK_KEY_PREFIX,
    VERTRETUNG_GAPS_KEY_PREFIX,
  ]);

  // silent skips the loading flag: the loading boundary unmounts the
  // whole tab subtree including any open modal, which would wipe the
  // SickReportModal's success state right after a report (#1843).
  const reload = useCallback(
    async (opts?: { silent?: boolean }) => {
      if (!opts?.silent) {
        setLoading(true);
      }
      try {
        const [q, abs] = await Promise.all([
          staffAbsenceService.getVacationQuota(staffId, year),
          staffAbsenceService.getAbsences(
            staffId,
            `${year}-01-01`,
            `${year}-12-31`,
          ),
        ]);
        setQuota(q);
        setAbsences(abs);
        // Page-level tab badge SWR (staff-pending-absences-${staffId}) lives
        // outside this component and otherwise stays stale after approvals,
        // denials and stornos. useSWRAuth prefixes every key with the tenant
        // slug ("phoenix:staff-pending-absences-…"), so a plain startsWith
        // never matches, we use includes for the cache hit.
        swrMutate(
          (key) =>
            typeof key === "string" && key.includes("staff-pending-absences-"),
        );
        // Sidebar pending counter (#1419) listens for this event.
        dispatchAbsencesRefresh();
      } catch (err) {
        logger.error("load_failed", {
          staff_id: staffId,
          error: err instanceof Error ? err.message : String(err),
        });
        toast.error("Daten konnten nicht geladen werden.");
      } finally {
        setLoading(false);
      }
    },
    [staffId, year, swrMutate, toast],
  );

  useEffect(() => {
    reload();
  }, [reload]);

  const pending = useMemo(() => pendingAbsences(absences), [absences]);
  const upcoming = useMemo(() => upcomingAbsences(absences), [absences]);
  const history = useMemo(() => historicalAbsences(absences), [absences]);

  const handleApprove = async (row: StaffAbsenceRow) => {
    setPendingActionId(row.id);
    try {
      await staffAbsenceService.approve(row.id);
      toast.success("Antrag genehmigt.");
      await reload();
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Genehmigung fehlgeschlagen.",
      );
    } finally {
      setPendingActionId(null);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleteLoading(true);
    setDeleteError("");
    try {
      await staffAbsenceService.deleteAbsence(staffId, deleteTarget.id);
      toast.success("Krankmeldung gelöscht.");
      setDeleteTarget(null);
      await Promise.all([refreshPlanCaches(), reload()]);
    } catch (err) {
      setDeleteError(
        err instanceof Error ? err.message : "Löschen fehlgeschlagen.",
      );
    } finally {
      setDeleteLoading(false);
    }
  };

  const content = (
    <div className="space-y-5">
      {/* Quota KPI cards */}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <QuotaTile
          label="Resturlaub"
          value={`${quota?.remaining_days ?? 0}`}
          hint="Tage"
          tone="primary"
        />
        <QuotaTile
          label="Anspruch"
          value={`${(quota?.entitled_days ?? 0) + (quota?.carryover_days ?? 0)}`}
          hint={
            (quota?.carryover_days ?? 0) > 0
              ? `${quota?.entitled_days ?? 0} + ${quota?.carryover_days ?? 0} Übertrag`
              : "Tage"
          }
          tone="primary"
          onEdit={canEdit ? () => setQuotaModal(true) : undefined}
        />
        <QuotaTile
          label="Genommen"
          value={`${quota?.taken_days ?? 0}`}
          hint="genehmigt"
          tone="success"
        />
        <QuotaTile
          label="Beantragt"
          value={`${quota?.reserved_days ?? 0}`}
          hint="offene Anträge"
          tone={(quota?.reserved_days ?? 0) > 0 ? "amber" : "primary"}
        />
      </div>

      <div className="flex items-center justify-between gap-3">
        {canManageSickReports && staff ? (
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={() => setSickStaff(staff ?? null)}
          >
            <Thermometer className="mr-1.5 h-4 w-4" aria-hidden />
            Krank melden
          </Button>
        ) : (
          <span />
        )}
        <span className="inline-flex items-center rounded-full border border-gray-200 bg-white px-3 py-1 text-xs font-medium text-gray-600">
          Jahr {year}
        </span>
      </div>

      <PendingAbsences
        rows={pending}
        canEdit={canEdit}
        pendingActionId={pendingActionId}
        onApprove={handleApprove}
        onDeny={setDenyModal}
        onQuestion={setQuestionModal}
      />

      {/* Upcoming approved absences */}
      <div className="rounded-3xl border border-gray-100/50 bg-white/90 p-5 shadow-[0_8px_30px_rgb(0,0,0,0.12)]">
        <h3 className="mb-3 text-xs font-semibold tracking-wider text-gray-500 uppercase">
          Kommende Abwesenheiten
        </h3>
        {upcoming.length === 0 ? (
          <p className="py-6 text-center text-sm text-gray-400">
            Keine geplanten Abwesenheiten.
          </p>
        ) : (
          <ul className="space-y-2">
            {upcoming.map((row) => (
              <AbsenceRow
                key={row.id}
                row={row}
                onDelete={
                  canManageSickReports && !isVacationWorkflowAbsence(row)
                    ? () => setDeleteTarget(row)
                    : undefined
                }
              />
            ))}
          </ul>
        )}
      </div>

      {/* Past history */}
      <div className="rounded-3xl border border-gray-100/50 bg-white/90 p-5 shadow-[0_8px_30px_rgb(0,0,0,0.12)]">
        <h3 className="mb-3 text-xs font-semibold tracking-wider text-gray-500 uppercase">
          Historie {year}
        </h3>
        {history.length === 0 ? (
          <p className="py-6 text-center text-sm text-gray-400">
            Keine vergangenen oder abgelehnten Abwesenheiten.
          </p>
        ) : (
          <ul className="space-y-2">
            {history.slice(0, 20).map((row) => (
              <AbsenceRow
                key={row.id}
                row={row}
                onDelete={
                  canManageSickReports && !isVacationWorkflowAbsence(row)
                    ? () => setDeleteTarget(row)
                    : undefined
                }
              />
            ))}
          </ul>
        )}
      </div>

      {/* Modals */}
      {sickStaff && (
        <SickReportModal
          isOpen
          staff={sickStaff}
          onClose={() => setSickStaff(null)}
          onCreated={() => {
            Promise.all([refreshPlanCaches(), reload({ silent: true })]).catch(
              (err: unknown) => {
                logger.error("post_create_refresh_failed", {
                  staff_id: staffId,
                  error: err instanceof Error ? err.message : String(err),
                });
              },
            );
          }}
        />
      )}
      <ConfirmDeleteModal
        isOpen={deleteTarget !== null}
        title={`${deleteTarget ? absenceTypeLabel(deleteTarget.absence_type) : "Abwesenheit"} löschen`}
        description={
          <>
            Die Abwesenheit{" "}
            <strong>
              {deleteTarget
                ? formatRange(deleteTarget.date_start, deleteTarget.date_end)
                : ""}
            </strong>{" "}
            wird gelöscht.
            {deleteTarget?.absence_type === "sick" && (
              <>
                {" "}
                Stornierte Schichten und als abwesend markierte Betreuungsblöcke
                werden wiederhergestellt, sofern noch kein Ersatz eingetragen
                wurde.
              </>
            )}
          </>
        }
        gate={{ mode: "twoStep" }}
        loading={deleteLoading}
        error={deleteError}
        confirmLabel="Löschen"
        loadingLabel="Wird gelöscht…"
        onConfirm={handleDelete}
        onClose={() => setDeleteTarget(null)}
      />
      {denyModal && (
        <DenyAbsenceModal
          absence={denyModal}
          onClose={() => setDenyModal(null)}
          onDenied={async () => {
            setDenyModal(null);
            await reload();
          }}
        />
      )}
      {questionModal && (
        <QuestionAbsenceModal
          absence={questionModal}
          onClose={() => setQuestionModal(null)}
          onQuestioned={async () => {
            setQuestionModal(null);
            await reload();
          }}
        />
      )}
      {quotaModal && quota && (
        <EditQuotaModal
          staffId={staffId}
          quota={quota}
          year={year}
          onClose={() => setQuotaModal(false)}
          onSaved={async () => {
            setQuotaModal(false);
            await reload();
          }}
        />
      )}
    </div>
  );

  return <TabLoadingBoundary loading={loading}>{content}</TabLoadingBoundary>;
}

function PendingAbsences({
  rows,
  canEdit,
  pendingActionId,
  onApprove,
  onDeny,
  onQuestion,
}: {
  readonly rows: StaffAbsenceRow[];
  readonly canEdit: boolean;
  readonly pendingActionId: number | null;
  readonly onApprove: (row: StaffAbsenceRow) => Promise<void>;
  readonly onDeny: (row: StaffAbsenceRow) => void;
  readonly onQuestion: (row: StaffAbsenceRow) => void;
}) {
  if (rows.length === 0) {
    return (
      <div className="flex items-center gap-3 rounded-3xl border border-gray-100/80 bg-white/60 px-5 py-4 text-sm text-gray-500">
        <span className="inline-flex h-7 w-7 items-center justify-center rounded-full bg-[#83CD2D]/15 text-[#4a7a15]">
          <svg
            className="h-4 w-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2.5}
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M5 13l4 4L19 7"
            />
          </svg>
        </span>
        <div>
          <p className="text-sm font-medium text-gray-700">
            Keine offenen Anfragen
          </p>
          <p className="text-xs text-gray-400">Alles bearbeitet.</p>
        </div>
      </div>
    );
  }

  return (
    <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-5">
      <div className="mb-3 flex items-center gap-2">
        <h3 className="text-xs font-semibold tracking-wider text-gray-400 uppercase">
          Eingehende Anfragen
        </h3>
        <span className="inline-flex h-4 min-w-[1rem] items-center justify-center rounded-full bg-red-500 px-1 text-[10px] font-bold text-white">
          {rows.length}
        </span>
      </div>
      <ul className="divide-y divide-gray-100">
        {rows.map((row) => (
          <AbsenceRequestRow
            key={row.id}
            row={row}
            isBusy={pendingActionId === row.id}
            showActions={canEdit}
            onApprove={(r) => {
              onApprove(r);
            }}
            onDeny={onDeny}
            onQuestion={onQuestion}
          />
        ))}
      </ul>
    </div>
  );
}

function QuotaTile({
  label,
  value,
  hint,
  tone,
  onEdit,
}: {
  readonly label: string;
  readonly value: string;
  readonly hint: string;
  readonly tone: "primary" | "success" | "amber" | "muted";
  readonly onEdit?: () => void;
}) {
  const valueClass = {
    primary: "text-gray-900",
    success: "text-[#70b525]",
    amber: "text-amber-600",
    muted: "text-gray-400",
  }[tone];
  return (
    <div className="relative rounded-2xl border border-gray-100 bg-white p-4 shadow-[0_8px_30px_rgb(0,0,0,0.06)]">
      <p className="text-[10px] font-semibold tracking-wider text-gray-400 uppercase">
        {label}
      </p>
      <p className={`mt-1 text-2xl font-bold tabular-nums ${valueClass}`}>
        {value}
      </p>
      <p className="mt-0.5 text-xs text-gray-400">{hint}</p>
      {onEdit && (
        <button
          type="button"
          onClick={onEdit}
          aria-label="Urlaubsanspruch bearbeiten"
          title="Urlaubsanspruch bearbeiten"
          className="absolute top-2 right-2 rounded-md p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700"
        >
          <svg
            className="h-3.5 w-3.5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
            />
          </svg>
        </button>
      )}
    </div>
  );
}

// Generic notes that duplicate the type badge get suppressed, admins seeded
// "Urlaub" or "Krankmeldung" as boilerplate, and rendering them next to the
// already-visible type badge is just noise.
const REDUNDANT_NOTES: Record<string, ReadonlyArray<string>> = {
  sick: ["krank", "krankheit", "krankmeldung", "au", "au-bescheinigung"],
  vacation: ["urlaub", "erholungsurlaub"],
  training: ["fortbildung", "schulung"],
  other: ["abwesend", "sonstige", "sonstiges"],
};

function isRedundantNote(absenceType: string, note: string): boolean {
  const normalized = note.trim().toLowerCase();
  if (!normalized) return true;
  const typeLabel = (ABSENCE_TYPE_LABEL[absenceType] ?? "").toLowerCase();
  if (normalized === typeLabel) return true;
  return (REDUNDANT_NOTES[absenceType] ?? []).includes(normalized);
}

function AbsenceRow({
  row,
  onDelete,
}: {
  readonly row: StaffAbsenceRow;
  // Delete action for admin-entered absences (status "reported"), e.g. a sick
  // report whose plan cascade can be undone. Omitted for read-only rows.
  readonly onDelete?: () => void;
}) {
  const meta = statusMeta(row.status);
  const showNote = row.note && !isRedundantNote(row.absence_type, row.note);
  const canDelete = onDelete && row.status === "reported";
  return (
    <li className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-gray-100 bg-white px-4 py-3">
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <StatusDotBadge
            label={ABSENCE_TYPE_LABEL[row.absence_type] ?? row.absence_type}
            color={
              ABSENCE_TYPE_HEX[row.absence_type] ?? LOCATION_COLORS.UNKNOWN
            }
          />
          <p className="text-sm font-medium text-gray-800">
            {formatRange(row.date_start, row.date_end)}
            <span className="ml-2 text-xs text-gray-500">
              · {formatDayCount(dayCountFor(row))}
            </span>
          </p>
        </div>
        {showNote && (
          <p className="mt-0.5 truncate text-xs text-gray-500">{row.note}</p>
        )}
        {row.decision_note && (
          <p className="mt-0.5 text-xs text-gray-500">
            <span className="font-medium">Anmerkung Leitung:</span>{" "}
            {row.decision_note}
          </p>
        )}
      </div>
      <div className="flex items-center gap-2">
        <StatusDotBadge label={meta.label} color={meta.color} />
        {canDelete && (
          <button
            type="button"
            onClick={onDelete}
            aria-label={`${absenceTypeLabel(row.absence_type)} ${formatRange(row.date_start, row.date_end)} löschen`}
            title={`${absenceTypeLabel(row.absence_type)} löschen`}
            className="rounded-md p-1.5 text-gray-400 transition-colors hover:bg-red-50 hover:text-red-600"
          >
            <Trash2 className="h-4 w-4" aria-hidden />
          </button>
        )}
      </div>
    </li>
  );
}

function EditQuotaModal({
  staffId,
  quota,
  year,
  onClose,
  onSaved,
}: {
  readonly staffId: string;
  readonly quota: StaffVacationQuotaSummary;
  readonly year: number;
  readonly onClose: () => void;
  readonly onSaved: () => void | Promise<void>;
}) {
  const [entitled, setEntitled] = useState(String(quota.entitled_days));
  const [carryover, setCarryover] = useState(String(quota.carryover_days));
  const [submitting, setSubmitting] = useState(false);
  const toast = useToast();

  const handleSubmit = async () => {
    const e = Number.parseFloat(entitled);
    const c = Number.parseFloat(carryover);
    if (Number.isNaN(e) || e < 0 || e > 366) {
      toast.error("Anspruch ungültig (0-366).");
      return;
    }
    if (Number.isNaN(c) || c < 0 || c > 366) {
      toast.error("Übertrag ungültig (0-366).");
      return;
    }
    setSubmitting(true);
    try {
      await staffAbsenceService.setVacationQuota(staffId, {
        year,
        entitled_days: e,
        carryover_days: c,
      });
      toast.success("Urlaubsanspruch gespeichert.");
      await onSaved();
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Speichern fehlgeschlagen.",
      );
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      isOpen
      onClose={() => !submitting && onClose()}
      title={`Urlaubsanspruch ${year} bearbeiten`}
      footer={
        <div className="flex w-full flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <button
            type="button"
            onClick={onClose}
            disabled={submitting}
            className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50 disabled:opacity-50"
          >
            Abbrechen
          </button>
          <button
            type="button"
            onClick={handleSubmit}
            disabled={submitting}
            className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700 disabled:opacity-50"
          >
            {submitting ? "…" : "Speichern"}
          </button>
        </div>
      }
    >
      <div className="space-y-4">
        <div>
          <label
            htmlFor="quota-entitled"
            className="mb-1 block text-xs font-semibold tracking-wider text-gray-500 uppercase"
          >
            Jahresanspruch (Tage)
          </label>
          <input
            id="quota-entitled"
            type="number"
            min="0"
            max="366"
            step="0.5"
            value={entitled}
            onChange={(e) => setEntitled(e.target.value)}
            className="w-full rounded-xl border border-gray-200 bg-white px-3 py-2 text-sm text-gray-800 focus:border-[#83CD2D] focus:outline-none"
          />
        </div>
        <div>
          <label
            htmlFor="quota-carryover"
            className="mb-1 block text-xs font-semibold tracking-wider text-gray-500 uppercase"
          >
            Übertrag aus Vorjahr (Tage)
          </label>
          <input
            id="quota-carryover"
            type="number"
            min="0"
            max="366"
            step="0.5"
            value={carryover}
            onChange={(e) => setCarryover(e.target.value)}
            className="w-full rounded-xl border border-gray-200 bg-white px-3 py-2 text-sm text-gray-800 focus:border-[#83CD2D] focus:outline-none"
          />
        </div>
      </div>
    </Modal>
  );
}
