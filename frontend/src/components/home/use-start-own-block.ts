import { useCallback, useLayoutEffect, useRef, useState } from "react";

import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { errorStatus } from "~/lib/expected-failure";
import { createLogger } from "~/lib/logger";
import { useTenantRouter } from "~/lib/tenant-router";
import { timetableOperationsApi } from "~/lib/timetable-operations-api";
import type { PlannedTimetableInstance } from "~/lib/timetable-operations-types";

const logger = createLogger({ component: "HomeStartOwnBlock" });

/**
 * Einen eigenen Block starten und direkt in seine Kinderliste springen
 * (#2180). „Mein Tag" und die Jetzt-Zone starten auf demselben Weg: zwei
 * Knöpfe, die dasselbe tun, dürfen nicht an zwei Stellen verschieden
 * scheitern.
 *
 * Ein Fehler kommt als Toast über den gemeinsamen Anzeigeweg (#2517), mit
 * Wiederholen, wenn ein zweiter Versuch helfen kann. `onFailure` lädt die
 * Tagesdaten neu: scheitert das Starten, hat meist ein anderer den Block
 * schon gestartet oder das Zeitfenster ist vorbei, und die Anzeige soll das
 * zeigen statt eines Knopfs, der wieder scheitert.
 */
export function useStartOwnBlock({
  onFailure,
}: {
  readonly onFailure: () => Promise<unknown>;
}) {
  const router = useTenantRouter();
  const { show } = useApiErrorDisplay();
  const [busyId, setBusyId] = useState<string | null>(null);
  const startRef = useRef<(instance: PlannedTimetableInstance) => void>(
    () => undefined,
  );

  const start = useCallback(
    async (instance: PlannedTimetableInstance) => {
      setBusyId(instance.id);
      try {
        const result = await timetableOperationsApi.start(instance.id);
        router.push(`/active-supervisions?session=${result.activeGroupId}`);
      } catch (err) {
        logger.error("home_block_start_failed", {
          instance_id: instance.id,
          error: err instanceof Error ? err.message : String(err),
          status: errorStatus(err),
        });
        void show(err, {
          object: "das Starten des Blocks",
          retry: () => startRef.current(instance),
        });
        await onFailure();
      } finally {
        setBusyId(null);
      }
    },
    [onFailure, router, show],
  );

  // Wiederholen startet mit dem aktuellen Stand des Hooks.
  useLayoutEffect(() => {
    startRef.current = (instance) => void start(instance);
  });

  return { start, busyId };
}
