"use client";

import { useCallback, useEffect, useState } from "react";
import { useApiLoadError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { fetchTraegerSchools, type TraegerSchool } from "~/lib/traeger-api";

const logger = createLogger({ component: "useTraegerSchools" });

/** The schools of the Träger, the hosts a holiday week may choose (#4659). */
export function useTraegerSchools() {
  const [schools, setSchools] = useState<TraegerSchool[] | null>(null);
  const loadError = useApiLoadError();
  const { show, clear } = loadError;

  const load = useCallback(async () => {
    try {
      setSchools(await fetchTraegerSchools());
      clear();
    } catch (err) {
      logger.error("traeger_schools_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await show(err, {
        object: "die Liste der Schulen",
        retry: () => void load(),
      });
    }
  }, [clear, show]);

  useEffect(() => {
    void load();
  }, [load]);

  return { schools, error: loadError.error };
}
