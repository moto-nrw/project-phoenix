"use client";

import { useEffect, useLayoutEffect, useRef, useState } from "react";

import { useApiLoadError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import type { DraftRoster } from "~/lib/offering-suggestion";
import type {
  OfferingPreview,
  OfferingSelection,
} from "~/lib/roster-deviation";
import { timetableService } from "~/lib/timetable-api";

const logger = createLogger({ component: "useOfferingPreview" });

/**
 * Loads a preview whenever its key changes; a null key loads nothing. A
 * failed load is shown where the preview is missing, with a retry.
 */
function usePreviewLoad(
  key: string | null,
  load: () => Promise<OfferingPreview>,
  logFields: Record<string, unknown>,
) {
  const [preview, setPreview] = useState<OfferingPreview | null>(null);
  const [loading, setLoading] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const loadError = useApiLoadError();
  const { show, clear } = loadError;
  // The loader closes over the current selection; the key says when it
  // changed, so the effect never reloads on a new function identity.
  const loadRef = useRef(load);
  const logRef = useRef(logFields);
  useLayoutEffect(() => {
    loadRef.current = load;
    logRef.current = logFields;
  });

  useEffect(() => {
    if (key === null) {
      setPreview(null);
      return;
    }
    let active = true;
    setLoading(true);
    clear();
    loadRef
      .current()
      .then((result) => {
        if (active) setPreview(result);
      })
      .catch((err: unknown) => {
        logger.error("offering_preview_failed", {
          ...logRef.current,
          error: err instanceof Error ? err.message : String(err),
        });
        if (!active) return;
        setPreview(null);
        void show(err, {
          object: "die Vorschau der Kinderliste",
          retry: () => setAttempt((value) => value + 1),
        });
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [key, attempt, show, clear]);

  return { preview, loading, error: loadError.error };
}

function selectionKey(selection: OfferingSelection): string {
  return [
    selection.offeringIds.join(","),
    selection.gradeLevels.join(","),
    selection.schoolClasses.join(","),
  ].join("|");
}

/**
 * Loads who joins and who leaves a Regeltermin per weekday when it takes its
 * children from the chosen offerings (#5103).
 */
export function useOfferingPreview(
  templateId: string | undefined,
  selection: OfferingSelection,
  from?: string,
) {
  const key =
    templateId && selection.offeringIds.length > 0
      ? `${templateId}|${selectionKey(selection)}|${from ?? ""}`
      : null;
  return usePreviewLoad(
    key,
    () =>
      from
        ? timetableService.getOfferingPreview(templateId!, selection, from)
        : timetableService.getOfferingPreview(templateId!, selection),
    { template_id: templateId },
  );
}

/**
 * The same preview for a Regeltermin that is not saved yet (#5104): it
 * compares the offering's children with the list the editor planned before
 * the switch.
 */
export function useDraftOfferingPreview(
  periodId: string,
  selection: OfferingSelection,
  roster: DraftRoster | null,
  from?: string,
) {
  const key =
    periodId && roster && selection.offeringIds.length > 0
      ? `${periodId}|${selectionKey(selection)}|${JSON.stringify(roster)}|${from ?? ""}`
      : null;
  return usePreviewLoad(
    key,
    () =>
      timetableService.getDraftOfferingPreview(
        periodId,
        selection,
        roster ?? {},
        from,
      ),
    { period_id: periodId },
  );
}
