"use client";

// Ermäßigungen des Trägers (#4664): die Regeln, aus denen die Elternabrechnung
// den Ferienbeitrag rechnet. Der Grundpreis ist der Preis des Zeitmodells im
// Ferienzeitraum; hier stehen die Geschwisterregel und die Befreiung bei
// Leistungsbezug, dazu die Kostenträger, die den Beitrag einer Familie
// übernehmen (#5627). Nur die Rolle mit dem Recht „Abrechnung“ sieht die
// Seite.

import { useCallback, useEffect, useState } from "react";
import { ContributionRulesCard } from "~/components/traeger/contribution-rules-card";
import { CostBearersCard } from "~/components/traeger/cost-bearers-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { useApiLoadError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  type ContributionRule,
  type CostBearer,
  fetchContributionRules,
  fetchCostBearers,
} from "~/lib/traeger-contributions-api";

const logger = createLogger({ component: "TraegerBeitraegePage" });

export default function TraegerBeitraegePage() {
  const [rules, setRules] = useState<ContributionRule[] | null>(null);
  const [bearers, setBearers] = useState<CostBearer[] | null>(null);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;

  const load = useCallback(async () => {
    try {
      const [loadedRules, loadedBearers] = await Promise.all([
        fetchContributionRules(),
        fetchCostBearers(),
      ]);
      setRules(loadedRules);
      setBearers(loadedBearers);
      clearLoadError();
    } catch (err) {
      logger.error("traeger_contribution_rules_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Ermäßigungen",
        retry: () => void load(),
      });
    }
  }, [clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <TenantPage
      title="Ermäßigungen"
      info="So entsteht der Ferienbeitrag: Jede gebuchte Woche kostet den Preis ihres Zeitmodells. Den Preis legen Sie im Ferienzeitraum fest. Danach gelten die Regeln auf dieser Seite, in dieser Reihenfolge. Beim Bestätigen einer Anmeldung wird der Beitrag festgehalten. Spätere Änderungen gelten nur für neue Bestätigungen."
      error={pageLoad.error}
      loading={!pageLoad.error && !(rules && bearers)}
    >
      {rules && bearers ? (
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
          <CostBearersCard bearers={bearers} onChanged={load} />
        </>
      ) : null}
    </TenantPage>
  );
}
