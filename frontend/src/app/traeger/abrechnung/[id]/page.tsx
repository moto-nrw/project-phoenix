"use client";

// Ein Abrechnungslauf (#4668): Vorschau aller Rechnungen eines
// Ferienzeitraums, Korrekturen mit Begründung und die Freigabe, die jede
// Rechnung mit ihrer Nummer ausstellt. Ausgestellte Rechnungen ändern sich
// nicht mehr. Jedes Öffnen steht im Protokoll des Laufs. Ein freigegebener
// Lauf hat die Übersicht für die Kämmerei (#4672). Eine ausgestellte Rechnung
// lässt sich stornieren und gutschreiben (#4690); eine stornierte Rechnung
// stellt der nächste Lauf des Zeitraums neu aus und schlägt ihre Korrekturen
// wieder vor (#5663). Die Statuszeile führt zurück zum Ferienzeitraum des
// Laufs, für alle, die die Ferienbetreuung öffnen dürfen (#5688). Jede
// Rechnung zeigt Zahlungsweg und Fälligkeit; die Fälligkeiten des Laufs
// lassen sich bis zur Freigabe ändern (#4689). Ein offener Lauf, der nichts
// abzurechnen hat, lässt sich verwerfen; er bleibt als „Verworfen“ in der
// Liste, und der Ferienzeitraum ist frei für einen neuen Lauf (#6628).

import { ArchiveX, Inbox } from "lucide-react";
import { useSession } from "next-auth/react";
import dynamic from "next/dynamic";
import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import {
  BillingCorrectionModal,
  type BillingCorrectionTarget,
} from "~/components/traeger/billing-correction-modal";
import { BillingCarriedCorrectionsCard } from "~/components/traeger/billing-carried-corrections-card";
import { BillingCostTakeoverCard } from "~/components/traeger/billing-cost-takeover-card";
import { BillingRunOverviewCard } from "~/components/traeger/billing-run-overview-card";
import { BillingLettersCard } from "~/components/traeger/billing-letters-card";
import { BillingMailCard } from "~/components/traeger/billing-mail-card";
import {
  BILLING_RUN_STATUS_LABELS,
  BILLING_RUN_STATUS_TONES,
  isEmptyBillingRun,
} from "~/components/traeger/billing-format";
import { runDueDatesText } from "~/components/traeger/billing-payment-format";
import { HolidayDebitHintsCard } from "~/components/traeger/care-run-review-lazy";
import { formatEuro } from "~/components/traeger/contribution-format";
import { formatDateRange } from "~/components/traeger/holiday-period-format";
import { HolidayHeadFacts } from "~/components/traeger/holiday-period-head";
import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { EmptyState } from "~/components/ui/empty-state";
import { ConfirmationModal } from "~/components/ui/modal";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import NavigationLink from "~/components/ui/navigation-link";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { runDocumentKindLabel } from "~/components/traeger/run-document-kind";
import { TenantPage } from "~/components/ui/tenant-page";
import {
  useApiErrorDisplay,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { hasPermission } from "~/lib/auth-utils";
import { holidayWeeksConcept } from "~/lib/concepts/holidayWeeks";
import { formatDate } from "~/lib/date-helpers";
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import {
  type BillingDocument,
  type BillingRunDetail,
  discardBillingRun,
  downloadBillingLetter,
  fetchBillingRun,
  fetchBillingRuns,
  releaseBillingRun,
  startBillingRun,
} from "~/lib/traeger-billing-api";
import { traegerPath } from "~/lib/traeger-url";

// The direct debit file of a released run (#4697) loads with the run.
const SepaDebitCard = dynamic(
  () =>
    import("~/components/traeger/sepa-debit-card").then(
      (module) => module.SepaDebitCard,
    ),
  { ssr: false },
);
const logger = createLogger({ component: "TraegerBillingRunPage" });

/** Das Recht, das die Ferienzeiträume öffnet (#4659). */
const TRAEGER_ENROLLMENT_RIGHT = "traeger_enrollment:manage";

function runDescription(run: BillingRunDetail): string {
  if (run.status === "released") {
    return "Die Rechnungen sind ausgestellt und lassen sich nicht mehr ändern.";
  }
  return "Vorschau mit allen bestätigten Anmeldungen ohne gültige Rechnung. Eine stornierte Rechnung wird hier neu ausgestellt, mit ihren Korrekturen als Vorschlag. Neue Bestätigungen kommen bis zur Freigabe dazu.";
}

/** „3 Rechnungen mit Betrag haben keine Debitorennummer.“ */
function invoicesWithoutDebtorNumber(count: number): string {
  return count === 1
    ? "1 Rechnung mit Betrag hat keine Debitorennummer."
    : `${count} Rechnungen mit Betrag haben keine Debitorennummer.`;
}

/**
 * The warning before the release when the payment text names the debtor
 * number and invoices with an amount have none (#5622): the release locks
 * the number, and their letters are refused until the placeholder goes.
 * The field to enter it shows at the registration only while the Träger
 * asks for the number (#5621); then `missingDebtorNumbers` counts these
 * invoices too, otherwise it is 0.
 */
function paymentTextWarning(run: BillingRunDetail): string {
  const where =
    run.missingDebtorNumbers > 0
      ? "Tragen Sie die Nummern vor dem Freigeben bei den Anmeldungen ein."
      : "Tragen Sie die Nummern vor dem Freigeben bei den Anmeldungen ein. Das Feld dafür erscheint, wenn in den Einstellungen unter „Allgemein“ im Bereich „Ferienbetreuung“ „Debitorennummer beim Bestätigen abfragen“ an ist.";
  return `${invoicesWithoutDebtorNumber(run.paymentTextMissingDebtorNumbers)} Ihr Zahlungstext nennt aber die Debitorennummer. Nach dem Freigeben können Sie sie nicht mehr eintragen. Schreiben gehen dann nur ohne den Platzhalter im Zahlungstext. ${where}`;
}

export default function TraegerBillingRunPage() {
  const params = useParams<{ id: string }>();
  const runId = params?.id ?? "";
  const [run, setRun] = useState<BillingRunDetail | null>(null);
  const [correcting, setCorrecting] = useState<BillingCorrectionTarget | null>(
    null,
  );
  const [confirmingRelease, setConfirmingRelease] = useState(false);
  const router = useRouter();
  const reissueErrors = useApiErrorDisplay();
  const [releasing, setReleasing] = useState(false);
  const [confirmingDiscard, setConfirmingDiscard] = useState(false);
  const [discarding, setDiscarding] = useState(false);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;
  const releaseErrors = useApiErrorDisplay();
  const letterErrors = useApiErrorDisplay();
  const { success } = useToast();
  const { data: session } = useSession();
  const canOpenPeriod = hasPermission(
    session ?? null,
    TRAEGER_ENROLLMENT_RIGHT,
  );

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

  const discard = async () => {
    if (!run) return;
    setDiscarding(true);
    try {
      setRun(await discardBillingRun(run.id));
      setConfirmingDiscard(false);
      success(
        `Der Lauf ist verworfen. Für ${run.periodName} können Sie einen neuen Lauf starten.`,
      );
    } catch (err) {
      logger.error("traeger_billing_run_discard_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setConfirmingDiscard(false);
      void releaseErrors.show(err, { object: "der Lauf" });
      void load();
    } finally {
      setDiscarding(false);
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

  // A cancelled invoice is issued again by the run of its period that is
  // still a preview; without one, a new run starts (#4690).
  const reissue = async () => {
    if (!run) return;
    try {
      const open = (await fetchBillingRuns()).find(
        (other) =>
          other.status === "calculated" &&
          other.holidayPeriodId === run.holidayPeriodId,
      );
      const target = open ?? (await startBillingRun(run.holidayPeriodId));
      router.push(traegerPath(`/traeger/abrechnung/${target.id}`));
    } catch (err) {
      logger.error("traeger_billing_reissue_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void reissueErrors.show(err, { object: "der neue Lauf" });
    }
  };

  const preview = run?.status === "calculated";
  const discarded = run?.status === "discarded";
  const empty = run ? isEmptyBillingRun(run) : false;
  const corrections =
    run?.documents.filter((document) => document.kind !== "invoice") ?? [];
  const canRelease =
    preview &&
    run.numberRangeSet &&
    run.previewCount > 0 &&
    !run.legalRemedyMissing;
  const paymentTextGaps = preview ? run.paymentTextMissingDebtorNumbers : 0;

  return (
    <TenantPage
      title={run?.periodName ?? "Abrechnungslauf"}
      back
      backHref={traegerPath("/traeger/abrechnung")}
      backLabel="Zurück zur Abrechnung"
      stats={
        run ? (
          <HolidayHeadFacts>
            <StatusBadge
              label={BILLING_RUN_STATUS_LABELS[run.status]}
              tone={BILLING_RUN_STATUS_TONES[run.status]}
            />
            {canOpenPeriod ? (
              <NavigationLink
                href={traegerPath(
                  `/traeger/ferien/${encodeURIComponent(run.holidayPeriodId)}`,
                )}
                className="inline-flex items-center gap-1.5 font-medium text-gray-900 underline-offset-2 hover:underline"
              >
                <MotoConceptIcon concept={holidayWeeksConcept} size={16} />
                Zum Ferienzeitraum
              </NavigationLink>
            ) : null}
            {discarded ? (
              <span>
                Verworfen am{" "}
                {run.discardedAt ? formatDate(run.discardedAt) : "–"}
              </span>
            ) : null}
          </HolidayHeadFacts>
        ) : null
      }
      statsLoading={!run && !pageLoad.error}
      actions={
        empty ? (
          <Button
            type="button"
            variant="outline_danger"
            size="md"
            onClick={() => setConfirmingDiscard(true)}
          >
            Leeren Lauf verwerfen
          </Button>
        ) : discarded ? (
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={() => void reissue()}
          >
            Neuen Lauf starten
          </Button>
        ) : preview ? (
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
        run && empty ? (
          <ConfirmationModal
            isOpen={confirmingDiscard}
            title="Leeren Lauf verwerfen?"
            confirmText="Verwerfen"
            confirmVariant="danger"
            isConfirmLoading={discarding}
            loadingText="Wird verworfen …"
            onClose={() => setConfirmingDiscard(false)}
            onConfirm={() => void discard()}
          >
            <div className="space-y-3 text-sm text-gray-700">
              <p>
                Der Lauf für {run.periodName} stellt nichts aus. Er bleibt als
                „Verworfen“ in der Liste.
              </p>
              <p>
                Danach ist der Ferienzeitraum frei. Sie können sofort einen
                neuen Lauf starten.
              </p>
            </div>
          </ConfirmationModal>
        ) : run ? (
          <>
            <BillingCorrectionModal
              runId={run.id}
              target={correcting}
              onClose={() => setCorrecting(null)}
              onCorrected={(corrected) => {
                setCorrecting(null);
                setRun(corrected);
              }}
              onIssued={() => {
                setCorrecting(null);
                void load();
              }}
            />
            <ConfirmationModal
              isOpen={confirmingRelease}
              title="Lauf freigeben?"
              confirmText={
                paymentTextGaps > 0 ? "Trotzdem freigeben" : "Freigeben"
              }
              confirmVariant={paymentTextGaps > 0 ? "warning" : "primary"}
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
              {paymentTextGaps > 0 ? (
                <Alert
                  type="warning"
                  className="mt-3"
                  message={`${invoicesWithoutDebtorNumber(paymentTextGaps)} Ihr Zahlungstext nennt aber die Debitorennummer. Nach dem Freigeben können Sie sie nicht mehr eintragen. Schreiben gehen dann nur ohne den Platzhalter im Zahlungstext.`}
                />
              ) : null}
            </ConfirmationModal>
          </>
        ) : null
      }
    >
      {run && (discarded || empty) ? (
        <SectionCard title="Abrechnungslauf">
          <EmptyState
            tone="neutral"
            icon={
              discarded ? (
                <ArchiveX className="h-7 w-7" aria-hidden="true" />
              ) : (
                <Inbox className="h-7 w-7" aria-hidden="true" />
              )
            }
            title={
              discarded
                ? "Dieser Lauf ist verworfen"
                : "Dieser Lauf hat nichts abzurechnen"
            }
            description={
              discarded
                ? `Er hatte nichts abzurechnen und hat nichts ausgestellt. Für ${run.periodName} können Sie einen neuen Lauf starten.`
                : `Für ${run.periodName} gibt es keine bestätigte Anmeldung ohne Rechnung. Sie können den Lauf verwerfen. Danach ist der Ferienzeitraum frei für einen neuen Lauf.`
            }
            className="py-6"
          />
          <div className="mt-2 border-t border-gray-100 pt-5">
            <DataGrid columns={2}>
              <DataField label="Ferienzeitraum">{run.periodName}</DataField>
              <DataField label="Betreuung">
                {formatDateRange(run.serviceFrom, run.serviceUntil)}
              </DataField>
              <DataField label="Gestartet am">
                {formatDate(run.createdAt)}
              </DataField>
              {discarded ? (
                <DataField label="Verworfen am">
                  {run.discardedAt ? formatDate(run.discardedAt) : "–"}
                </DataField>
              ) : null}
            </DataGrid>
          </div>
        </SectionCard>
      ) : run ? (
        <>
          <SectionCard
            title="Abrechnungslauf"
            description={runDescription(run)}
            actions={
              preview ? (
                <ButtonLink
                  href={traegerPath(
                    `/traeger/abrechnung/${run.id}/faelligkeiten`,
                  )}
                  variant="outline"
                  size="md"
                >
                  Fälligkeiten ändern
                </ButtonLink>
              ) : undefined
            }
          >
            <DataGrid columns={2}>
              <DataField label="Betreuung">
                {formatDateRange(run.serviceFrom, run.serviceUntil)}
              </DataField>
              <DataField label="Rechnungen">{run.previewCount}</DataField>
              <DataField
                label={
                  corrections.length > 0
                    ? "Summe nach Storno und Gutschrift"
                    : "Summe"
                }
              >
                <span className="tabular-nums">
                  {formatEuro(run.previewCents)}
                </span>
              </DataField>
              {corrections.length > 0 ? (
                <DataField label="Storno und Gutschrift">
                  {corrections.length === 1
                    ? "1 Dokument"
                    : `${corrections.length} Dokumente`}{" "}
                  über{" "}
                  <span className="tabular-nums">
                    {formatEuro(
                      corrections.reduce(
                        (sum, document) => sum + document.totalCents,
                        0,
                      ),
                    )}
                  </span>
                </DataField>
              ) : null}
              <DataField label="Abrechnung">
                Im Voraus, nach den bestätigten Buchungen
              </DataField>
              <DataField label="Dokumentart">
                {runDocumentKindLabel(run.documentKind)}
              </DataField>
              <DataField label="Fällig">
                {runDueDatesText(run.dueDates ?? []) ??
                  "Nach dem Zahlungsziel der Ferienbetreuung"}
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
          {preview && run.legalRemedyMissing ? (
            <Alert
              type="warning"
              title="Rechtsbehelfsbelehrung fehlt"
              message="Dieser Lauf stellt Gebührenbescheide aus. In der Vorlage „Bescheid“ fehlt die Rechtsbehelfsbelehrung. Wer das Recht „Einstellungen“ hat, muss sie eintragen. Danach können Sie freigeben."
              action={
                <ButtonLink
                  href={traegerPath(
                    "/traeger/einstellungen/briefkopf/vorlagen/bescheid",
                  )}
                  variant="surface"
                  size="compact"
                >
                  Vorlage öffnen
                </ButtonLink>
              }
            />
          ) : null}
          {preview && !run.numberRangeSet ? (
            <Alert
              type="warning"
              message="Bevor Sie freigeben: Legen Sie das Muster der Rechnungsnummern fest."
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
          {paymentTextGaps > 0 ? (
            <Alert
              type="warning"
              title="Schreiben brauchen die Debitorennummer"
              message={paymentTextWarning(run)}
            />
          ) : null}
          {preview && run.missingDebtorNumbers > 0 && paymentTextGaps === 0 ? (
            <Alert
              type="warning"
              message={
                run.missingDebtorNumbers === 1
                  ? "1 Anmeldung hat noch keine Debitorennummer. Tragen Sie sie vor dem Freigeben ein."
                  : `${run.missingDebtorNumbers} Anmeldungen haben noch keine Debitorennummer. Tragen Sie sie vor dem Freigeben ein.`
              }
            />
          ) : null}
          {preview ? <HolidayDebitHintsCard documents={run.documents} /> : null}
          {preview && run.unbilled > 0 ? (
            <Alert
              type="info"
              message={`${run.unbilled} bestätigte Anmeldung(en) haben keinen festgehaltenen Beitrag und kommen nicht in diesen Lauf.`}
            />
          ) : null}
          {run.status === "released" ? (
            <BillingLettersCard
              runId={run.id}
              payments={run.documents
                .filter((document) => document.kind === "invoice")
                .map((document) => document.payment)}
              letters={run.letters}
              onCreated={(letters) => setRun({ ...run, letters })}
              documentKind={run.documentKind}
            />
          ) : null}
          {run.status === "released" ? (
            // A Storno or a Gutschrift changes the file (#4697).
            <SepaDebitCard
              key={run.documents.length}
              kind="holiday-runs"
              runId={run.id}
            />
          ) : null}
          {run.status === "released" && run.letters ? (
            // A new Storno or Gutschrift is a new letter to send (#4690).
            <BillingMailCard key={run.documents.length} runId={run.id} />
          ) : null}
          {preview ? (
            <BillingCarriedCorrectionsCard
              run={run}
              onChange={(child, carried) =>
                setCorrecting({ kind: "carried", child, carried })
              }
              onRemoved={setRun}
            />
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
            onCancel={
              preview
                ? undefined
                : (document) =>
                    setCorrecting({ kind: "cancellation", document })
            }
            onCredit={
              preview
                ? undefined
                : (document, child, leftCents) =>
                    setCorrecting({
                      kind: "credit",
                      document,
                      child,
                      leftCents,
                    })
            }
            onReissue={preview ? undefined : () => void reissue()}
          />
          <BillingCostTakeoverCard run={run} />
        </>
      ) : null}
    </TenantPage>
  );
}
