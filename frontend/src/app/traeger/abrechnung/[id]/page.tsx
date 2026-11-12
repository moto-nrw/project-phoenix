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
// lassen sich bis zur Freigabe ändern (#4689).

import { useSession } from "next-auth/react";
import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import {
  BillingCorrectionModal,
  type BillingCorrectionTarget,
} from "~/components/traeger/billing-correction-modal";
import { BillingCarriedCorrectionsCard } from "~/components/traeger/billing-carried-corrections-card";
import {
  debitHints,
  debitOpenPoints,
  debitsHaltRelease,
  holidayDebitItems,
} from "~/components/traeger/care-debit-hints";
import {
  type OpenPoint,
  ReleaseOpenPoints,
} from "~/components/traeger/release-open-points";
import {
  BillingCostTakeoverCard,
  runCostTakeovers,
} from "~/components/traeger/billing-cost-takeover-card";
import { BillingRunOverviewCard } from "~/components/traeger/billing-run-overview-card";
import { BillingLettersCard } from "~/components/traeger/billing-letters-card";
import { BillingMailCard } from "~/components/traeger/billing-mail-card";
import {
  BILLING_RUN_STATUS_LABELS,
  BILLING_RUN_STATUS_TONES,
  numberPatternPreview,
} from "~/components/traeger/billing-format";
import { runDueDatesText } from "~/components/traeger/billing-payment-format";
import { HolidayDebitHintsCard } from "~/components/traeger/care-run-review-lazy";
import { formatEuro } from "~/components/traeger/contribution-format";
import { formatDateRange } from "~/components/traeger/holiday-period-format";
import { HolidayHeadFacts } from "~/components/traeger/holiday-period-head";
import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { ConfirmationModal } from "~/components/ui/modal";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
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
import { billingConcept } from "~/lib/concepts/billing";
import { berlinTodayISO, formatDate } from "~/lib/date-helpers";
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import {
  type BillingDocument,
  type BillingRunDetail,
  downloadBillingLetter,
  fetchBillingRun,
  fetchBillingRuns,
  releaseBillingRun,
  startBillingRun,
} from "~/lib/traeger-billing-api";
import { traegerPath } from "~/lib/traeger-url";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";

const logger = createLogger({ component: "TraegerBillingRunPage" });

/** Das Recht, das die Ferienzeiträume öffnet (#4659). */
const TRAEGER_ENROLLMENT_RIGHT = "traeger_enrollment:manage";

function runDescription(run: BillingRunDetail): string {
  if (run.status === "released") {
    return "Die Rechnungen sind ausgestellt und lassen sich nicht mehr ändern.";
  }
  // The re-issue of a cancelled invoice is explained by its own card.
  return "Vorschau aller bestätigten Anmeldungen ohne gültige Rechnung. Neue Bestätigungen kommen bis zur Freigabe dazu.";
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
  const corrections =
    run?.documents.filter((document) => document.kind !== "invoice") ?? [];
  const canRelease =
    preview &&
    run.numberRangeSet &&
    run.previewCount > 0 &&
    !run.legalRemedyMissing;
  const paymentTextGaps = preview ? run.paymentTextMissingDebtorNumbers : 0;
  // What the preview still asks to check, repeated where the release is
  // decided: numbers are final afterwards.
  const runDebitHints = preview
    ? debitHints(holidayDebitItems(run.documents))
    : [];
  // The early debit is fixed by a later due date, the rest at the mandates.
  const earlyPoints = debitOpenPoints(
    runDebitHints.filter((hint) => hint.dueTooSoon),
  );
  const mandatePoints = debitOpenPoints(
    runDebitHints.filter((hint) => !hint.dueTooSoon),
  );
  // The release refuses a direct debit without mandate: no button for it.
  const releaseHalted = debitsHaltRelease(runDebitHints);
  const carriedCount = preview
    ? run.documents.reduce(
        (sum, document) =>
          sum +
          document.children.reduce(
            (inner, child) => inner + child.carriedCorrections.length,
            0,
          ),
        0,
      )
    : 0;
  // A carried correction already counts in its new invoice: a fact of the
  // summary, not an open point that turns the release into a warning.
  const openPoints: OpenPoint[] = [
    ...earlyPoints.map((text) => ({
      text,
      action: run ? (
        <ButtonLink
          href={traegerPath(`/traeger/abrechnung/${run.id}/faelligkeiten`)}
          variant="ghost_tone"
          size="compact"
        >
          Fälligkeiten ändern
        </ButtonLink>
      ) : undefined,
    })),
    ...mandatePoints.map((text) => ({ text })),
  ];
  // Belongs to the invoice sum it is part of, not to the cost bearers'.
  const carriedNote =
    carriedCount === 0
      ? ""
      : carriedCount === 1
        ? " (darin 1 übernommene Korrektur aus einer stornierten Rechnung)"
        : ` (darin ${carriedCount} übernommene Korrekturen aus stornierten Rechnungen)`;
  const releaseWarns = paymentTextGaps > 0 || openPoints.length > 0;
  // The run sum is what the families pay; cost bearers pay their share
  // apart, so the summary names both.
  const takeoverCents = run
    ? runCostTakeovers(run).reduce((sum, takeover) => sum + takeover.cents, 0)
    : 0;
  const sumLabel = [
    corrections.length > 0 ? "Summe nach Storno und Gutschrift" : "Summe",
    takeoverCents > 0 ? "für Familien" : "",
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <TenantPage
      title={run?.periodName ?? "Abrechnungslauf"}
      leading={
        <ConceptIconTile
          concept={holidayWeeksConcept}
          size="page"
          className="max-sm:hidden"
        />
      }
      back
      backHref={traegerPath("/traeger/abrechnung")}
      backLabel="Zurück zu den Abrechnungsläufen"
      stats={
        run ? (
          <HolidayHeadFacts>
            <StatusBadge
              label={BILLING_RUN_STATUS_LABELS[run.status]}
              tone={BILLING_RUN_STATUS_TONES[run.status]}
            />
          </HolidayHeadFacts>
        ) : null
      }
      statsLoading={!run && !pageLoad.error}
      // The cross-link stands with the actions, as „Zur Abrechnung“ does on
      // the registrations; the status line carries only facts.
      actions={
        (run && canOpenPeriod) || preview ? (
          <>
            {run && canOpenPeriod ? (
              <ButtonLink
                href={traegerPath(
                  `/traeger/ferien/${encodeURIComponent(run.holidayPeriodId)}`,
                )}
                variant="outline"
                size="md"
              >
                <MotoConceptIcon
                  concept={holidayWeeksConcept}
                  size={18}
                  className="mr-2"
                />
                Zum Ferienzeitraum
              </ButtonLink>
            ) : null}
            {preview ? (
              <Button
                type="button"
                variant="primary"
                size="md"
                disabled={!canRelease}
                onClick={() => setConfirmingRelease(true)}
              >
                Lauf freigeben
              </Button>
            ) : null}
          </>
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
              onIssued={() => {
                setCorrecting(null);
                void load();
              }}
            />
            <ConfirmationModal
              isOpen={confirmingRelease}
              title="Lauf freigeben?"
              confirmText={
                releaseWarns && !releaseHalted
                  ? "Trotzdem freigeben"
                  : "Freigeben"
              }
              confirmVariant={
                releaseWarns && !releaseHalted ? "warning" : "primary"
              }
              isConfirmDisabled={releaseHalted}
              isConfirmLoading={releasing}
              loadingText="Wird freigegeben …"
              onClose={() => setConfirmingRelease(false)}
              onConfirm={() => void release()}
            >
              <p className="text-sm text-gray-700">
                {run.previewCount === 1
                  ? `1 Rechnung über ${formatEuro(run.previewCents)}${carriedNote} bekommt ihre Nummer. Danach lässt sie sich nicht mehr ändern.`
                  : `${run.previewCount} Rechnungen über ${formatEuro(run.previewCents)}${carriedNote} bekommen ihre Nummer. Danach lassen sie sich nicht mehr ändern.`}
                {takeoverCents > 0
                  ? ` Kostenträger übernehmen zusätzlich ${formatEuro(takeoverCents)}.`
                  : ""}
              </p>
              {paymentTextGaps > 0 ? (
                <Alert
                  type="warning"
                  className="mt-3"
                  message={`${invoicesWithoutDebtorNumber(paymentTextGaps)} Ihr Zahlungstext nennt aber die Debitorennummer. Nach dem Freigeben können Sie sie nicht mehr eintragen. Schreiben gehen dann nur ohne den Platzhalter im Zahlungstext.`}
                />
              ) : null}
              <ReleaseOpenPoints points={openPoints} />
            </ConfirmationModal>
          </>
        ) : null
      }
    >
      {run ? (
        <>
          <SectionCard
            leading={<ConceptIconTile concept={billingConcept} />}
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
              <DataField label={sumLabel}>
                <span className="tabular-nums">
                  {formatEuro(run.previewCents)}
                </span>
              </DataField>
              {takeoverCents > 0 ? (
                <DataField label="Kostenübernahme">
                  <span className="tabular-nums">
                    {formatEuro(takeoverCents)}
                  </span>{" "}
                  zahlen Kostenträger
                </DataField>
              ) : null}
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
                  <>
                    <span className="text-gray-600">z.&nbsp;B. </span>
                    <span className="font-mono">
                      {numberPatternPreview(
                        run.numberPattern,
                        berlinTodayISO(),
                      )}
                    </span>{" "}
                    <span className="text-gray-600">
                      (die Rechnungsnummer kommt mit der Freigabe)
                    </span>
                  </>
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
              message="Dieser Lauf stellt Gebührenbescheide aus. In der Vorlage „Gebührenbescheid“ fehlt die Rechtsbehelfsbelehrung. Wer die Rolle „Verwaltung“ hat, muss sie eintragen. Danach können Sie freigeben."
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
          {preview ? (
            <HolidayDebitHintsCard
              documents={run.documents}
              dueDatesHref={`/traeger/abrechnung/${run.id}/faelligkeiten`}
            />
          ) : null}
          {preview && run.unbilled > 0 ? (
            <Alert
              type="info"
              message={
                run.unbilled === 1
                  ? "1 bestätigte Anmeldung hat keinen festgehaltenen Beitrag und kommt nicht in diesen Lauf."
                  : `${run.unbilled} bestätigte Anmeldungen haben keinen festgehaltenen Beitrag und kommen nicht in diesen Lauf.`
              }
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
