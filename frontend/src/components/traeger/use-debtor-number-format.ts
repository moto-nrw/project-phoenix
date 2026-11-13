"use client";

import { useEffect, useState } from "react";
import {
  type DebtorNumberFormat,
  NO_DEBTOR_NUMBER_FORMAT,
} from "~/lib/debtor-number-format";
import { createLogger } from "~/lib/logger";
import { fetchDebtorNumberFormat } from "~/lib/traeger-billing-families-api";

const logger = createLogger({ component: "useDebtorNumberFormat" });

/**
 * The format of the Träger's debtor numbers (#6675) for the fields of a
 * family. It only shapes the field: without it the backend still checks
 * every number, so a failed read stays in the log and the field keeps its
 * plain hint.
 */
export function useDebtorNumberFormat(): DebtorNumberFormat {
  const [format, setFormat] = useState<DebtorNumberFormat>(
    NO_DEBTOR_NUMBER_FORMAT,
  );
  useEffect(() => {
    let current = true;
    fetchDebtorNumberFormat()
      .then((loaded) => {
        if (current) setFormat(loaded);
      })
      .catch((err: unknown) => {
        logger.warn("traeger_debtor_number_format_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
    return () => {
      current = false;
    };
  }, []);
  return format;
}
