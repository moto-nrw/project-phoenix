"use client";

/**
 * „Termine absagen“ (#3594): sagt alle geplanten Termine eines Zeitraums ab
 * und nimmt sie aus dem Plan, etwa wenn ein Schließtag erst nach der Planung
 * eingetragen wurde. Zuerst zählt moto die betroffenen Termine (dry_run),
 * dann folgt die zweistufige Bestätigung des geteilten Löschdialogs.
 *
 * Abgesagt werden nur geplante Termine ab heute. Laufende, beendete und
 * vergangene Termine bleiben. Serien, die bewusst auch an Schließtagen
 * geplant sind (Ferienbetreuung), bleiben ebenfalls, bis die Person das
 * Häkchen „Auch diese Serien absagen“ setzt; der Dialog nennt sie mit Namen.
 * Eltern bekommen keine Nachricht.
 */

import { useEffect, useId, useState } from "react";
import type { DateRange } from "react-day-picker";

import { Checkbox } from "~/components/ui/checkbox";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { DateRangePicker } from "~/components/ui/date-range-picker";
import { useToast } from "~/contexts/ToastContext";
import { formatClosingDayRange } from "~/lib/closing-day-helpers";
import { berlinTodayISO, parseISODate, toISODate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { useTenantMutateMatching } from "~/lib/swr";
import { timetableService } from "~/lib/timetable-api";
import type { BulkCancelResult } from "~/lib/timetable-types";

const logger = createLogger({ component: "BulkCancelAppointmentsModal" });

/** SWR-Präfixe der Planflächen, die nach dem Absagen neu laden. */
const PLAN_CACHE_PREFIXES = [
  "timetable-week-",
  "timetable-month-",
  "timetable-day-",
  "timetable-gaps-",
  // Planungszeiträume des Betreuungsplans: die Verwendung zählt Termine.
  "database-calendar-periods-list",
] as const;

type CountState =
  | { kind: "idle" }
  | { kind: "loading" }
  | ({ kind: "ready" } & Pick<BulkCancelResult, "count" | "keptSeries">)
  | { kind: "error" };

function termineLabel(count: number): string {
  return count === 1 ? "1 Termin" : `${count} Termine`;
}

function appointmentsLabel(count: number): string {
  return count === 1 ? "wird 1 Termin" : `werden ${count} Termine`;
}

/** „Ferienbetreuung (5 Termine), Ferienspiele (1 Termin)“ */
function keptSeriesLabel(series: BulkCancelResult["keptSeries"]): string {
  return series
    .map((entry) => `${entry.name} (${termineLabel(entry.count)})`)
    .join(", ");
}

/**
 * Ein Zeitraum, der schon begonnen hat, startet heute: vergangene Termine
 * sagt moto ohnehin nicht ab, und die Auswahl erlaubt keine früheren Tage.
 */
function clampToToday(from: string, to: string): string {
  const today = berlinTodayISO();
  return from !== "" && from < today && to >= today ? today : from;
}

export function BulkCancelAppointmentsModal({
  isOpen,
  initialFrom,
  initialTo,
  afterSave = false,
  onClose,
  onCancelled,
}: {
  /** Direkt nach dem Speichern eines Schließtags geöffnet (#3594). */
  readonly afterSave?: boolean;
  readonly isOpen: boolean;
  /** Vorgeschlagener erster Tag als "YYYY-MM-DD"; die Person kann ihn ändern. */
  readonly initialFrom: string;
  /** Vorgeschlagener letzter Tag als "YYYY-MM-DD"; die Person kann ihn ändern. */
  readonly initialTo: string;
  readonly onClose: () => void;
  readonly onCancelled?: (count: number) => void;
}) {
  const [from, setFrom] = useState(initialFrom);
  const [to, setTo] = useState(initialTo);
  const [includeSeries, setIncludeSeries] = useState(false);
  const [countState, setCountState] = useState<CountState>({ kind: "idle" });
  const [cancelling, setCancelling] = useState(false);
  const [error, setError] = useState("");
  const { success: toastSuccess } = useToast();
  const refreshPlan = useTenantMutateMatching(PLAN_CACHE_PREFIXES);
  const rangeLabelId = useId();
  const includeSeriesId = useId();

  useEffect(() => {
    if (!isOpen) return;
    setFrom(clampToToday(initialFrom, initialTo));
    setTo(initialTo);
    setIncludeSeries(false);
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
      .bulkCancel(from, to, true, includeSeries)
      .then((preview) => {
        if (active) {
          setCountState({
            kind: "ready",
            count: preview.count,
            keptSeries: preview.keptSeries,
          });
        }
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
  }, [isOpen, from, to, includeSeries]);

  const handleRangeChange = (range: DateRange | undefined) => {
    setFrom(range?.from ? toISODate(range.from) : "");
    setTo(range?.to ? toISODate(range.to) : "");
  };

  const handleConfirm = async () => {
    setCancelling(true);
    setError("");
    try {
      const result = await timetableService.bulkCancel(
        from,
        to,
        false,
        includeSeries,
      );
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

  // „Im Zeitraum 12.10.2026 – 16.10.2026“, ein einzelner Tag „Am 12.10.2026“.
  const rangeText = `${from === to ? "Am" : "Im Zeitraum"} ${formatClosingDayRange({ startDate: from, endDate: to })}`;
  const ready = countState.kind === "ready" ? countState : null;
  const keptSeries = ready?.keptSeries ?? [];
  // Das Häkchen bleibt sichtbar, solange es gesetzt ist: nach dem Neuzählen
  // bleibt keine Serie mehr übrig, die Person muss es aber abwählen können.
  const showIncludeSeries = includeSeries || keptSeries.length > 0;

  return (
    <ConfirmDeleteModal
      isOpen={isOpen}
      title="Termine im Zeitraum absagen"
      description={
        <div className="flex flex-col gap-3">
          {afterSave && (
            <p>Der Schließtag ist gespeichert. Es sind noch Termine geplant.</p>
          )}
          <div
            role="group"
            aria-labelledby={rangeLabelId}
            className="flex flex-col gap-1"
          >
            <span
              id={rangeLabelId}
              className="text-sm font-medium text-gray-700"
            >
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
          {countState.kind === "loading" && <p>Die Termine werden gezählt.</p>}
          {countState.kind === "error" && (
            <p>
              Die Termine konnten nicht gezählt werden. Bitte versuchen Sie es
              noch einmal.
            </p>
          )}
          {ready && ready.count === 0 && keptSeries.length === 0 && (
            <p>{rangeText} sind keine Termine mehr geplant.</p>
          )}
          {ready && ready.count === 0 && keptSeries.length > 0 && (
            <p>
              {rangeText} sind nur noch Serien geplant, die auch an Schließtagen
              stattfinden: {keptSeriesLabel(keptSeries)}. Zum Absagen setzen Sie
              unten das Häkchen.
            </p>
          )}
          {ready && ready.count > 0 && (
            <p>
              {rangeText} {appointmentsLabel(ready.count)} abgesagt. Eltern
              bekommen keine Nachricht.
            </p>
          )}
          {ready && ready.count > 0 && keptSeries.length > 0 && (
            <p>
              Serien, die auch an Schließtagen stattfinden, bleiben:{" "}
              {keptSeriesLabel(keptSeries)}.
            </p>
          )}
          {showIncludeSeries && (
            <label htmlFor={includeSeriesId} className="flex items-start gap-2">
              <Checkbox
                id={includeSeriesId}
                checked={includeSeries}
                disabled={cancelling}
                onChange={(event) => setIncludeSeries(event.target.checked)}
              />
              <span className="text-sm text-gray-800">
                Auch diese Serien absagen
              </span>
            </label>
          )}
        </div>
      }
      gate={{ mode: "twoStep", firstStepLabel: "Termine absagen" }}
      cancelLabel="Termine behalten"
      confirmDisabled={!ready || ready.count === 0 || cancelling}
      onConfirm={handleConfirm}
      onClose={onClose}
      loading={cancelling}
      error={error}
      confirmLabel="Endgültig absagen"
      loadingLabel="Wird abgesagt…"
    />
  );
}
