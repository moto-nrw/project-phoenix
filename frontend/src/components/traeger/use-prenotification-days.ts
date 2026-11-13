"use client";

// Die Vorabankündigung des Gläubigers in Tagen (#4695): Laufseite,
// Freigabefrage und Fälligkeiten nennen damit den ersten erlaubten Tag einer
// Abbuchung.

import { useEffect, useState } from "react";
import { createLogger } from "~/lib/logger";
import { fetchSepaCreditor } from "~/lib/traeger-sepa-api";

const logger = createLogger({ component: "usePrenotificationDays" });

/**
 * The creditor's pre-notification days; null while unknown, while `enabled`
 * is false, or when the read fails: the screens then name the rule without
 * its days.
 */
export function usePrenotificationDays(enabled = true): number | null {
  const [days, setDays] = useState<number | null>(null);
  useEffect(() => {
    if (!enabled) return;
    let current = true;
    fetchSepaCreditor()
      .then((creditor) => {
        if (current) setDays(creditor.prenotificationDays);
      })
      .catch((err: unknown) => {
        // Silent on purpose: the hint names the rule without its days, and
        // nothing on the page depends on the number.
        logger.warn("traeger_prenotification_days_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
    return () => {
      current = false;
    };
  }, [enabled]);
  return days;
}
