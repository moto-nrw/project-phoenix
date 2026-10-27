"use client";

// Beiträge des Trägers (#4664): die Regeln, aus denen die Elternabrechnung
// den Ferienbeitrag rechnet. Der Grundpreis ist der Preis des Zeitmodells im
// Ferienzeitraum; hier stehen die Geschwisterregel und die Befreiung bei
// Leistungsbezug. Nur die Rolle mit dem Recht „Abrechnung“ sieht die Seite.

import { useCallback, useEffect, useState } from "react";
import { ContributionRulesCard } from "~/components/traeger/contribution-rules-card";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { Loading } from "~/components/ui/loading";
import { SectionCard } from "~/components/ui/section-card";
import { useApiLoadError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  type ContributionRule,
  fetchContributionRules,
} from "~/lib/traeger-contributions-api";

const logger = createLogger({ component: "TraegerBeitraegePage" });

export default function TraegerBeitraegePage() {
  const [rules, setRules] = useState<ContributionRule[] | null>(null);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;

  const load = useCallback(async () => {
    try {
      setRules(await fetchContributionRules());
      clearLoadError();
    } catch (err) {
      logger.error("traeger_contribution_rules_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Beitragsregeln",
        retry: () => void load(),
      });
    }
  }, [clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <div className="w-full space-y-6">
      <h1 className="text-xl font-semibold text-gray-900 sm:text-2xl">
        Beiträge
      </h1>
      <SectionCard title="So entsteht der Ferienbeitrag">
        <p className="text-sm text-gray-700">
          Jede gebuchte Woche kostet den Preis ihres Zeitmodells. Den Preis
          legen Sie im Ferienzeitraum fest. Danach gelten die Regeln unten, in
          dieser Reihenfolge. Beim Bestätigen einer Anmeldung wird der Beitrag
          festgehalten. Spätere Änderungen gelten nur für neue Bestätigungen.
        </p>
      </SectionCard>
      {pageLoad.error ? (
        <LoadErrorAlert error={pageLoad.error} />
      ) : rules ? (
        <>
          <ContributionRulesCard
            kind="sibling"
            rules={rules.filter((rule) => rule.kind === "sibling")}
            onChanged={load}
          />
          <ContributionRulesCard
            kind="benefits_exemption"
            rules={rules.filter((rule) => rule.kind === "benefits_exemption")}
            onChanged={load}
          />
        </>
      ) : (
        <Loading fullPage={false} />
      )}
    </div>
  );
}
