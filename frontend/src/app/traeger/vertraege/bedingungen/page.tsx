"use client";

// Vertragsbedingungen je Angebot (#4678): Laufzeit, Kündigung,
// Sonderkündigung, Satzung und Vertragspartner. Jeder neue Vertrag eines
// Angebots übernimmt sie; ohne sie entsteht für das Angebot kein Entwurf.

import { useCallback, useEffect, useState } from "react";
import { CareContractConditions } from "~/components/traeger/care-contract-conditions";
import { careContractTabs } from "~/components/traeger/care-contract-tabs";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { useApiLoadError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  type OfferingTemplate,
  fetchContractTemplates,
} from "~/lib/traeger-care-contracts-api";

const logger = createLogger({ component: "TraegerVertragsbedingungenPage" });

export default function TraegerVertragsbedingungenPage() {
  const [offerings, setOfferings] = useState<OfferingTemplate[] | null>(null);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;

  const load = useCallback(async () => {
    try {
      setOfferings(await fetchContractTemplates());
      clearLoadError();
    } catch (err) {
      logger.error("traeger_contract_conditions_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Übersicht der Vertragsbedingungen",
        retry: () => void load(),
      });
    }
  }, [clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const active = offerings?.filter((offering) => offering.active) ?? [];
  const missing = active.filter((offering) => !offering.template).length;

  return (
    <TenantPage
      title="Verträge"
      info="Vertragsbedingungen legen fest, was in jedem Vertrag eines Angebots steht: Laufzeit, Kündigung, Sonderkündigung und Satzung. Den Brieftext ändern Sie unter „Einstellungen“ → „Briefkopf und Vorlagen“."
      tabs={careContractTabs("bedingungen", missing)}
      stats={
        offerings && active.length > 0 ? (
          <TenantPageStats
            items={[
              {
                value: active.length,
                label: active.length === 1 ? "Angebot" : "Angebote",
              },
              { value: active.length - missing, label: "festgelegt" },
              { value: missing, label: "fehlen" },
            ]}
          />
        ) : null
      }
      statsLoading={!offerings && !pageLoad.error}
      error={pageLoad.error}
      loading={!pageLoad.error && !offerings}
    >
      {offerings ? (
        <CareContractConditions
          offerings={offerings}
          onSaved={(saved) =>
            setOfferings((current) =>
              (current ?? []).map((offering) =>
                offering.schoolId === saved.schoolId &&
                offering.offeringId === saved.offeringId
                  ? saved
                  : offering,
              ),
            )
          }
        />
      ) : null}
    </TenantPage>
  );
}
