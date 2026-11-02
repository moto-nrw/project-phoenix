"use client";

// Beiträge des Trägers (#4664): die Regeln, aus denen die Elternabrechnung
// den Ferienbeitrag rechnet. Der Grundpreis ist der Preis des Zeitmodells im
// Ferienzeitraum; hier stehen die Geschwisterregel und die Befreiung bei
// Leistungsbezug. Nur die Rolle mit dem Recht „Abrechnung“ sieht die Seite.

import { useCallback, useEffect, useState } from "react";
import { ContributionRulesCard } from "~/components/traeger/contribution-rules-card";
import { TenantPage } from "~/components/ui/tenant-page";
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
    <TenantPage
      title="Beiträge"
      info="So entsteht der Ferienbeitrag: Jede gebuchte Woche kostet den Preis ihres Zeitmodells. Den Preis legen Sie im Ferienzeitraum fest. Danach gelten die Regeln auf dieser Seite, in dieser Reihenfolge. Beim Bestätigen einer Anmeldung wird der Beitrag festgehalten. Spätere Änderungen gelten nur für neue Bestätigungen."
      error={pageLoad.error}
      loading={!pageLoad.error && !rules}
    >
      {rules ? (
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
      ) : null}
    </TenantPage>
  );
}
