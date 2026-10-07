"use client";

// Stundenkonto-Vorschau für einen geplanten Freizeitausgleich (#2873).
// Informiert vor dem Eintragen; ein negativer Stand wird bestätigt, nicht
// verhindert. Seit #3256 Teil des gemeinsamen Dialogs „Abwesenheit eintragen".

import { useCallback, useEffect, useState } from "react";

import { formatSignedDuration } from "~/components/staff/staff-time-views";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { useApiLoadError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  staffAbsenceService,
  type CompTimeBalancePreview,
} from "~/lib/staff-api";

const logger = createLogger({ component: "CompTimePreview" });

export function useCompTimePreview({
  enabled,
  staffId,
  dateStart,
  dateEnd,
  halfDay,
}: {
  readonly enabled: boolean;
  readonly staffId: string;
  readonly dateStart: string;
  readonly dateEnd: string;
  readonly halfDay: boolean;
}): {
  preview: CompTimeBalancePreview | null;
  loading: boolean;
  error: unknown;
  reload: () => void;
} {
  const [preview, setPreview] = useState<CompTimeBalancePreview | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [attempt, setAttempt] = useState(0);
  const reload = useCallback(() => setAttempt((value) => value + 1), []);

  useEffect(() => {
    // Sofort verwerfen: die alte Projektion darf während des Nachladens weder
    // angezeigt werden noch eine Buchung ohne Bestätigung durchlassen.
    setPreview(null);
    setError(null);
    if (!enabled || !dateStart || !dateEnd) {
      setLoading(false);
      return;
    }
    let stale = false;
    setLoading(true);
    staffAbsenceService
      .getCompTimePreview(staffId, { dateStart, dateEnd, halfDay })
      .then((result) => {
        if (!stale) setPreview(result);
      })
      .catch((err: unknown) => {
        if (stale) return;
        // Rein informativ: der Block zeigt den Ladefehler, die Buchung
        // selbst bleibt möglich und meldet ihre eigenen Fehler.
        logger.error("comp_time_preview_failed", {
          staff_id: staffId,
          error: err instanceof Error ? err.message : String(err),
        });
        setError(err);
      })
      .finally(() => {
        if (!stale) setLoading(false);
      });
    return () => {
      stale = true;
    };
  }, [enabled, staffId, dateStart, dateEnd, halfDay, attempt]);

  return { preview, loading, error, reload };
}

export function CompTimePreviewPanel({
  preview,
  loading,
  error = null,
  onRetry,
}: {
  readonly preview: CompTimeBalancePreview | null;
  readonly loading: boolean;
  readonly error?: unknown;
  readonly onRetry?: () => void;
}) {
  const load = useApiLoadError();
  const showLoadError = load.show;
  const clearLoadError = load.clear;
  useEffect(() => {
    if (error) {
      void showLoadError(error, {
        object: "die Vorschau für das Stundenkonto",
        retry: onRetry,
      });
    } else {
      clearLoadError();
    }
  }, [error, onRetry, showLoadError, clearLoadError]);

  if (error) return <LoadErrorAlert error={load.error} />;
  if (preview === null && !loading) return null;
  return (
    <div className="rounded-lg bg-gray-50 p-3">
      {preview === null ? (
        <p className="text-sm text-gray-500">Stundenkonto wird berechnet …</p>
      ) : (
        <>
          <DataGrid columns={1}>
            <DataField inline label="Stundenkonto aktuell">
              <span className="tabular-nums">
                {formatSignedDuration(preview.currentBalanceMinutes)}
              </span>
            </DataField>
            {preview.futureCommitmentMinutes > 0 && (
              <DataField inline label="Bereits geplanter Freizeitausgleich">
                <span className="tabular-nums">
                  {formatSignedDuration(-preview.futureCommitmentMinutes)}
                </span>
              </DataField>
            )}
            {preview.futureAdjustmentMinutes !== 0 && (
              <DataField inline label="Bereits geplante Buchungen">
                <span className="tabular-nums">
                  {formatSignedDuration(preview.futureAdjustmentMinutes)}
                </span>
              </DataField>
            )}
            <DataField inline label="Abzug für diesen Eintrag">
              <span className="tabular-nums">
                {formatSignedDuration(-preview.deductionMinutes)}
              </span>
            </DataField>
            <DataField inline label="Stundenkonto danach">
              <span
                className={`font-semibold tabular-nums ${
                  preview.projectedBalanceMinutes < 0
                    ? "text-moto-red-strong"
                    : ""
                }`}
              >
                {formatSignedDuration(preview.projectedBalanceMinutes)}
              </span>
            </DataField>
          </DataGrid>
          {preview.realizedDeductionMinutes > 0 && (
            <p className="mt-2 text-xs text-gray-500">
              Tage bis heute sind im aktuellen Stand schon enthalten.
            </p>
          )}
        </>
      )}
    </div>
  );
}
