"use client";

/**
 * „Termine absagen“ (#3594): sagt alle geplanten Termine eines Zeitraums ab
 * und nimmt sie aus dem Plan, etwa wenn ein Schließtag erst nach der Planung
 * eingetragen wurde. Zuerst zählt moto die betroffenen Termine (dry_run),
 * dann folgt die zweistufige Bestätigung des geteilten Löschdialogs.
 *
 * Abgesagt werden nur geplante Termine ab heute. Laufende, beendete und
 * vergangene Termine bleiben, ebenso Serien, die bewusst auch an
 * Schließtagen geplant sind (Ferienbetreuung). Eltern bekommen keine
 * Nachricht.
 */

import { useEffect, useState } from "react";
import type { DateRange } from "react-day-picker";

import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { DateRangePicker } from "~/components/ui/date-range-picker";
import { useToast } from "~/contexts/ToastContext";
import {
  berlinTodayISO,
  formatDate,
  parseISODate,
  toISODate,
} from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { useTenantMutateMatching } from "~/lib/swr";
import { timetableService } from "~/lib/timetable-api";

const logger = createLogger({ component: "BulkCancelAppointmentsModal" });

/** SWR-Präfixe der Planflächen, die nach dem Absagen neu laden. */
const PLAN_CACHE_PREFIXES = [
  "timetable-week-",
  "timetable-month-",
  "timetable-day-",
  "timetable-gaps-",
] as const;

type CountState =
  | { kind: "idle" }
  | { kind: "loading" }
  | { kind: "ready"; count: number }
  | { kind: "error" };

function appointmentsLabel(count: number): string {
  return count === 1
    ? "1 geplanter Termin wird"
    : `${count} geplante Termine werden`;
}

export function BulkCancelAppointmentsModal({
  isOpen,
  initialFrom,
  initialTo,
  rangeEditable = false,
  onClose,
  onCancelled,
}: {
  readonly isOpen: boolean;
  /** Erster Tag des Zeitraums als "YYYY-MM-DD". */
  readonly initialFrom: string;
  /** Letzter Tag des Zeitraums als "YYYY-MM-DD". */
  readonly initialTo: string;
  /** Zeigt eine Zeitraumauswahl, z. B. im Betreuungsplan. */
  readonly rangeEditable?: boolean;
  readonly onClose: () => void;
  readonly onCancelled?: (count: number) => void;
}) {
  const [from, setFrom] = useState(initialFrom);
  const [to, setTo] = useState(initialTo);
  const [countState, setCountState] = useState<CountState>({ kind: "idle" });
  const [cancelling, setCancelling] = useState(false);
  const [error, setError] = useState("");
  const { success: toastSuccess } = useToast();
  const refreshPlan = useTenantMutateMatching(PLAN_CACHE_PREFIXES);

  useEffect(() => {
    if (!isOpen) return;
    setFrom(initialFrom);
    setTo(initialTo);
    setError("");
  }, [isOpen, initialFrom, initialTo]);

  useEffect(() => {
    if (!isOpen || from === "" || to === "") {
      setCountState({ kind: "idle" });
      return;
    }
    let active = true;
    setCountState({ kind: "loading" });
    timetableService
      .bulkCancel(from, to, true)
      .then((preview) => {
        if (active) setCountState({ kind: "ready", count: preview.count });
      })
      .catch((err: unknown) => {
        logger.error("bulk_cancel_preview_failed", {
          from,
          to,
          error: err instanceof Error ? err.message : String(err),
        });
        if (active) setCountState({ kind: "error" });
      });
    return () => {
      active = false;
    };
  }, [isOpen, from, to]);

  const handleRangeChange = (range: DateRange | undefined) => {
    setFrom(range?.from ? toISODate(range.from) : "");
    setTo(range?.to ? toISODate(range.to) : "");
  };

  const handleConfirm = async () => {
    setCancelling(true);
    setError("");
    try {
      const result = await timetableService.bulkCancel(from, to, false);
      toastSuccess(
        result.count === 1
          ? "1 Termin abgesagt"
          : `${result.count} Termine abgesagt`,
      );
      await refreshPlan();
      onCancelled?.(result.count);
      onClose();
    } catch (err) {
      logger.error("bulk_cancel_failed", {
        from,
        to,
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.",
      );
    } finally {
      setCancelling(false);
    }
  };

  const rangeText =
    from !== "" && to !== ""
      ? from === to
        ? formatDate(from)
        : `${formatDate(from)} bis ${formatDate(to)}`
      : "";

  return (
    <ConfirmDeleteModal
      isOpen={isOpen}
      title="Termine absagen"
      description={
        <div className="flex flex-col gap-3">
          {rangeEditable ? (
            <div className="flex flex-col gap-1">
              <span className="text-sm font-medium text-gray-700">
                Zeitraum
              </span>
              <DateRangePicker
                value={
                  from !== "" && to !== ""
                    ? { from: parseISODate(from), to: parseISODate(to) }
                    : undefined
                }
                onChange={handleRangeChange}
                fromMin={parseISODate(berlinTodayISO())}
              />
            </div>
          ) : (
            rangeText !== "" && <p>Zeitraum: {rangeText}</p>
          )}
          {countState.kind === "loading" && <p>Die Termine werden gezählt.</p>}
          {countState.kind === "error" && (
            <p>
              Die Termine konnten nicht gezählt werden. Bitte versuchen Sie es
              noch einmal.
            </p>
          )}
          {countState.kind === "ready" && countState.count === 0 && (
            <p>In diesem Zeitraum sind keine Termine mehr geplant.</p>
          )}
          {countState.kind === "ready" && countState.count > 0 && (
            <>
              <p>
                {appointmentsLabel(countState.count)} abgesagt und aus dem Plan
                entfernt. Eltern bekommen keine Nachricht.
              </p>
              <p>
                Serien, die auch an Schließtagen geplant sind, bleiben. Zum
                Beispiel die Ferienbetreuung.
              </p>
            </>
          )}
        </div>
      }
      gate={{ mode: "twoStep", firstStepLabel: "Termine absagen" }}
      confirmDisabled={
        countState.kind !== "ready" || countState.count === 0 || cancelling
      }
      onConfirm={handleConfirm}
      onClose={onClose}
      loading={cancelling}
      error={error}
      confirmLabel="Endgültig absagen"
      loadingLabel="Wird abgesagt …"
    />
  );
}
