"use client";

import { useCallback, useEffect, useState } from "react";
import {
  type DebtorNumberFormat,
  NO_DEBTOR_NUMBER_FORMAT,
} from "~/lib/debtor-number-format";
import { createLogger } from "~/lib/logger";
import {
  type DebtorNumberSettings,
  fetchDebtorNumberSettings,
} from "~/lib/traeger-billing-families-api";

const logger = createLogger({ component: "useDebtorNumberFormat" });

interface DebtorNumberSettingsState {
  /** The settings, null until read or after a failed read. */
  readonly settings: DebtorNumberSettings | null;
  /** The failed read; null while it loads or once it succeeded. */
  readonly error: unknown;
  /** Reads the settings again. */
  readonly reload: () => void;
}

/**
 * The format of the Träger's debtor numbers (#6675) and whom they name
 * (#7007). A failed read stays unresolved: guessing the scope would show
 * the numbers of the wrong store (#7007), so the page shows the error.
 */
export function useDebtorNumberSettings(): DebtorNumberSettingsState {
  const [settings, setSettings] = useState<DebtorNumberSettings | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    let current = true;
    fetchDebtorNumberSettings()
      .then((loaded) => {
        if (!current) return;
        setSettings(loaded);
        setError(null);
      })
      .catch((err: unknown) => {
        logger.warn("traeger_debtor_number_format_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (current) setError(err);
      });
    return () => {
      current = false;
    };
  }, [attempt]);
  const reload = useCallback(() => {
    setError(null);
    setAttempt((count) => count + 1);
  }, []);
  return { settings, error, reload };
}

/**
 * The format of the Träger's debtor numbers (#6675) for a family's fields.
 * It only shapes the field: without it the backend still checks every
 * number, so a failed read keeps the plain hint.
 */
export function useDebtorNumberFormat(): DebtorNumberFormat {
  return useDebtorNumberSettings().settings?.format ?? NO_DEBTOR_NUMBER_FORMAT;
}
