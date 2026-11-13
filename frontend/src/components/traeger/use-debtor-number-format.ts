"use client";

import { useEffect, useState } from "react";
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

/**
 * The format of the Träger's debtor numbers (#6675) and whom they name
 * (#7007), null until read. The format only shapes the field: without it
 * the backend still checks every number, so a failed read stays in the log
 * and the page keeps the number per payer.
 */
export function useDebtorNumberSettings(): DebtorNumberSettings | null {
  const [settings, setSettings] = useState<DebtorNumberSettings | null>(null);
  useEffect(() => {
    let current = true;
    fetchDebtorNumberSettings()
      .then((loaded) => {
        if (current) setSettings(loaded);
      })
      .catch((err: unknown) => {
        logger.warn("traeger_debtor_number_format_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (current) {
          setSettings({ format: NO_DEBTOR_NUMBER_FORMAT, scope: "payer" });
        }
      });
    return () => {
      current = false;
    };
  }, []);
  return settings;
}

/** The format of the Träger's debtor numbers (#6675) for a family's fields. */
export function useDebtorNumberFormat(): DebtorNumberFormat {
  return useDebtorNumberSettings()?.format ?? NO_DEBTOR_NUMBER_FORMAT;
}
