"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import { Button } from "~/components/ui/button";
import { ConfirmationModal } from "~/components/ui/modal";
import { SectionCard } from "~/components/ui/section-card";
import { StatusColorBadge } from "~/components/ui/status-color-badge";
import { Textarea } from "~/components/ui/textarea";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import {
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
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
    halfDay: a.halfDay,
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
  const [questionedVacations, setQuestionedVacations] = useState<
    StaffAbsence[]
  >([]);
  const [loading, setLoading] = useState(true);
  const toast = useToast();
  // A failed load stays in the card (#2514): without the quota the tiles
  // would read as "0 Tage", which is wrong, not empty.
  const load = useApiLoadError();
  const showLoadError = load.show;
  const clearLoadError = load.clear;
  const cancelErrors = useApiFormError();
  const loadAllRef = useRef<() => Promise<void>>(async () => undefined);

  const year = useMemo(() => new Date().getFullYear(), []);

  const loadAll = useCallback(async () => {
    setLoading(true);
    try {
      const yearStart = `${year}-01-01`;
      const yearEnd = `${year}-12-31`;
      const [q, abs, questions] = await Promise.all([
        timeTrackingService.getVacationQuota(year),
        timeTrackingService.getAbsences(yearStart, yearEnd),
        timeTrackingService.getQuestionedAbsences(),
      ]);
      setQuota(q);
      setVacations(abs.filter((a) => a.absenceType === "vacation"));
      setQuestionedVacations(
        questions.filter(
          (a) => a.absenceType === "vacation" && a.status === "question",
        ),
      );
      clearLoadError();
    } catch (err) {
      logger.error("load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Urlaubsübersicht",
        retry: () => void loadAllRef.current(),
      });
    } finally {
      setLoading(false);
    }
  }, [year, showLoadError, clearLoadError]);

  useLayoutEffect(() => {
    loadAllRef.current = loadAll;
  });

  useEffect(() => {
    // loadAll shows its own failure in the card.
    void loadAll();
  }, [loadAll]);

  const counts = useMemo(() => {
    const reserved = vacations.filter((v) => v.status === "requested").length;
    const question = questionedVacations.length;
    const approved = vacations.filter(
      (v) =>
        (v.status === "approved" || v.status === "reported") &&
        new Date(v.dateEnd) >= new Date(),
    ).length;
    const declined = vacations.filter((v) => v.status === "declined").length;
    return { reserved, question, approved, declined };
  }, [questionedVacations.length, vacations]);

  const recentVacations = useMemo(
    () =>
      vacations
        .filter((vacation) => vacation.status !== "question")
        .sort(
          (a, b) =>
            new Date(b.requestedAt ?? b.dateStart).getTime() -
            new Date(a.requestedAt ?? a.dateStart).getTime(),
        )
        .slice(0, 8),
    [vacations],
  );

  const sortedQuestionedVacations = useMemo(
    () =>
      questionedVacations
        .slice()
        .sort(
          (a, b) =>
            new Date(b.requestedAt ?? b.dateStart).getTime() -
            new Date(a.requestedAt ?? a.dateStart).getTime(),
        ),
    [questionedVacations],
  );

  const blockingVacations = useMemo(
    () =>
      Array.from(
        new Map(
          [...vacations, ...questionedVacations].map((vacation) => [
            vacation.id,
            vacation,
          ]),
        ).values(),
      ),
    [questionedVacations, vacations],
  );

  const [cancelTarget, setCancelTarget] = useState<StaffAbsence | null>(null);
  const [cancelSubmitting, setCancelSubmitting] = useState(false);

  const handleCancel = (absence: StaffAbsence) => {
    cancelErrors.clear();
    setCancelTarget(absence);
  };

  const handleResubmitted = useCallback(() => {
    dispatchAbsencesRefresh();
    void loadAll();
  }, [loadAll]);

  const confirmCancel = async () => {
    if (!cancelTarget) return;
    setCancelSubmitting(true);
    cancelErrors.clear();
    try {
      await timeTrackingService.cancelAbsence(cancelTarget.id);
    } catch (err) {
      logger.error("cancel_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      // The dialog stays open and shows why (#2514).
      await cancelErrors.show(err, {
        object: "die Stornierung",
        retry: () => void confirmCancel(),
      });
      return;
    } finally {
      setCancelSubmitting(false);
    }
    toast.success("Der Antrag ist storniert.");
    setCancelTarget(null);
    dispatchAbsencesRefresh();
    await loadAll();
  };

  const remainingDays = quota?.remaining_days ?? 0;

  return (
    <>
      <SectionCard
        title="Urlaub"
        description="Urlaubsanträge stellen, Status verfolgen und stornieren."
        actions={
          <>
            <span className="text-xs text-gray-400">{year}</span>
            <Button
              type="button"
              variant="primary"
              size="md"
              onClick={() => setModalOpen(true)}
              disabled={loading || load.error !== null}
            >
              Urlaub beantragen
            </Button>
          </>
        }
        bodyClassName="mt-5"
      >
        {load.error ? (
          <LoadErrorAlert error={load.error} />
        ) : (
          <>
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

            {sortedQuestionedVacations.length > 0 && (
              <div className="mt-5 border-t border-gray-100 pt-5">
                <h3 className="mb-3 text-xs font-semibold tracking-wider text-gray-500 uppercase">
                  Rückfragen
                </h3>
                <ul className="space-y-2">
                  {sortedQuestionedVacations.map((vacation) => (
                    <AbsenceRequestItem
                      key={vacation.id}
                      absence={vacation}
                      currentTimestamp={currentTimestamp}
                      onCancel={handleCancel}
                      onResubmitted={handleResubmitted}
                      showResubmit
                    />
                  ))}
                </ul>
              </div>
            )}

            {recentVacations.length > 0 && (
              <div className="mt-5 border-t border-gray-100 pt-5">
                <h3 className="mb-3 text-xs font-semibold tracking-wider text-gray-500 uppercase">
                  Meine Anträge
                </h3>
                <ul className="space-y-2">
                  {recentVacations.map((vacation) => (
                    <AbsenceRequestItem
                      key={vacation.id}
                      absence={vacation}
                      currentTimestamp={currentTimestamp}
                      onCancel={handleCancel}
                    />
                  ))}
                </ul>
              </div>
            )}
          </>
        )}
      </SectionCard>

      <VacationRequestModal
        isOpen={modalOpen}
        onClose={() => setModalOpen(false)}
        onSubmitted={() => {
          loadAll();
        }}
        remainingDays={remainingDays}
        existingVacations={blockingVacations}
      />

      <ConfirmationModal
        isOpen={cancelTarget !== null}
        onClose={() => !cancelSubmitting && setCancelTarget(null)}
        onConfirm={() => {
          void confirmCancel();
        }}
        title="Antrag stornieren"
        confirmText="Stornieren"
        cancelText="Behalten"
        isConfirmLoading={cancelSubmitting}
        confirmVariant="danger"
      >
        {cancelTarget && (
          <div className="space-y-2 text-sm text-gray-700">
            <FormErrorAlert message={cancelErrors.error} />
            <p>Möchten Sie diesen Urlaubsantrag wirklich stornieren?</p>
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

function AbsenceRequestItem({
  absence,
  currentTimestamp,
  onCancel,
  onResubmitted,
  showResubmit = false,
}: {
  readonly absence: StaffAbsence;
  readonly currentTimestamp: number;
  readonly onCancel: (absence: StaffAbsence) => void;
  readonly onResubmitted?: () => void;
  readonly showResubmit?: boolean;
}) {
  const meta = statusMeta(absence.status);
  const cancelable =
    absence.status === "requested" ||
    absence.status === "question" ||
    (absence.status === "approved" &&
      new Date(absence.dateStart).getTime() > currentTimestamp);

  return (
    <li className="moto-content-surface rounded-xl border px-4 py-3 shadow-sm">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="min-w-0">
          <p className="text-sm font-medium text-gray-800">
            {formatRange(absence.dateStart, absence.dateEnd)}
            <span className="ml-2 text-xs text-gray-500">
              · {formatDayCount(dayCountFor(absence))}
            </span>
          </p>
          {absence.note && (
            <p className="mt-0.5 truncate text-xs text-gray-500">
              {absence.note}
            </p>
          )}
          {absence.decisionNote && (
            <p className="mt-0.5 text-xs text-gray-500">
              <span className="font-medium">
                {decisionNoteLabel(absence.status)}
              </span>{" "}
              {absence.decisionNote}
            </p>
          )}
        </div>
        <div className="flex items-center gap-3">
          <StatusColorBadge label={meta.label} color={meta.color} />
          {cancelable && (
            <Button
              type="button"
              variant="ghost"
              size="compact"
              onClick={() => onCancel(absence)}
              className="text-moto-red hover:text-moto-red-hover px-0 hover:bg-transparent"
            >
              Stornieren
            </Button>
          )}
        </div>
      </div>
      {showResubmit && onResubmitted && (
        <ResubmitAbsenceForm absence={absence} onResubmitted={onResubmitted} />
      )}
    </li>
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
  // Fehler stehen am Formular (Alert oben, Feldfehler am Feld), nicht als
  // Toast: Bauart 2 Regel 5.
  const formRef = useRef<HTMLDivElement>(null);
  const formErrors = useApiFormError(formRef);
  // „Wiederholen“ sendet die aktuelle Antwort.
  const latestSubmitRef = useRef<() => Promise<void>>(async () => undefined);
  const toast = useToast();

  const handleSubmit = async () => {
    formErrors.clear();
    if (note.trim().length < 3) {
      formErrors.invalid("Bitte prüfen Sie das markierte Feld.", {
        note: "Bitte geben Sie eine kurze Antwort ein.",
      });
      return;
    }
    setSubmitting(true);
    try {
      await timeTrackingService.resubmitAbsence(absence.id, note.trim());
    } catch (err) {
      logger.error("resubmit_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await formErrors.show(err, {
        object: "die Antwort",
        retry: () => void latestSubmitRef.current(),
      });
      return;
    } finally {
      setSubmitting(false);
    }
    toast.success("Ihre Antwort ist gesendet.");
    onResubmitted();
  };
  useLayoutEffect(() => {
    latestSubmitRef.current = handleSubmit;
  });

  return (
    <div ref={formRef} className="mt-3 border-t border-gray-100 pt-3">
      <FormErrorAlert message={formErrors.error} className="mb-3" />
      <label
        htmlFor={`resubmit-note-${absence.id}`}
        className="mb-1 block text-xs font-semibold tracking-wider text-gray-500 uppercase"
      >
        Ihre Antwort
      </label>
      <Textarea
        id={`resubmit-note-${absence.id}`}
        name="note"
        value={note}
        onChange={(e) => setNote(e.target.value)}
        rows={2}
        maxLength={500}
        placeholder="Antwort auf die Rückfrage ergänzen…"
        error={formErrors.fieldError("note")}
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
  // Brand hexes from LOCATION_COLORS (green GROUP_ROOM, orange SCHOOLYARD in
  // its darkened Alert foreground) — never generic Tailwind hues.
  const valueClass = {
    primary: "text-gray-900",
    success: "text-moto-green-hover",
    amber: "text-moto-amber-strong",
    muted: "text-gray-400",
  }[tone];
  return (
    <div className="rounded-2xl border border-gray-200 bg-gray-50 p-4">
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
