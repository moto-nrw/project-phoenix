"use client";

import { useEffect, useState } from "react";
import { useApiLoadError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { externalStaffNote } from "~/lib/staff-helpers";
import { substitutionService } from "~/lib/substitution-api";
import type { RunningSupervision } from "~/lib/substitution-helpers";

const logger = createLogger({ component: "RunningSupervision" });

export type SupervisionTarget = RunningSupervision["availableTargets"][number];

/**
 * Loads the running supervision a dialog acts on while it is open, with the
 * load failure and its „Wiederholen“. `object` names it in the error text.
 */
export function useRunningSupervision(
  activeGroupId: string | null,
  isOpen: boolean,
  object: string,
) {
  const [overview, setOverview] = useState<RunningSupervision | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  // Erhöht von „Wiederholen“: lädt die Aufsicht erneut.
  const [attempt, setAttempt] = useState(0);
  const {
    error: loadError,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();
  useEffect(() => {
    if (!isOpen || !activeGroupId) return;
    let current = true;
    setOverview(null);
    clearLoadError();
    setIsLoading(true);
    void substitutionService
      .fetchRunningSupervision(activeGroupId)
      .then((result) => current && setOverview(result))
      .catch((cause: unknown) => {
        logger.error("running_supervision_load_failed", {
          error: cause instanceof Error ? cause.message : String(cause),
        });
        if (current)
          void showLoadError(cause, {
            object,
            retry: () => setAttempt((value) => value + 1),
          });
      })
      .finally(() => current && setIsLoading(false));
    return () => {
      current = false;
    };
  }, [activeGroupId, isOpen, attempt, object, showLoadError, clearLoadError]);
  return { overview, loadError, isLoading };
}

// Wie in den anderen Auswahllisten: „Name (Extern · Musikschule, Einladung
// offen)“ (#4573). Externe mit jedem Zugang kommen hier an (ADR 0091).
export function supervisionTargetLabel(target: SupervisionTarget): string {
  const note = externalStaffNote(target);
  return note ? `${target.fullName} (${note})` : target.fullName;
}
