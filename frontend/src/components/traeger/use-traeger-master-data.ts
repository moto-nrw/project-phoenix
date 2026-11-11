"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useApiLoadError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  fetchTraegerMasterData,
  type TraegerMasterData,
} from "~/lib/traeger-api";

const logger = createLogger({ component: "TraegerMasterData" });

const EMPTY: TraegerMasterData = {
  contact: { name: "", phone: "", email: "" },
  withdrawal: { phone: "", email: "" },
  logo: null,
  bankAccounts: [],
};

/**
 * Die Stammdaten des Trägers (#4653) für die Einstellungsseiten, die sie zeigen.
 * `loaded` gilt erst nach einem vollständigen Lesen: Leere Formulare nach einem
 * Fehler würden gespeicherte Werte überschreiben. Ein fehlgeschlagenes
 * Nachladen lässt die gezeigten Werte stehen und meldet es.
 */
export function useTraegerMasterData(object: string, enabled = true) {
  const [masterData, setMasterData] = useState<TraegerMasterData>(EMPTY);
  const [loading, setLoading] = useState(true);
  const [failure, setFailure] = useState<unknown>();
  const [loaded, setLoaded] = useState(false);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;
  const request = useRef(0);

  const read = useCallback(async () => {
    const current = ++request.current;
    try {
      const data = await fetchTraegerMasterData();
      if (current !== request.current) return;
      setMasterData(data);
      setFailure(undefined);
      setLoaded(true);
      clearLoadError();
    } catch (err) {
      if (current === request.current) throw err;
    } finally {
      if (current === request.current) setLoading(false);
    }
  }, [clearLoadError]);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      await read();
    } catch (err) {
      setFailure(err);
      logger.error("traeger_master_data_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, { object, retry: () => void load() });
    }
  }, [object, read, showLoadError]);

  const refresh = useCallback(async () => {
    try {
      await read();
    } catch (err) {
      setFailure(err);
      logger.warn("traeger_master_data_refresh_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, { object, retry: () => void load() });
    }
  }, [load, object, read, showLoadError]);

  useEffect(() => {
    if (!enabled) return;
    void load();
  }, [enabled, load]);

  return {
    masterData,
    loaded,
    loading,
    failure,
    error: pageLoad.error,
    refresh,
  };
}
