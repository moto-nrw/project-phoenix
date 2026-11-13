"use client";

// Empfänger und Zahlung einer Familie (#4689): an wen ihre Rechnungen gehen
// und wie sie zahlt, mit ihrem SEPA-Mandat (#4695). Geöffnet aus dem Monatslauf für die Familie eines
// Vertrags; eine eigene Seite, damit die Seite des Laufs das Formular nicht
// lädt. Die gespeicherte Abweichung wird bei jedem Öffnen frisch gelesen.

import { SepaMandateStatusCard } from "~/components/traeger/sepa-mandate-status-card";
import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import {
  findFamilyBilling,
  runDueDatesText,
} from "~/components/traeger/billing-payment-format";
import { formatMonth } from "~/components/traeger/care-billing-format";
import { FamilyBillingForm } from "~/components/traeger/family-billing-form";
import { useDebtorNumberFormat } from "~/components/traeger/use-debtor-number-format";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { useApiLoadError } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { createLogger } from "~/lib/logger";
import {
  type FamilyBilling,
  fetchFamilyBillings,
} from "~/lib/traeger-billing-terms-api";
import {
  type CareInvoice,
  type CareRunDetail,
  fetchCareRun,
} from "~/lib/traeger-care-billing-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerFamilyBillingPage" });

interface Loaded {
  readonly run: CareRunDetail;
  readonly invoice: CareInvoice;
  readonly existing: FamilyBilling | null;
}

export default function TraegerFamilyBillingPage() {
  const params = useParams<{ id: string; contractId: string }>();
  const router = useRouter();
  const runId = params?.id ?? "";
  const contractId = params?.contractId ?? "";
  const [loaded, setLoaded] = useState<Loaded | null>(null);
  const pageLoad = useApiLoadError();
  const debtorNumberFormat = useDebtorNumberFormat();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;
  const runHref = traegerPath(`/traeger/abrechnung/monatsbeitraege/${runId}`);

  const load = useCallback(async () => {
    try {
      const [run, families] = await Promise.all([
        fetchCareRun(runId),
        fetchFamilyBillings(),
      ]);
      const invoice = run.invoices.find(
        (entry) => entry.contractId === contractId,
      );
      if (!invoice) {
        throw new ApiError("Die Rechnung gehört nicht zu diesem Lauf.", 404);
      }
      setLoaded({
        run,
        invoice,
        existing: findFamilyBilling(families, {
          contractId,
          familyEmail: invoice.familyEmail,
          recipientEmail: invoice.recipient.email,
          recipientKind: invoice.recipient.kind,
        }),
      });
      clearLoadError();
    } catch (err) {
      logger.error("traeger_family_billing_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Angabe zu Empfänger und Zahlung",
        retry: () => void load(),
      });
    }
  }, [runId, contractId, showLoadError, clearLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const childName = loaded
    ? `${loaded.invoice.childFirstName} ${loaded.invoice.childLastName}`.trim()
    : "";

  return (
    <TenantPage
      title={
        childName
          ? `Empfänger und Zahlung: Familie von ${childName}`
          : "Empfänger und Zahlung"
      }
      back
      backHref={runHref}
      backLabel="Zurück zum Monatslauf"
      stats={
        loaded ? (
          <span>Monatsbeitrag {formatMonth(loaded.run.billingMonth)}</span>
        ) : null
      }
      statsLoading={!loaded && !pageLoad.error}
      error={pageLoad.error}
      loading={!pageLoad.error && !loaded}
    >
      {loaded ? (
        <SectionCard title="Empfänger und Zahlung">
          <FamilyBillingForm
            target={{
              contractId,
              applicantName:
                loaded.invoice.recipient.kind === "payer"
                  ? ""
                  : `${loaded.invoice.recipient.firstName} ${loaded.invoice.recipient.lastName}`.trim(),
              childName,
              issued: loaded.invoice.number !== "",
            }}
            format={debtorNumberFormat}
            existing={loaded.existing}
            runDueText={runDueDatesText(loaded.run.dueDates ?? [])}
            onCancel={() => router.push(runHref)}
            onSaved={() => router.push(runHref)}
          />
        </SectionCard>
      ) : null}
      {loaded ? <SepaMandateStatusCard contractId={contractId} /> : null}
    </TenantPage>
  );
}
