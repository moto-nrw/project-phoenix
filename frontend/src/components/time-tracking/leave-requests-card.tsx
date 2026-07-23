"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { Button } from "~/components/ui/button";
import { ConfirmationModal } from "~/components/ui/modal";
import { StatusDotBadge } from "~/components/ui/status-dot-badge";
import { Textarea } from "~/components/ui/textarea";
import { useToast } from "~/contexts/ToastContext";
import {
  absenceStatusMeta,
  dayCountFor as sharedDayCountFor,
  dispatchAbsencesRefresh,
  formatAbsenceRange,
  formatDayCount,
} from "~/lib/absence-helpers";
import { createLogger } from "~/lib/logger";
import type { StaffAbsence } from "~/lib/time-tracking-helpers";
import {
  timeTrackingService,
  type VacationQuotaSummary,
} from "~/lib/time-tracking-api";
import { VacationRequestModal } from "./vacation-request-modal";
import { useCurrentTimestamp } from "~/lib/hooks/use-current-timestamp";

const logger = createLogger({ component: "LeaveRequestsCard" });

function formatRange(start: string, end: string): string {
  return formatAbsenceRange(start, end);
}

function dayCountFor(a: StaffAbsence): number {
  return sharedDayCountFor({
    workingDays: a.workingDays,
    dateStart: a.dateStart,
    dateEnd: a.dateEnd,
    startHalfDay: a.startHalfDay,
    endHalfDay: a.endHalfDay,
    hasBoundaryFields: true,
  });
}

function statusMeta(status: string) {
  return absenceStatusMeta(status, { requestedLabel: "Wartet auf Antwort" });
}

function decisionNoteLabel(status: string): string {
  if (status === "question" || status === "canceled") {
    return "Rückfrage der Leitung:";
  }
  if (status === "requested") {
    return "Vorherige Rückfrage der Leitung:";
  }
  return "Anmerkung der Leitung:";
}

export function LeaveRequestsCard() {
  const currentTimestamp = useCurrentTimestamp();
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
    const question = vacations.filter((v) => v.status === "question").length;
    const approved = vacations.filter(
      (v) =>
        (v.status === "approved" || v.status === "reported") &&
        new Date(v.dateEnd) >= new Date(),
    ).length;
    const declined = vacations.filter((v) => v.status === "declined").length;
    return { reserved, question, approved, declined };
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
      dispatchAbsencesRefresh();
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

        <div
          className={`grid grid-cols-2 gap-4 ${counts.question > 0 ? "lg:grid-cols-5" : "lg:grid-cols-4"}`}
        >
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
          {counts.question > 0 && (
            <Tile
              label="Rückfrage"
              value={String(counts.question)}
              hint="Antwort nötig"
              tone="amber"
            />
          )}
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
                  const isQuestioned = v.status === "question";
                  const cancelable =
                    v.status === "requested" ||
                    isQuestioned ||
                    (v.status === "approved" &&
                      new Date(v.dateStart).getTime() > currentTimestamp);
                  return (
                    <li
                      key={v.id}
                      className="rounded-xl border border-gray-100 bg-white px-4 py-3"
                    >
                      <div className="flex flex-wrap items-center justify-between gap-3">
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
                                {decisionNoteLabel(v.status)}
                              </span>{" "}
                              {v.decisionNote}
                            </p>
                          )}
                        </div>
                        <div className="flex items-center gap-3">
                          <StatusDotBadge
                            label={meta.label}
                            color={meta.color}
                          />
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
                      </div>
                      {isQuestioned && (
                        <ResubmitAbsenceForm
                          absence={v}
                          onResubmitted={() => {
                            dispatchAbsencesRefresh();
                            loadAll();
                          }}
                        />
                      )}
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

// Inline answer form for a Rückfrage (#1419): the MA amends their note and
// resubmits, moving the request back to "requested" for a final decision.
function ResubmitAbsenceForm({
  absence,
  onResubmitted,
}: {
  readonly absence: StaffAbsence;
  readonly onResubmitted: () => void;
}) {
  const [note, setNote] = useState(absence.note ?? "");
  const [submitting, setSubmitting] = useState(false);
  const toast = useToast();

  const handleSubmit = async () => {
    if (note.trim().length < 3) {
      toast.error("Bitte gib eine kurze Antwort ein.");
      return;
    }
    setSubmitting(true);
    try {
      await timeTrackingService.resubmitAbsence(absence.id, note.trim());
      toast.success("Antrag erneut eingereicht.");
      onResubmitted();
    } catch (err) {
      toast.error(
        err instanceof Error
          ? err.message
          : "Antrag konnte nicht erneut eingereicht werden.",
      );
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="mt-3 border-t border-gray-100 pt-3">
      <label
        htmlFor={`resubmit-note-${absence.id}`}
        className="mb-1 block text-xs font-semibold tracking-wider text-gray-500 uppercase"
      >
        Deine Antwort
      </label>
      <Textarea
        id={`resubmit-note-${absence.id}`}
        value={note}
        onChange={(e) => setNote(e.target.value)}
        rows={2}
        maxLength={500}
        placeholder="Antwort auf die Rückfrage ergänzen…"
      />
      <div className="mt-2 flex justify-end">
        <Button
          type="button"
          variant="primary"
          size="compact"
          onClick={handleSubmit}
          disabled={submitting}
        >
          {submitting ? "…" : "Antwort senden & erneut einreichen"}
        </Button>
      </div>
    </div>
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
