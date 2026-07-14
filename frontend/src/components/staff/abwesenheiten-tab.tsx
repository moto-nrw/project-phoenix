"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useSWRConfig } from "swr";
import { Thermometer, Trash2 } from "lucide-react";

import {
  SickReportModal,
  type SickReportStaff,
} from "~/components/staff/sick-report-modal";
import { Button } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { Loading } from "~/components/ui/loading";
import { Modal } from "~/components/ui/modal";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { todayISO } from "~/lib/date-helpers";
import {
  staffAbsenceService,
  type StaffAbsenceRow,
  type StaffVacationQuotaSummary,
} from "~/lib/staff-api";
import { useTenantMutateMatching } from "~/lib/swr";

const logger = createLogger({ component: "AbwesenheitenTab" });

const ABSENCE_TYPE_LABEL: Record<string, string> = {
  vacation: "Urlaub",
  sick: "Krank",
  training: "Fortbildung",
  other: "Sonstige",
};

// Noun form for the delete action ("Krankmeldung löschen", not "Krank
// löschen"); unknown types fall back to the generic noun.
function absenceTypeLabel(absenceType: string): string {
  if (absenceType === "sick") return "Krankmeldung";
  if (absenceType === "vacation") return "Urlaub";
  if (absenceType === "training") return "Fortbildung";
  return "Abwesenheit";
}

const ABSENCE_TYPE_COLOR: Record<string, string> = {
  vacation: "bg-[#5080D8]/15 text-[#5080D8]",
  sick: "bg-[#EAB308]/15 text-amber-700",
  training: "bg-[#7C3AED]/15 text-purple-700",
  other: "bg-gray-100 text-gray-600",
};

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString("de-DE", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

function formatRange(start: string, end: string): string {
  return start === end
    ? formatDate(start)
    : `${formatDate(start)} - ${formatDate(end)}`;
}

// Mon-Fri working days inclusive (Feiertage kommen in Tranche 3). Used as
// a fallback when the row predates the backend-computed working_days field.
function countWorkdaysInclusive(startISO: string, endISO: string): number {
  const start = new Date(`${startISO}T00:00:00`);
  const end = new Date(`${endISO}T00:00:00`);
  if (end.getTime() < start.getTime()) return 0;
  let n = 0;
  const cur = new Date(start);
  while (cur.getTime() <= end.getTime()) {
    const dow = cur.getDay();
    if (dow !== 0 && dow !== 6) n += 1;
    cur.setDate(cur.getDate() + 1);
  }
  return n;
}

function formatDayCount(days: number): string {
  // German decimal comma, drop trailing ",0" so "5" prints as "5 Tage".
  const rounded = Math.round(days * 10) / 10;
  const display = Number.isInteger(rounded)
    ? rounded.toString()
    : rounded.toFixed(1).replace(".", ",");
  return `${display} ${rounded === 1 ? "Tag" : "Tage"}`;
}

function dayCountFor(row: StaffAbsenceRow): number {
  if (row.working_days != null) return row.working_days;
  const base = countWorkdaysInclusive(row.date_start, row.date_end);
  if (base <= 0) return base;
  const hasBoundaryFields =
    row.start_half_day !== undefined || row.end_half_day !== undefined;
  if (!hasBoundaryFields) return row.half_day ? base - 0.5 : base;
  const start = new Date(`${row.date_start.slice(0, 10)}T00:00:00`);
  const end = new Date(`${row.date_end.slice(0, 10)}T00:00:00`);
  let days = base;
  if (row.start_half_day && isWorkday(start)) days -= 0.5;
  if (
    row.end_half_day &&
    row.date_end.slice(0, 10) !== row.date_start.slice(0, 10) &&
    isWorkday(end)
  ) {
    days -= 0.5;
  }
  if (
    row.end_half_day &&
    row.date_end.slice(0, 10) === row.date_start.slice(0, 10) &&
    !row.start_half_day &&
    isWorkday(end)
  ) {
    days -= 0.5;
  }
  return days;
}

function isWorkday(date: Date): boolean {
  const dow = date.getDay();
  return dow !== 0 && dow !== 6;
}

interface StatusMeta {
  readonly label: string;
  readonly className: string;
}

function statusMeta(status: string): StatusMeta {
  switch (status) {
    case "requested":
      return { label: "Wartet", className: "bg-amber-50 text-amber-700" };
    case "approved":
      return {
        label: "Genehmigt",
        className: "bg-[#83CD2D]/15 text-[#4a7a15]",
      };
    case "declined":
      return { label: "Abgelehnt", className: "bg-red-50 text-red-700" };
    case "canceled":
      return { label: "Storniert", className: "bg-gray-100 text-gray-500" };
    case "reported":
      return { label: "Eingetragen", className: "bg-gray-100 text-gray-700" };
    default:
      return { label: status, className: "bg-gray-100 text-gray-600" };
  }
}

export function AbwesenheitenTab({
  staffId,
  canEdit,
  staff,
}: {
  readonly staffId: string;
  readonly canEdit: boolean;
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
    "vertretungsplan-week-",
    "vertretungsplan-gaps-",
  ]);

  // silent skips the loading flag: the loading early-return unmounts the
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

  const pending = useMemo(
    () => absences.filter((a) => a.status === "requested"),
    [absences],
  );
  const upcoming = useMemo(() => {
    const today = todayISO();
    return absences.filter(
      (a) =>
        (a.status === "approved" || a.status === "reported") &&
        a.date_end >= today,
    );
  }, [absences]);
  const history = useMemo(
    () =>
      absences
        .filter(
          (a) =>
            a.status === "declined" ||
            a.status === "canceled" ||
            ((a.status === "approved" || a.status === "reported") &&
              a.date_end < todayISO()),
        )
        .sort((a, b) => (a.date_start < b.date_start ? 1 : -1)),
    [absences],
  );

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
      void refreshPlanCaches();
      await reload();
    } catch (err) {
      setDeleteError(
        err instanceof Error ? err.message : "Löschen fehlgeschlagen.",
      );
    } finally {
      setDeleteLoading(false);
    }
  };

  if (loading) {
    return <Loading fullPage={false} />;
  }

  return (
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
        {canEdit && staff ? (
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
                onDelete={canEdit ? () => setDeleteTarget(row) : undefined}
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
                onDelete={canEdit ? () => setDeleteTarget(row) : undefined}
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
            void refreshPlanCaches();
            void reload({ silent: true });
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
}

function PendingAbsences({
  rows,
  canEdit,
  pendingActionId,
  onApprove,
  onDeny,
}: {
  readonly rows: StaffAbsenceRow[];
  readonly canEdit: boolean;
  readonly pendingActionId: number | null;
  readonly onApprove: (row: StaffAbsenceRow) => Promise<void>;
  readonly onDeny: (row: StaffAbsenceRow) => void;
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
    <div className="rounded-3xl border-2 border-amber-200 bg-amber-50/40 p-5 shadow-[0_8px_30px_rgb(0,0,0,0.08)]">
      <div className="mb-3 flex items-center gap-2">
        <span className="inline-flex h-5 w-5 items-center justify-center rounded-full bg-amber-500 text-[10px] font-bold text-white">
          {rows.length}
        </span>
        <h3 className="text-sm font-bold tracking-wide text-amber-900 uppercase">
          Eingehende Anfragen
        </h3>
      </div>
      <ul className="space-y-2">
        {rows.map((row) => (
          <PendingAbsenceRow
            key={row.id}
            row={row}
            canEdit={canEdit}
            isBusy={pendingActionId === row.id}
            onApprove={onApprove}
            onDeny={onDeny}
          />
        ))}
      </ul>
    </div>
  );
}

function PendingAbsenceRow({
  row,
  canEdit,
  isBusy,
  onApprove,
  onDeny,
}: {
  readonly row: StaffAbsenceRow;
  readonly canEdit: boolean;
  readonly isBusy: boolean;
  readonly onApprove: (row: StaffAbsenceRow) => Promise<void>;
  readonly onDeny: (row: StaffAbsenceRow) => void;
}) {
  return (
    <li className="rounded-2xl border border-amber-100 bg-white p-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <span
              className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${ABSENCE_TYPE_COLOR[row.absence_type] ?? "bg-gray-100 text-gray-600"}`}
            >
              {ABSENCE_TYPE_LABEL[row.absence_type] ?? row.absence_type}
            </span>
            <p className="text-sm font-medium text-gray-800">
              {formatRange(row.date_start, row.date_end)}
              {row.half_day && (
                <span className="ml-2 text-xs text-gray-500">(halber Tag)</span>
              )}
              {row.working_days != null && (
                <span className="ml-2 text-xs text-gray-500">
                  · {row.working_days} {row.working_days === 1 ? "Tag" : "Tage"}
                </span>
              )}
            </p>
          </div>
          {row.note && (
            <p className="mt-2 text-xs text-gray-600">
              <span className="font-medium">Notiz:</span> {row.note}
            </p>
          )}
          {row.requested_at && (
            <p className="mt-1 text-[11px] text-gray-400">
              Eingegangen {formatDate(row.requested_at)}
            </p>
          )}
        </div>
        {canEdit && (
          <div className="flex w-full flex-col gap-2 min-[420px]:w-auto min-[420px]:flex-row">
            <button
              type="button"
              onClick={() => onDeny(row)}
              disabled={isBusy}
              className="rounded-lg border border-gray-300 px-3 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-50 disabled:opacity-50"
            >
              Ablehnen
            </button>
            <button
              type="button"
              onClick={() => {
                onApprove(row);
              }}
              disabled={isBusy}
              className="rounded-lg bg-[#83CD2D] px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-[#74b827] disabled:opacity-50"
            >
              {isBusy ? "…" : "Genehmigen"}
            </button>
          </div>
        )}
      </div>
    </li>
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
          <span
            className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${ABSENCE_TYPE_COLOR[row.absence_type] ?? "bg-gray-100 text-gray-600"}`}
          >
            {ABSENCE_TYPE_LABEL[row.absence_type] ?? row.absence_type}
          </span>
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
        <span
          className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${meta.className}`}
        >
          {meta.label}
        </span>
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

function DenyAbsenceModal({
  absence,
  onClose,
  onDenied,
}: {
  readonly absence: StaffAbsenceRow;
  readonly onClose: () => void;
  readonly onDenied: () => void | Promise<void>;
}) {
  const [reason, setReason] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const toast = useToast();

  const handleSubmit = async () => {
    if (reason.trim().length < 3) {
      toast.error("Bitte gib eine kurze Begründung ein.");
      return;
    }
    setSubmitting(true);
    try {
      await staffAbsenceService.deny(absence.id, reason.trim());
      toast.success("Antrag abgelehnt.");
      await onDenied();
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Ablehnung fehlgeschlagen.",
      );
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      isOpen
      onClose={() => !submitting && onClose()}
      title="Antrag ablehnen"
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
            className="rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-red-700 disabled:opacity-50"
          >
            {submitting ? "…" : "Ablehnen"}
          </button>
        </div>
      }
    >
      <div className="space-y-3">
        <p className="text-sm text-gray-700">
          Antrag {formatRange(absence.date_start, absence.date_end)} (
          {ABSENCE_TYPE_LABEL[absence.absence_type] ?? absence.absence_type})
        </p>
        <label
          htmlFor="deny-reason"
          className="block text-xs font-semibold tracking-wider text-gray-500 uppercase"
        >
          Begründung
        </label>
        <textarea
          id="deny-reason"
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          rows={4}
          maxLength={500}
          placeholder="Wird der Mitarbeiterin per E-Mail mitgeteilt."
          className="w-full resize-none rounded-xl border border-gray-200 bg-white px-3 py-2 text-sm text-gray-800 placeholder:text-gray-400 focus:border-red-500 focus:outline-none"
        />
        <p className="text-right text-xs text-gray-400">{reason.length}/500</p>
      </div>
    </Modal>
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
