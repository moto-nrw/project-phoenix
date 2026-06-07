"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { ConfirmationModal } from "~/components/ui/modal";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import type { StaffAbsence } from "~/lib/time-tracking-helpers";
import {
  timeTrackingService,
  type VacationQuotaSummary,
} from "~/lib/time-tracking-api";
import { VacationRequestModal } from "./vacation-request-modal";

const logger = createLogger({ component: "LeaveRequestsCard" });

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

// Mon-Fri inclusive (Feiertage kommen in Tranche 3). Fallback when the row
// predates the backend-computed workingDays field.
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
  const rounded = Math.round(days * 10) / 10;
  const display = Number.isInteger(rounded)
    ? rounded.toString()
    : rounded.toFixed(1).replace(".", ",");
  return `${display} ${rounded === 1 ? "Tag" : "Tage"}`;
}

function dayCountFor(a: StaffAbsence): number {
  if (a.workingDays != null) return a.workingDays;
  const base = countWorkdaysInclusive(a.dateStart, a.dateEnd);
  if (base <= 0) return base;
  const start = new Date(`${a.dateStart}T00:00:00`);
  const end = new Date(`${a.dateEnd}T00:00:00`);
  const sameDay = a.dateStart === a.dateEnd;
  let days = base;
  if (a.startHalfDay && isWorkday(start)) days -= 0.5;
  if (a.endHalfDay && !sameDay && isWorkday(end)) days -= 0.5;
  if (a.endHalfDay && sameDay && !a.startHalfDay && isWorkday(end)) {
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
      return {
        label: "Wartet auf Antwort",
        className: "bg-amber-50 text-amber-700",
      };
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

export function LeaveRequestsCard() {
  const [modalOpen, setModalOpen] = useState(false);
  const [quota, setQuota] = useState<VacationQuotaSummary | null>(null);
  const [vacations, setVacations] = useState<StaffAbsence[]>([]);
  const [loading, setLoading] = useState(true);
  const toast = useToast();

  const year = useMemo(() => new Date().getFullYear(), []);

  const loadAll = useCallback(async () => {
    setLoading(true);
    try {
      const yearStart = `${year}-01-01`;
      const yearEnd = `${year}-12-31`;
      const [q, abs] = await Promise.all([
        timeTrackingService.getVacationQuota(year),
        timeTrackingService.getAbsences(yearStart, yearEnd),
      ]);
      setQuota(q);
      setVacations(abs.filter((a) => a.absenceType === "vacation"));
    } catch (err) {
      logger.error("load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
    } finally {
      setLoading(false);
    }
  }, [year]);

  useEffect(() => {
    loadAll();
  }, [loadAll]);

  const counts = useMemo(() => {
    const reserved = vacations.filter((v) => v.status === "requested").length;
    const approved = vacations.filter(
      (v) =>
        (v.status === "approved" || v.status === "reported") &&
        new Date(v.dateEnd) >= new Date(),
    ).length;
    const declined = vacations.filter((v) => v.status === "declined").length;
    return { reserved, approved, declined };
  }, [vacations]);

  const [cancelTarget, setCancelTarget] = useState<StaffAbsence | null>(null);
  const [cancelSubmitting, setCancelSubmitting] = useState(false);

  const handleCancel = (absence: StaffAbsence) => {
    setCancelTarget(absence);
  };

  const confirmCancel = async () => {
    if (!cancelTarget) return;
    setCancelSubmitting(true);
    try {
      await timeTrackingService.cancelAbsence(cancelTarget.id);
      toast.success("Antrag storniert.");
      setCancelTarget(null);
      await loadAll();
    } catch (err) {
      toast.error(
        err instanceof Error
          ? err.message
          : "Antrag konnte nicht storniert werden.",
      );
    } finally {
      setCancelSubmitting(false);
    }
  };

  const remainingDays = quota?.remaining_days ?? 0;

  return (
    <>
      <div className="rounded-3xl border border-gray-100/50 bg-white/90 p-5 shadow-[0_8px_30px_rgb(0,0,0,0.12)] sm:p-6 md:p-8">
        <div className="mb-5 flex flex-wrap items-center justify-between gap-2">
          <h2 className="text-base font-bold text-gray-900 sm:text-lg">
            Urlaub
          </h2>
          <span className="text-xs text-gray-400">{year}</span>
        </div>

        <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
          <Tile
            label="Resturlaub"
            value={loading ? "-" : `${remainingDays} Tage`}
            hint={
              quota
                ? `${quota.entitled_days + quota.carryover_days} Anspruch`
                : "lädt…"
            }
            tone="primary"
          />
          <Tile
            label="Beantragt"
            value={loading ? "-" : String(counts.reserved)}
            hint="wartet auf Antwort"
            tone={counts.reserved > 0 ? "amber" : "muted"}
          />
          <Tile
            label="Genehmigt"
            value={loading ? "-" : String(counts.approved)}
            hint="kommende Tage"
            tone={counts.approved > 0 ? "success" : "muted"}
          />
          <Tile
            label="Abgelehnt"
            value={loading ? "-" : String(counts.declined)}
            hint="dieses Jahr"
            tone="muted"
          />
        </div>

        <div className="mt-5 flex flex-col gap-2 border-t border-gray-100 pt-4 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-xs text-gray-500">
            Urlaubsanträge stellen, Status verfolgen und stornieren.
          </p>
          <button
            type="button"
            onClick={() => setModalOpen(true)}
            disabled={loading}
            className="rounded-full bg-gray-900 px-4 py-2 text-xs font-medium text-white transition-colors hover:bg-gray-700 disabled:opacity-50"
          >
            Urlaub beantragen
          </button>
        </div>

        {vacations.length > 0 && (
          <div className="mt-5 border-t border-gray-100 pt-5">
            <h3 className="mb-3 text-xs font-semibold tracking-wider text-gray-500 uppercase">
              Meine Anträge
            </h3>
            <ul className="space-y-2">
              {vacations
                .slice()
                .sort(
                  (a, b) =>
                    new Date(b.requestedAt ?? b.dateStart).getTime() -
                    new Date(a.requestedAt ?? a.dateStart).getTime(),
                )
                .slice(0, 8)
                .map((v) => {
                  const meta = statusMeta(v.status);
                  const cancelable =
                    v.status === "requested" ||
                    (v.status === "approved" &&
                      new Date(v.dateStart) > new Date());
                  return (
                    <li
                      key={v.id}
                      className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-gray-100 bg-white px-4 py-3"
                    >
                      <div className="min-w-0">
                        <p className="text-sm font-medium text-gray-800">
                          {formatRange(v.dateStart, v.dateEnd)}
                          <span className="ml-2 text-xs text-gray-500">
                            · {formatDayCount(dayCountFor(v))}
                          </span>
                        </p>
                        {v.note && (
                          <p className="mt-0.5 truncate text-xs text-gray-500">
                            {v.note}
                          </p>
                        )}
                        {v.decisionNote && (
                          <p className="mt-0.5 text-xs text-gray-500">
                            <span className="font-medium">
                              Anmerkung Leitung:
                            </span>{" "}
                            {v.decisionNote}
                          </p>
                        )}
                      </div>
                      <div className="flex items-center gap-3">
                        <span
                          className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${meta.className}`}
                        >
                          {meta.label}
                        </span>
                        {cancelable && (
                          <button
                            type="button"
                            onClick={() => handleCancel(v)}
                            className="text-xs font-medium text-red-600 hover:text-red-700"
                          >
                            Stornieren
                          </button>
                        )}
                      </div>
                    </li>
                  );
                })}
            </ul>
          </div>
        )}
      </div>

      <VacationRequestModal
        isOpen={modalOpen}
        onClose={() => setModalOpen(false)}
        onSubmitted={() => {
          loadAll();
        }}
        remainingDays={remainingDays}
        existingVacations={vacations}
      />

      <ConfirmationModal
        isOpen={cancelTarget !== null}
        onClose={() => !cancelSubmitting && setCancelTarget(null)}
        onConfirm={() => {
          confirmCancel();
        }}
        title="Antrag stornieren"
        confirmText="Stornieren"
        cancelText="Behalten"
        isConfirmLoading={cancelSubmitting}
        confirmButtonClass="bg-red-600 hover:bg-red-700"
      >
        {cancelTarget && (
          <div className="space-y-2 text-sm text-gray-700">
            <p>Möchtest du diesen Urlaubsantrag wirklich stornieren?</p>
            <p className="text-xs text-gray-500">
              {formatRange(cancelTarget.dateStart, cancelTarget.dateEnd)}
              {cancelTarget.status === "approved" && " (bereits genehmigt)"}
            </p>
          </div>
        )}
      </ConfirmationModal>
    </>
  );
}

function Tile({
  label,
  value,
  hint,
  tone,
}: {
  readonly label: string;
  readonly value: string;
  readonly hint: string;
  readonly tone: "primary" | "success" | "amber" | "muted";
}) {
  const valueClass = {
    primary: "text-gray-900",
    success: "text-[#70b525]",
    amber: "text-amber-600",
    muted: "text-gray-400",
  }[tone];
  return (
    <div className="rounded-2xl border border-gray-100 bg-white/70 p-4">
      <p className="text-[10px] font-semibold tracking-wider text-gray-400 uppercase">
        {label}
      </p>
      <p className={`mt-1 text-lg font-bold sm:text-xl ${valueClass}`}>
        {value}
      </p>
      <p className="mt-0.5 text-xs text-gray-400">{hint}</p>
    </div>
  );
}
