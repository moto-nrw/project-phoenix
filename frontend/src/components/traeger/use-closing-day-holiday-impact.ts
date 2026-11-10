"use client";

/**
 * Lädt zum Entwurf eines Träger-Schließtags, welche Ferienbuchungen er betrifft
 * (#6434). Der Dialog zeigt das vor dem Speichern; das Speichern bestätigt mit
 * dem Zeichen der zuletzt gezeigten Vorschau. Eine ältere Antwort überschreibt
 * nie eine neuere.
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

export function useClosingDayHolidayImpact(open: boolean) {
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

  useEffect(() => {
    if (!open || !completeDraft(draft)) {
      latest.current += 1;
      setImpact(null);
      setLoading(false);
      clearLoadError();
      return;
    }
    const request = ++latest.current;
    setLoading(true);
    const timer = window.setTimeout(() => {
      previewTraegerClosingDayHolidayBookings({ ...draft, reason: "" })
        .then((loaded) => {
          if (request !== latest.current) return;
          clearLoadError();
          setImpact(loaded);
        })
        .catch((err: unknown) => {
          if (request !== latest.current) return;
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
          if (request === latest.current) setLoading(false);
        });
    }, PREVIEW_DELAY_MS);
    return () => window.clearTimeout(timer);
  }, [open, draft, generation, clearLoadError, showLoadError]);

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
    blocked: completeDraft(draft) && (loading || impact === null),
  };
}
