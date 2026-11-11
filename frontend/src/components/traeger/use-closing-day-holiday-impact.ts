"use client";

/**
 * Lädt zum Entwurf eines Träger-Schließtags, welche Ferienbuchungen er betrifft
 * (#6434). Der Dialog zeigt das vor dem Speichern; das Speichern bestätigt mit
 * dem Zeichen der zuletzt gezeigten Vorschau. Eine ältere Antwort überschreibt
 * nie eine neuere. Beim Ändern und Löschen schickt die Vorschau den
 * gespeicherten Schließtag mit; dann nennt sie auch die Tage, die wieder
 * öffnen (#6457). Das Löschen braucht keinen Entwurf.
 */

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";

import type { ClosingDayDraft } from "~/components/planning/closing-day-modal";
import { useApiLoadError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  type HolidayClosureImpact,
  previewTraegerClosingDayHolidayBookings,
  type TraegerClosingDay,
  type TraegerClosingDayChange,
} from "~/lib/traeger-closing-days";

const logger = createLogger({ component: "useClosingDayHolidayImpact" });

/** Wartezeit nach der letzten Änderung, bevor die Vorschau lädt. */
const PREVIEW_DELAY_MS = 300;

function completeDraft(
  draft: ClosingDayDraft | null,
): draft is ClosingDayDraft {
  return (
    draft !== null &&
    draft.startDate !== "" &&
    draft.endDate !== "" &&
    draft.endDate >= draft.startDate &&
    draft.schoolIds.length > 0
  );
}

/**
 * `saved` ist der gespeicherte Schließtag beim Ändern oder Löschen, `null`
 * beim Anlegen. Mit `mode: "delete"` lädt die Vorschau zum Löschen von
 * `saved`, sobald `open` gilt.
 */
export function useClosingDayHolidayImpact(
  open: boolean,
  saved: TraegerClosingDay | null = null,
  mode: "save" | "delete" = "save",
) {
  const [draft, setDraft] = useState<ClosingDayDraft | null>(null);
  const [impact, setImpact] = useState<HolidayClosureImpact | null>(null);
  const [loading, setLoading] = useState(false);
  const [generation, setGeneration] = useState(0);
  const loadError = useApiLoadError();
  const { clear: clearLoadError, show: showLoadError } = loadError;
  const latest = useRef(0);
  const retryRef = useRef<() => void>(() => undefined);
  useLayoutEffect(() => {
    retryRef.current = () => setGeneration((value) => value + 1);
  });

  const deleting = mode === "delete";
  const request: ClosingDayDraft | null = deleting ? saved : draft;
  useEffect(() => {
    if (!open || !completeDraft(request)) {
      latest.current += 1;
      setImpact(null);
      setLoading(false);
      clearLoadError();
      return;
    }
    const call = ++latest.current;
    const change: TraegerClosingDayChange | undefined = saved
      ? { closingDay: saved, delete: deleting }
      : undefined;
    setLoading(true);
    const timer = window.setTimeout(
      () => {
        previewTraegerClosingDayHolidayBookings(
          { ...request, reason: "" },
          change,
        )
          .then((loaded) => {
            if (call !== latest.current) return;
            clearLoadError();
            setImpact(loaded);
          })
          .catch((err: unknown) => {
            if (call !== latest.current) return;
            logger.error("traeger_closing_day_holiday_preview_failed", {
              error: err instanceof Error ? err.message : String(err),
            });
            setImpact(null);
            void showLoadError(err, {
              object: "die gebuchte Ferienbetreuung",
              retry: () => retryRef.current(),
            });
          })
          .finally(() => {
            if (call === latest.current) setLoading(false);
          });
      },
      deleting ? 0 : PREVIEW_DELAY_MS,
    );
    return () => window.clearTimeout(timer);
  }, [
    open,
    request,
    saved,
    deleting,
    generation,
    clearLoadError,
    showLoadError,
  ]);

  const onDraftChange = useCallback(
    (next: ClosingDayDraft) => setDraft(next),
    [],
  );
  const reload = useCallback(() => setGeneration((value) => value + 1), []);

  return {
    onDraftChange,
    impact,
    loading,
    error: loadError.error,
    reload,
    /** Das Speichern wartet, bis die Vorschau zum Entwurf geladen ist. */
    blocked: completeDraft(request) && (loading || impact === null),
  };
}
