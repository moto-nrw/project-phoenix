"use client";

// Ein Monatslauf (#4684): die Vorschau aller Rechnungen eines Monats nach
// dem gültigen Vertrag und der Preisliste, und die Freigabe, die jede
// Rechnung mit ihrer Nummer ausstellt. Danach ändern sich die Rechnungen
// nicht mehr. Jedes Öffnen steht im Protokoll des Laufs.

import { useParams } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import {
  CARE_RUN_STATUS_LABELS,
  CARE_RUN_STATUS_TONES,
  MONTH_RULE_LABELS,
  formatMonth,
  invoiceCountLabel,
} from "~/components/traeger/care-billing-format";
import {
  CareRunInvoices,
  CareDuplicatesCard,
  CareUnpricedCard,
} from "~/components/traeger/care-run-invoices";
import { formatEuro } from "~/components/traeger/contribution-format";
import { HolidayHeadFacts } from "~/components/traeger/holiday-period-head";
import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { ConfirmationModal } from "~/components/ui/modal";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage } from "~/components/ui/tenant-page";
import {
  useApiErrorDisplay,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  type CareRunDetail,
  fetchCareRun,
  releaseCareRun,
} from "~/lib/traeger-care-billing-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerCareRunPage" });

/**
 * "1 Kind mit doppeltem Vertrag wird nicht abgerechnet.", after the release
 * "… ist nicht abgerechnet."
 */
function notBilledLabel(children: number, released = false): string {
  if (children === 1) {
    return `1 Kind mit doppeltem Vertrag ${released ? "ist" : "wird"} nicht abgerechnet.`;
  }
  return `${children} Kinder mit doppeltem Vertrag ${released ? "sind" : "werden"} nicht abgerechnet.`;
}

export default function TraegerCareRunPage() {
  const params = useParams<{ id: string }>();
  const runId = params?.id ?? "";
  const [run, setRun] = useState<CareRunDetail | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [releasing, setReleasing] = useState(false);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;
  const releaseErrors = useApiErrorDisplay();
  const { success } = useToast();

  const load = useCallback(async () => {
    try {
      setRun(await fetchCareRun(runId));
      clearLoadError();
    } catch (err) {
      logger.error("traeger_care_run_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "der Monatslauf",
        retry: () => void load(),
      });
    }
  }, [runId, clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const release = async () => {
    if (!run) return;
    setReleasing(true);
    try {
      const released = await releaseCareRun(run);
      setRun(released);
      setConfirming(false);
      success(
        released.duplicateChildren > 0
          ? `Der Lauf ist freigegeben. ${notBilledLabel(released.duplicateChildren, true)}`
          : "Der Lauf ist freigegeben. Die Rechnungen sind ausgestellt.",
      );
    } catch (err) {
      logger.error("traeger_care_run_release_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setConfirming(false);
      void releaseErrors.show(err, { object: "der Monatslauf" });
      void load();
    } finally {
      setReleasing(false);
    }
  };

  const preview = run?.status === "calculated";
  const canRelease = preview && run.numberRangeSet && run.previewCount > 0;

  return (
    <TenantPage
      title={
        run ? `Monatsbeitrag ${formatMonth(run.billingMonth)}` : "Monatslauf"
      }
      back
      backHref={traegerPath("/traeger/abrechnung/monatsbeitraege")}
      backLabel="Zurück zu den Monatsbeiträgen"
      stats={
        run ? (
          <HolidayHeadFacts>
            <StatusBadge
              label={CARE_RUN_STATUS_LABELS[run.status]}
              tone={CARE_RUN_STATUS_TONES[run.status]}
            />
            <span className="tabular-nums">
              {invoiceCountLabel(run.previewCount)} ·{" "}
              {formatEuro(run.previewCents)}
            </span>
          </HolidayHeadFacts>
        ) : null
      }
      statsLoading={!run && !pageLoad.error}
      actions={
        preview ? (
          <Button
            type="button"
            variant="primary"
            size="md"
            disabled={!canRelease}
            onClick={() => setConfirming(true)}
          >
            Lauf freigeben
          </Button>
        ) : undefined
      }
      error={pageLoad.error}
      loading={!pageLoad.error && !run}
      overlays={
        run ? (
          <ConfirmationModal
            isOpen={confirming}
            title="Monatslauf freigeben?"
            confirmText="Freigeben"
            isConfirmLoading={releasing}
            loadingText="Wird freigegeben …"
            onClose={() => setConfirming(false)}
            onConfirm={() => void release()}
          >
            <p className="text-sm text-gray-700">
              {invoiceCountLabel(run.previewCount)} über{" "}
              {formatEuro(run.previewCents)} für {formatMonth(run.billingMonth)}{" "}
              bekommen ihre Nummer. Danach lassen sie sich nicht mehr ändern.
            </p>
            {run.duplicateChildren > 0 ? (
              <p className="mt-3 text-sm text-gray-700">
                {notBilledLabel(run.duplicateChildren)} Das holen Sie mit einem
                neuen Lauf nach, wenn ein Vertrag beendet ist.
              </p>
            ) : null}
          </ConfirmationModal>
        ) : null
      }
    >
      {run ? (
        <>
          <SectionCard
            title="Monatslauf"
            description={
              preview
                ? "Vorschau nach den Verträgen, die in diesem Monat gelten. Neue Verträge und Preise kommen bis zur Freigabe dazu."
                : "Die Rechnungen sind ausgestellt und lassen sich nicht mehr ändern."
            }
          >
            <DataGrid columns={2}>
              <DataField label="Monat">
                {formatMonth(run.billingMonth)}
              </DataField>
              <DataField label="Rechnungen">
                <span className="tabular-nums">
                  {invoiceCountLabel(run.previewCount)} ·{" "}
                  {formatEuro(run.previewCents)}
                </span>
              </DataField>
              <DataField label="Abgerechnet wird">
                Nach Vertrag, nicht nach Anwesenheit
              </DataField>
              <DataField label="Vertrag ändert sich im Monat">
                {run.monthRule ? MONTH_RULE_LABELS[run.monthRule] : "–"}
              </DataField>
              <DataField label="Rechnungsnummern">
                {run.issuedOn ? (
                  `Ausgestellt am ${formatDate(run.issuedOn)}`
                ) : run.numberRangeSet ? (
                  <span className="font-mono">{run.numberPattern}</span>
                ) : (
                  "Noch kein Muster"
                )}
              </DataField>
              {run.billed > 0 ? (
                <DataField label="Schon abgerechnet">
                  {run.billed === 1
                    ? "1 Kind aus einem früheren Lauf"
                    : `${run.billed} Kinder aus früheren Läufen`}
                </DataField>
              ) : null}
            </DataGrid>
          </SectionCard>
          {preview && !run.numberRangeSet ? (
            <Alert
              type="warning"
              message="Bevor Sie freigeben: Legen Sie das Muster der Rechnungsnummern für Monatsbeiträge fest."
              action={
                <ButtonLink
                  href={traegerPath("/traeger/einstellungen/abrechnung")}
                  variant="surface"
                  size="compact"
                >
                  Abrechnungsvorgaben öffnen
                </ButtonLink>
              }
            />
          ) : null}
          <CareDuplicatesCard duplicates={run.duplicates} released={!preview} />
          {preview ? <CareUnpricedCard unpriced={run.unpriced} /> : null}
          <CareRunInvoices
            invoices={run.invoices}
            month={run.billingMonth}
            released={!preview}
          />
        </>
      ) : null}
    </TenantPage>
  );
}
