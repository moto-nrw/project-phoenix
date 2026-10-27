"use client";

import { useEffect, useState } from "react";

import { useApiLoadError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import type {
  OfferingPreview,
  OfferingSelection,
} from "~/lib/roster-deviation";
import { timetableService } from "~/lib/timetable-api";

const logger = createLogger({ component: "useOfferingPreview" });

/**
 * Loads who joins and who leaves a Regeltermin per weekday when it takes its
 * children from the chosen offerings (#5103). A failed load is shown where
 * the preview is missing, with a retry.
 */
export function useOfferingPreview(
  templateId: string | undefined,
  selection: OfferingSelection,
  from?: string,
) {
  const [preview, setPreview] = useState<OfferingPreview | null>(null);
  const [loading, setLoading] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const load = useApiLoadError();
  const { show, clear } = load;
  const offeringKey = selection.offeringIds.join(",");
  const gradeKey = selection.gradeLevels.join(",");
  const classKey = selection.schoolClasses.join(",");

  useEffect(() => {
    if (!templateId || offeringKey === "") {
      setPreview(null);
      return;
    }
    let active = true;
    setLoading(true);
    clear();
    const current: OfferingSelection = {
      offeringIds: offeringKey.split(","),
      gradeLevels: gradeKey === "" ? [] : gradeKey.split(",").map(Number),
      schoolClasses: classKey === "" ? [] : classKey.split(","),
    };
    const request = from
      ? timetableService.getOfferingPreview(templateId, current, from)
      : timetableService.getOfferingPreview(templateId, current);
    request
      .then((result) => {
        if (active) setPreview(result);
      })
      .catch((err: unknown) => {
        logger.error("offering_preview_failed", {
          template_id: templateId,
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
  }, [templateId, offeringKey, gradeKey, classKey, from, attempt, show, clear]);

  return { preview, loading, error: load.error };
}
