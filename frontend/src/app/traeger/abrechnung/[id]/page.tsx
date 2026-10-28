"use client";

// Ein Abrechnungslauf (#4668): Vorschau aller Rechnungen eines
// Ferienzeitraums, Korrekturen mit Begründung und die Freigabe, die jede
// Rechnung mit ihrer Nummer ausstellt. Ausgestellte Rechnungen ändern sich
// nicht mehr. Jedes Öffnen steht im Protokoll des Laufs. Ein freigegebener
// Lauf hat die Übersicht für die Kämmerei (#4672).

import { useParams } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import {
  BillingCorrectionModal,
  type BillingCorrectionTarget,
} from "~/components/traeger/billing-correction-modal";
import { BillingRunOverviewCard } from "~/components/traeger/billing-run-overview-card";
import { BillingLettersCard } from "~/components/traeger/billing-letters-card";
import { BillingMailCard } from "~/components/traeger/billing-mail-card";
import {
  BILLING_RUN_STATUS_LABELS,
  BILLING_RUN_STATUS_TONES,
} from "~/components/traeger/billing-format";
import { formatEuro } from "~/components/traeger/contribution-format";
import { formatDateRange } from "~/components/traeger/holiday-period-format";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
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
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import {
  type BillingDocument,
  type BillingRunDetail,
  downloadBillingLetter,
  fetchBillingRun,
  releaseBillingRun,
} from "~/lib/traeger-billing-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerBillingRunPage" });

function runDescription(run: BillingRunDetail): string {
  if (run.status === "released") {
    return "Die Rechnungen sind ausgestellt und lassen sich nicht mehr ändern.";
  }
  return "Vorschau mit allen bestätigten Anmeldungen, die noch keine Rechnung haben. Neue Bestätigungen kommen bis zur Freigabe dazu.";
}

export default function TraegerBillingRunPage() {
  const params = useParams<{ id: string }>();
  const runId = params?.id ?? "";
  const [run, setRun] = useState<BillingRunDetail | null>(null);
  const [correcting, setCorrecting] = useState<BillingCorrectionTarget | null>(
    null,
  );
  const [confirmingRelease, setConfirmingRelease] = useState(false);
  const [releasing, setReleasing] = useState(false);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;
  const releaseErrors = useApiErrorDisplay();
  const letterErrors = useApiErrorDisplay();
  const { success } = useToast();

  const load = useCallback(async () => {
    try {
      setRun(await fetchBillingRun(runId));
      clearLoadError();
    } catch (err) {
      logger.error("traeger_billing_run_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "der Abrechnungslauf",
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
      setRun(await releaseBillingRun(run));
      setConfirmingRelease(false);
      success("Der Lauf ist freigegeben. Die Rechnungen sind ausgestellt.");
    } catch (err) {
      logger.error("traeger_billing_run_release_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setConfirmingRelease(false);
      void releaseErrors.show(err, { object: "der Lauf" });
      void load();
    } finally {
      setReleasing(false);
    }
  };

  const downloadLetter = async (document: BillingDocument) => {
    if (!run) return;
    try {
      const file = await downloadBillingLetter(run.id, document.id);
      downloadBlob(file.blob, file.filename);
    } catch (err) {
      logger.error("traeger_billing_letter_download_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void letterErrors.show(err, { object: "das Schreiben" });
    }
  };

  const preview = run?.status === "calculated";
  const canRelease = preview && run.numberRangeSet && run.previewCount > 0;

  return (
    <TenantPage
      title={run?.periodName ?? "Abrechnungslauf"}
      back
      backHref={traegerPath("/traeger/abrechnung")}
      backLabel="Zurück zur Abrechnung"
      stats={
        run ? (
          <StatusBadge
            label={BILLING_RUN_STATUS_LABELS[run.status]}
            tone={BILLING_RUN_STATUS_TONES[run.status]}
          />
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
            onClick={() => setConfirmingRelease(true)}
          >
            Lauf freigeben
          </Button>
        ) : undefined
      }
      error={pageLoad.error}
      loading={!pageLoad.error && !run}
      overlays={
        run ? (
          <>
            <BillingCorrectionModal
              runId={run.id}
              target={correcting}
              onClose={() => setCorrecting(null)}
              onCorrected={(corrected) => {
                setCorrecting(null);
                setRun(corrected);
              }}
            />
            <ConfirmationModal
              isOpen={confirmingRelease}
              title="Lauf freigeben?"
              confirmText="Freigeben"
              isConfirmLoading={releasing}
              loadingText="Wird freigegeben …"
              onClose={() => setConfirmingRelease(false)}
              onConfirm={() => void release()}
            >
              <p className="text-sm text-gray-700">
                {run.previewCount} Rechnung(en) über{" "}
                {formatEuro(run.previewCents)} bekommen ihre Nummer. Danach
                lassen sie sich nicht mehr ändern.
              </p>
            </ConfirmationModal>
          </>
        ) : null
      }
    >
      {run ? (
        <>
          <SectionCard
            title="Abrechnungslauf"
            description={runDescription(run)}
          >
            <DataGrid columns={2}>
              <DataField label="Betreuung">
                {formatDateRange(run.serviceFrom, run.serviceUntil)}
              </DataField>
              <DataField label="Rechnungen">{run.previewCount}</DataField>
              <DataField label="Summe">
                <span className="tabular-nums">
                  {formatEuro(run.previewCents)}
                </span>
              </DataField>
              <DataField label="Abrechnung">
                Im Voraus, nach den bestätigten Buchungen
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
            </DataGrid>
          </SectionCard>
          {preview && !run.numberRangeSet ? (
            <Alert
              type="warning"
              message="Bevor Sie freigeben: Legen Sie unter „Abrechnung“ das Muster der Rechnungsnummern fest."
            />
          ) : null}
          {preview && run.unbilled > 0 ? (
            <Alert
              type="info"
              message={`${run.unbilled} bestätigte Anmeldung(en) haben keinen festgehaltenen Beitrag und kommen nicht in diesen Lauf.`}
            />
          ) : null}
          {run.status === "released" ? (
            <BillingLettersCard
              runId={run.id}
              letters={run.letters}
              onCreated={(letters) => setRun({ ...run, letters })}
            />
          ) : null}
          {run.status === "released" && run.letters ? (
            <BillingMailCard runId={run.id} />
          ) : null}
          <BillingRunOverviewCard
            run={run}
            onCorrect={
              preview
                ? (document, child) =>
                    setCorrecting({
                      registrationId: document.registrationId,
                      child,
                    })
                : undefined
            }
            onDownloadLetter={
              run.letters
                ? (document) => void downloadLetter(document)
                : undefined
            }
          />
        </>
      ) : null}
    </TenantPage>
  );
}
