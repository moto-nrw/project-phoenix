import { useCallback, useState } from "react";

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
 * `onFailure` lädt die Tagesdaten neu: scheitert das Starten, hat meist ein
 * anderer den Block schon gestartet oder das Zeitfenster ist vorbei, und die
 * Anzeige soll das zeigen statt eines Knopfs, der wieder scheitert.
 */
export function useStartOwnBlock({
  onFailure,
}: {
  readonly onFailure: () => Promise<unknown>;
}) {
  const router = useTenantRouter();
  const [busyId, setBusyId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const start = useCallback(
    async (instance: PlannedTimetableInstance) => {
      setBusyId(instance.id);
      setError(null);
      try {
        const result = await timetableOperationsApi.start(instance.id);
        router.push(`/active-supervisions?session=${result.activeGroupId}`);
      } catch (err) {
        logger.error("home_block_start_failed", {
          instance_id: instance.id,
          error: err instanceof Error ? err.message : String(err),
        });
        setError(
          "Der Block konnte nicht gestartet werden. Bitte noch einmal versuchen.",
        );
        await onFailure();
      } finally {
        setBusyId(null);
      }
    },
    [onFailure, router],
  );

  return { start, busyId, error };
}
