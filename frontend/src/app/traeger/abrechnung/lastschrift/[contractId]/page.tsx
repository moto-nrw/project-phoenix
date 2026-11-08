"use client";

// Das SEPA-Lastschriftmandat der Familie eines Vertrags (#4695): erfassen,
// widerrufen, Verlauf. Eine eigene Seite, damit Vertrags- und Empfängerseite
// nur den Stand zeigen und die Formulare nicht laden. Die Familie ist die
// E-Mail-Adresse der Vertragspartei; ihr Mandat gilt auch für Geschwister.

import { useParams } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { SepaMandatesCard } from "~/components/traeger/sepa-mandates-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { useApiLoadError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { type SepaMandate, fetchSepaMandates } from "~/lib/traeger-sepa-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerSepaMandatePage" });

export default function TraegerSepaMandatePage() {
  const params = useParams<{ contractId: string }>();
  const contractId = params?.contractId ?? "";
  const [mandates, setMandates] = useState<SepaMandate[] | null>(null);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;

  const load = useCallback(async () => {
    try {
      setMandates(await fetchSepaMandates(contractId));
      clearLoadError();
    } catch (err) {
      logger.error("traeger_sepa_mandate_page_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "das Mandat",
        retry: () => void load(),
      });
    }
  }, [contractId, clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const party = mandates?.[0];
  const family = party
    ? `${party.partyFirstName} ${party.partyLastName}`.trim()
    : "";

  return (
    <TenantPage
      title={
        family ? `Lastschrift: Familie ${party?.partyLastName}` : "Lastschrift"
      }
      back
      backHref={traegerPath("/traeger/abrechnung/lastschrift")}
      backLabel="Zurück zur Lastschrift"
      error={pageLoad.error}
      loading={!pageLoad.error && mandates === null}
    >
      {mandates !== null ? (
        <SepaMandatesCard
          contractId={contractId}
          holder={family}
          mandates={mandates}
          onChanged={() => void load()}
        />
      ) : null}
    </TenantPage>
  );
}
