"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";

import {
  AllowanceCard,
  AllowanceExpiryNote,
} from "~/components/staff/allowance-card";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { useApiLoadError } from "~/contexts/ToastContext";
import { ABSENCES_REFRESH_EVENT } from "~/lib/absence-helpers";
import { createLogger } from "~/lib/logger";
import {
  timeTrackingService,
  type OwnAbsenceAllowance,
} from "~/lib/time-tracking-api";

const logger = createLogger({ component: "OwnAllowancesCard" });

// Die eigenen Kontingente der schuleigenen Abwesenheitsarten (#4971): dieselben
// Karten wie im Reiter „Abwesenheiten“ der Leitung, nur ohne „Anspruch ändern“.
// Ohne Art mit Kontingent bleibt der Block weg; ein Ladefehler steht im Block,
// damit er nicht als „0 Tage“ gelesen wird.
export function OwnAllowancesCard() {
  const [allowances, setAllowances] = useState<OwnAbsenceAllowance[] | null>(
    null,
  );
  const load = useApiLoadError();
  const showLoadError = load.show;
  const clearLoadError = load.clear;
  const loadRef = useRef<() => Promise<void>>(async () => undefined);

  const loadAllowances = useCallback(async () => {
    try {
      setAllowances(await timeTrackingService.getOwnAbsenceAllowances());
      clearLoadError();
    } catch (err) {
      logger.error("load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Übersicht der Kontingente",
        retry: () => void loadRef.current(),
      });
    }
  }, [showLoadError, clearLoadError]);

  useLayoutEffect(() => {
    loadRef.current = loadAllowances;
  });

  useEffect(() => {
    // loadAllowances shows its own failure in the card.
    void loadAllowances();
    const reload = () => void loadAllowances();
    window.addEventListener(ABSENCES_REFRESH_EVENT, reload);
    return () => window.removeEventListener(ABSENCES_REFRESH_EVENT, reload);
  }, [loadAllowances]);

  if (!load.error && (allowances === null || allowances.length === 0)) {
    return null;
  }

  return (
    <SectionCard
      title="Kontingente"
      description="Ihre Tage je Abwesenheitsart. Den Anspruch trägt Ihre Leitung ein."
      // Wie die Urlaubskarte darüber: ohne shrink-0 staucht die Flex-Spalte
      // der Seite die Karte auf die Kopfhöhe (#4363).
      className="shrink-0"
      bodyClassName="compact:mt-4 mt-5 max-sm:mt-4"
    >
      {load.error ? (
        <LoadErrorAlert error={load.error} />
      ) : (
        <div className="grid gap-3 lg:grid-cols-2">
          {(allowances ?? []).map((entry) => (
            <AllowanceCard
              key={entry.summary.absenceTypeId}
              title={entry.name}
              headingLevel={3}
              year={entry.summary.year}
              entitled={entry.summary.entitledDays}
              taken={entry.summary.takenDays}
              reserved={entry.summary.reservedDays}
              remaining={entry.summary.remainingDays}
              footer={<AllowanceExpiryNote summary={entry.summary} />}
            />
          ))}
        </div>
      )}
    </SectionCard>
  );
}
