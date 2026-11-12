"use client";

// Ein Monatslauf (#4684): die Vorschau aller Rechnungen eines Monats nach
// dem gültigen Vertrag und der Preisliste, und die Freigabe, die jede
// Rechnung mit ihrer Nummer ausstellt. Danach ändern sich die Rechnungen
// nicht mehr. Vor der Freigabe ist die Prüfphase (#5916): Rechnungsdatum
// ändern, einzelne Rechnungen mit Grund ausschließen, Prüfliste laden. Die
// Freigabe stellt genau den geprüften Stand aus. Ein freigegebener Lauf zeigt den Stand seiner Schreiben an die
// Familien; Erstellen, Senden, Drucken und Stornieren liegen auf der Seite
// „Schreiben“ ([id]/schreiben). Jedes Öffnen steht im Protokoll des Laufs.

import dynamic from "next/dynamic";
import { ArchiveX, Inbox } from "lucide-react";
import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import {
  CARE_RUN_STATUS_LABELS,
  CARE_RUN_STATUS_TONES,
  MONTH_RULE_LABELS,
  formatMonth,
  invoiceCountLabel,
  isEmptyCareRun,
} from "~/components/traeger/care-billing-format";
import {
  CareRunInvoices,
  CareDuplicatesCard,
  CareUnpricedCard,
} from "~/components/traeger/care-run-invoices";
import { CareLettersSummaryCard } from "~/components/traeger/care-letters-summary-card";
import { CareRunMealsCard } from "~/components/traeger/care-run-meals";
import { runDueDatesText } from "~/components/traeger/billing-payment-format";
import {
  invoiceDateLabel,
  invoiceKindLabel,
} from "~/components/traeger/care-review-format";
import {
  CareDebitHintsCard,
  CareExcludedCard,
  CareReviewExport,
  exclusionColumn,
} from "~/components/traeger/care-run-review-lazy";
import { formatEuro } from "~/components/traeger/contribution-format";
import { HolidayHeadFacts } from "~/components/traeger/holiday-period-head";
import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { EmptyState } from "~/components/ui/empty-state";
import { ConfirmationModal } from "~/components/ui/modal";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { runDocumentKindLabel } from "~/components/traeger/run-document-kind";
import { TenantPage } from "~/components/ui/tenant-page";
import { WizardStepper } from "~/components/ui/wizard-stepper";
import {
  useApiErrorDisplay,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  type CareFeeNoticeHold,
  type CareInvoice,
  type CareLetters,
  type CareRunDetail,
  discardCareRun,
  fetchCareLetters,
  fetchCareRun,
  releaseCareRun,
} from "~/lib/traeger-care-billing-api";
import { includeCareInvoice } from "~/lib/traeger-care-review-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerCareRunPage" });

// The dialogs of the review load when one opens (#5916).
const CareExcludeModal = dynamic(
  () =>
    import("~/components/traeger/care-run-review-modals").then(
      (module) => module.CareExcludeModal,
    ),
  { ssr: false },
);
// Only a run in which a cost bearer pays a part loads its card (#4687).
const CareCostBearerCard = dynamic(
  () =>
    import("~/components/traeger/care-cost-bearer-card").then(
      (module) => module.CareCostBearerCard,
    ),
  { ssr: false },
);
// The direct debit file of a released run (#4697) loads with the run.
const SepaDebitCard = dynamic(
  () =>
    import("~/components/traeger/sepa-debit-card").then(
      (module) => module.SepaDebitCard,
    ),
  { ssr: false },
);
const CareInvoiceDateModal = dynamic(
  () =>
    import("~/components/traeger/care-run-review-modals").then(
      (module) => module.CareInvoiceDateModal,
    ),
  { ssr: false },
);

/** The three steps of a monthly run; the page shows where it stands. */
const RUN_STEPS = [
  "Vorschau prüfen",
  "Freigeben: Rechnungen bekommen ihre Nummer",
  "An die Familien senden",
] as const;

/**
 * "1 Kind mit doppeltem Vertrag wird nicht abgerechnet.", after the release
 * "… ist nicht abgerechnet."
 */
function feeNoticeHeldText(held: readonly CareFeeNoticeHold[]): string {
  const names = held
    .slice(0, 5)
    .map((hold) => `${hold.childFirstName} ${hold.childLastName}`.trim());
  const rest = held.length - names.length;
  const list =
    rest > 0
      ? `${names.join(", ")} und ${rest} weitere`
      : names.length > 1
        ? `${names.slice(0, -1).join(", ")} und ${names.at(-1)}`
        : (names[0] ?? "");
  const subject =
    held.length === 1
      ? `Der Jahresbescheid von ${list} setzt diesen Monat auf 0,00 €.`
      : `Die Jahresbescheide von ${list} setzen diesen Monat auf 0,00 €.`;
  return `${subject} Der Monat bleibt gesperrt. Dieser Lauf rechnet dafür keine Betreuung ab. Wollen Sie den Monat abrechnen, stellen Sie einen neuen Bescheid aus.`;
}

/** Why „Lauf freigeben“ is blocked, in the order the warnings show. */
function releaseBlockReason(run: CareRunDetail): string | undefined {
  if (run.previewCount === 0) {
    if (run.excludedCount > 0 || (run.freeOfCharge ?? 0) > 0) return undefined;
    if (run.unpriced.length > 0) {
      return "Noch keine Rechnung: Für einige Kinder fehlt ein Preis.";
    }
    if (run.duplicates.length > 0) {
      return "Noch keine Rechnung: Einige Kinder haben einen doppelten Vertrag.";
    }
    return "Der Lauf hat keine Rechnung zum Freigeben.";
  }
  if (run.legalRemedyMissing) {
    return "Tragen Sie zuerst die Rechtsbehelfsbelehrung in der Vorlage „Bescheid“ ein.";
  }
  if (!run.numberRangeSet) {
    return "Legen Sie zuerst das Muster der Rechnungsnummern für Monatsbeiträge fest.";
  }
  return undefined;
}

function notBilledLabel(children: number, released = false): string {
  if (children === 1) {
    return `1 Kind mit doppeltem Vertrag ${released ? "ist" : "wird"} nicht abgerechnet.`;
  }
  return `${children} Kinder mit doppeltem Vertrag ${released ? "sind" : "werden"} nicht abgerechnet.`;
}

export default function TraegerCareRunPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const runId = params?.id ?? "";
  const [run, setRun] = useState<CareRunDetail | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [releasing, setReleasing] = useState(false);
  const [confirmingDiscard, setConfirmingDiscard] = useState(false);
  const [discarding, setDiscarding] = useState(false);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;
  const releaseErrors = useApiErrorDisplay();
  const { success } = useToast();
  const [letters, setLetters] = useState<CareLetters | null>(null);
  const [excluding, setExcluding] = useState<CareInvoice | null>(null);
  const [editingDate, setEditingDate] = useState(false);
  const [returning, setReturning] = useState<string | null>(null);
  const reviewErrors = useApiErrorDisplay();
  const lettersLoad = useApiLoadError();
  const { show: showLettersError, clear: clearLettersError } = lettersLoad;

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

  const released = run?.status === "released";
  const loadLetters = useCallback(async () => {
    try {
      setLetters(await fetchCareLetters(runId));
      clearLettersError();
    } catch (err) {
      logger.error("traeger_care_letters_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLettersError(err, {
        object: "die Schreiben",
        retry: () => void loadLetters(),
      });
    }
  }, [runId, clearLettersError, showLettersError]);

  useEffect(() => {
    if (released && run.previewCount > 0) void loadLetters();
  }, [released, run?.previewCount, loadLetters]);

  const release = async () => {
    if (!run) return;
    setReleasing(true);
    try {
      const released = await releaseCareRun(run);
      setRun(released);
      setConfirming(false);
      success(
        released.previewCount === 0
          ? "Der Lauf ist ohne Rechnungen abgeschlossen."
          : released.duplicateChildren > 0
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

  const discard = async () => {
    if (!run) return;
    setDiscarding(true);
    try {
      setRun(await discardCareRun(run.id));
      setConfirmingDiscard(false);
      success(
        `Der Lauf ist verworfen. Für ${formatMonth(run.billingMonth)} können Sie einen neuen Lauf starten.`,
      );
    } catch (err) {
      logger.error("traeger_care_run_discard_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setConfirmingDiscard(false);
      void releaseErrors.show(err, { object: "der Monatslauf" });
      void load();
    } finally {
      setDiscarding(false);
    }
  };

  const include = async (invoice: CareInvoice) => {
    if (!run) return;
    setReturning(`${invoice.contractId}-${invoice.service}`);
    try {
      setRun(await includeCareInvoice(run.id, invoice));
      success(
        `${invoiceKindLabel(invoice)} von ${invoice.childFirstName} ${invoice.childLastName} ist wieder im Lauf.`,
      );
    } catch (err) {
      logger.error("traeger_care_invoice_include_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void reviewErrors.show(err, { object: "die Rechnung" });
      void load();
    } finally {
      setReturning(null);
    }
  };

  // Recipient and payment of a family are set on their own page, so the run
  // does not load that form (#4689).
  const openFamilyBilling = (invoice: CareInvoice) => {
    if (!run) return;
    router.push(
      traegerPath(
        `/traeger/abrechnung/monatsbeitraege/${run.id}/empfaenger/${invoice.contractId}`,
      ),
    );
  };

  // One invoice is changed on its own page (#6197), so the run does not load
  // the forms and the history.
  const openAmend = (invoice: CareInvoice) => {
    if (!run) return;
    const service = invoice.service === "meal" ? "?leistung=mensa" : "";
    router.push(
      traegerPath(
        `/traeger/abrechnung/monatsbeitraege/${run.id}/rechnung/${invoice.contractId}${service}`,
      ),
    );
  };

  const preview = run?.status === "calculated";
  const discarded = run?.status === "discarded";
  const empty = run ? isEmptyCareRun(run) : false;
  const canRelease =
    preview &&
    !(run.previewCount > 0 && run.legalRemedyMissing) &&
    (run.previewCount > 0
      ? run.numberRangeSet
      : run.excludedCount > 0 || (run.freeOfCharge ?? 0) > 0);
  const issuable = run?.invoices.filter((invoice) => !invoice.exclusion) ?? [];
  const conflicted = issuable.find((invoice) =>
    (invoice.amendments ?? []).some(
      (amendment) => amendment.revocation === null && amendment.conflict,
    ),
  );
  const excluded = run?.invoices.filter((invoice) => invoice.exclusion) ?? [];

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
            {discarded ? (
              <span>
                Verworfen am{" "}
                {run.discardedAt ? formatDate(run.discardedAt) : "–"}
              </span>
            ) : (
              <span className="tabular-nums">
                {invoiceCountLabel(run.previewCount)} ·{" "}
                {formatEuro(run.previewCents)}
              </span>
            )}
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
            tooltip="moto fragt vorher nach, dann ist der Monat frei für einen neuen Lauf."
            onClick={() => setConfirmingDiscard(true)}
          >
            Leeren Lauf verwerfen
          </Button>
        ) : preview ? (
          <Button
            type="button"
            variant="primary"
            size="md"
            disabled={!canRelease}
            disabledReason={releaseBlockReason(run)}
            tooltip={
              run.previewCount === 0
                ? "moto fragt vorher nach, dann ist der Lauf ohne Rechnungen abgeschlossen."
                : "moto fragt vorher nach, dann bekommen alle Rechnungen ihre feste Nummer."
            }
            onClick={() => setConfirming(true)}
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
                Der Monatslauf für {formatMonth(run.billingMonth)} stellt nichts
                aus. Er bleibt als „Verworfen“ in der Liste.
              </p>
              <p>
                Danach ist der Monat frei. Sie können sofort einen neuen Lauf
                starten.
              </p>
            </div>
          </ConfirmationModal>
        ) : run ? (
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
              {run.previewCount === 0 ? (
                "Der Lauf wird ohne Rechnungen abgeschlossen. Die Schulen sind danach wieder frei."
              ) : (
                <>
                  {invoiceCountLabel(run.previewCount)} über{" "}
                  {formatEuro(run.previewCents)} für{" "}
                  {formatMonth(run.billingMonth)} bekommen ihre Nummer und das
                  Rechnungsdatum {invoiceDateLabel(run.invoiceDate)}. Danach
                  lassen sie sich nicht mehr ändern.
                </>
              )}
            </p>
            {run.excludedCount > 0 ? (
              <p className="mt-3 text-sm text-gray-700">
                {run.excludedCount === 1
                  ? "1 ausgeschlossene Rechnung bekommt keine Nummer."
                  : `${run.excludedCount} ausgeschlossene Rechnungen bekommen keine Nummer.`}
              </p>
            ) : null}
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
      {run && !discarded && run.feeNoticeHeld?.length ? (
        <Alert
          type="warning"
          title="Durch aufgehobenen Bescheid gesperrt"
          message={feeNoticeHeldText(run.feeNoticeHeld)}
          action={
            <ButtonLink
              href={traegerPath("/traeger/abrechnung/jahresbescheide")}
              variant="surface"
              size="compact"
              tooltip="Öffnet die Jahresbescheide, dort starten Sie einen neuen Jahreslauf."
            >
              Neuen Bescheid ausstellen
            </ButtonLink>
          }
        />
      ) : null}
      {run && (discarded || empty) ? (
        <>
          <SectionCard title="Monatslauf">
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
                  ? `Er hatte nichts abzurechnen und hat nichts ausgestellt. Für ${formatMonth(run.billingMonth)} können Sie einen neuen Lauf starten.`
                  : `Für ${formatMonth(run.billingMonth)} gibt es keine Rechnung. Es fehlt auch kein Preis. Sie können den Lauf verwerfen. Danach ist der Monat frei für einen neuen Lauf.`
              }
              className="py-6"
            />
            <div className="mt-2 border-t border-gray-100 pt-5">
              <DataGrid columns={2}>
                <DataField label="Zeitraum">
                  {formatMonth(run.billingMonth)}
                </DataField>
                <DataField label="Schulen">
                  {run.schools.map((school) => school.name).join(", ") || "–"}
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
          {/* Mensa months a school has not released yet come with a later
              run; the empty run still names them. */}
          {empty ? (
            <CareRunMealsCard
              gaps={run.mealGaps}
              unbilled={run.mealUnbilled}
              changed={run.mealChanged}
            />
          ) : null}
        </>
      ) : run ? (
        <>
          {preview ? (
            <>
              <CareExcludeModal
                runId={run.id}
                invoice={excluding}
                invoices={run.invoices}
                onClose={() => setExcluding(null)}
                onExcluded={(next, invoice) => {
                  setRun(next);
                  setExcluding(null);
                  success(
                    `${invoiceKindLabel(invoice)} von ${invoice.childFirstName} ${invoice.childLastName} ist ausgeschlossen.`,
                  );
                }}
              />
              <CareInvoiceDateModal
                run={run}
                isOpen={editingDate}
                onClose={() => setEditingDate(false)}
                onSaved={(next) => {
                  setRun(next);
                  setEditingDate(false);
                  success("Das Rechnungsdatum ist gespeichert.");
                }}
              />
            </>
          ) : null}
          <SectionCard
            title="Monatslauf"
            description={
              preview
                ? "Prüfen Sie die Vorschau. Neue Verträge und Preise kommen bis zur Freigabe dazu. Stimmt eine Rechnung noch nicht, ändern Sie sie oder schließen sie aus. Beides geht über das Menü ihrer Zeile."
                : run.previewCount === 0
                  ? "Der Lauf ist ohne Rechnungen abgeschlossen. Die Schulen sind wieder frei."
                  : "Die Rechnungen sind ausgestellt und lassen sich nicht mehr ändern. Eine falsche Rechnung stornieren Sie auf der Seite „Schreiben“."
            }
            action={preview ? <CareReviewExport runId={run.id} /> : undefined}
          >
            <WizardStepper
              steps={RUN_STEPS}
              current={preview ? 0 : 1}
              className="mb-5"
            />
            <DataGrid columns={2}>
              <DataField label="Monat">
                {formatMonth(run.billingMonth)}
              </DataField>
              <DataField label="Schulen">
                {preview
                  ? run.schools.map((school) => school.name).join(", ") || "–"
                  : run.schoolIds.length === 0
                    ? "–"
                    : run.schoolIds.length === 1
                      ? "1 Schule"
                      : `${run.schoolIds.length} Schulen`}
              </DataField>
              <DataField label="Rechnungsdatum">
                {preview ? (
                  <span className="flex flex-wrap items-center gap-x-3 gap-y-1">
                    <span>{invoiceDateLabel(run.invoiceDate)}</span>
                    <Button
                      type="button"
                      variant="link"
                      size="md"
                      aria-label="Rechnungsdatum ändern"
                      tooltip="Öffnet ein Fenster, um das Rechnungsdatum des Laufs zu ändern."
                      onClick={() => setEditingDate(true)}
                    >
                      Ändern
                    </Button>
                  </span>
                ) : run.issuedOn ? (
                  formatDate(run.issuedOn)
                ) : (
                  "–"
                )}
              </DataField>
              <DataField label="Fällig">
                {preview ? (
                  <span className="flex flex-wrap items-center gap-x-3 gap-y-1">
                    <span>
                      {runDueDatesText(run.dueDates ?? []) ??
                        "Nach dem Zahlungsziel der Leistung"}
                    </span>
                    <ButtonLink
                      href={traegerPath(
                        `/traeger/abrechnung/monatsbeitraege/${run.id}/faelligkeiten`,
                      )}
                      variant="link"
                      size="md"
                      aria-label="Fälligkeiten ändern"
                      tooltip="Öffnet die Fälligkeiten der Rechnungen dieses Laufs zum Ändern."
                    >
                      Ändern
                    </ButtonLink>
                  </span>
                ) : (
                  (runDueDatesText(run.dueDates ?? []) ??
                  "Nach dem Zahlungsziel der Leistung")
                )}
              </DataField>
              <DataField label="Rechnungen">
                <span className="tabular-nums">
                  {invoiceCountLabel(run.previewCount)} ·{" "}
                  {formatEuro(run.previewCents)}
                </span>
              </DataField>
              <DataField label="Abgerechnet wird">
                Module nach Vertrag, Mensa nach Freigabe der Schule
              </DataField>
              <DataField label="Vertrag ändert sich im Monat">
                {run.monthRule ? MONTH_RULE_LABELS[run.monthRule] : "–"}
              </DataField>
              <DataField label="Dokumentart">
                {runDocumentKindLabel(run.documentKind)}
              </DataField>
              <DataField label="Rechnungsnummern">
                {run.numberRangeSet ? (
                  <span className="font-mono">{run.numberPattern}</span>
                ) : (
                  "Noch kein Muster"
                )}
              </DataField>
              {preview && run.excludedCount > 0 ? (
                <DataField label="Ausgeschlossen">
                  <span className="tabular-nums">
                    {invoiceCountLabel(run.excludedCount)} ·{" "}
                    {formatEuro(run.excludedCents)}
                  </span>
                </DataField>
              ) : null}
              {(run.freeOfCharge ?? 0) > 0 ? (
                <DataField label="Beitragsfreier Monat">
                  {run.freeOfCharge === 1
                    ? "1 Vertrag ohne Rechnung"
                    : `${run.freeOfCharge} Verträge ohne Rechnung`}
                </DataField>
              ) : null}
              {run.billed > 0 ? (
                <DataField label="Schon abgerechnet">
                  {run.billed === 1
                    ? "1 Kind aus einem früheren Lauf"
                    : `${run.billed} Kinder aus früheren Läufen`}
                </DataField>
              ) : null}
            </DataGrid>
          </SectionCard>
          {preview && run.previewCount > 0 && run.legalRemedyMissing ? (
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
                  tooltip="Öffnet die Vorlage „Bescheid“ in den Einstellungen."
                >
                  Vorlage öffnen
                </ButtonLink>
              }
            />
          ) : null}
          {preview && run.previewCount > 0 && !run.numberRangeSet ? (
            <Alert
              type="warning"
              message="Bevor Sie freigeben: Legen Sie das Muster der Rechnungsnummern für Monatsbeiträge fest."
              action={
                <ButtonLink
                  href={traegerPath("/traeger/einstellungen/abrechnung")}
                  variant="surface"
                  size="compact"
                  tooltip="Öffnet die Einstellungen für das Muster der Rechnungsnummern."
                >
                  Abrechnungsvorgaben öffnen
                </ButtonLink>
              }
            />
          ) : null}
          {preview && (run.amendmentConflicts ?? 0) > 0 ? (
            <Alert
              type="warning"
              message={`${run.amendmentConflicts === 1 ? "Eine Änderung einer Rechnung passt" : `${run.amendmentConflicts ?? 0} Änderungen von Rechnungen passen`} nicht mehr. Nehmen Sie sie auf der Seite der Rechnung zurück. Erst dann können Sie freigeben.`}
              action={
                conflicted ? (
                  <Button
                    type="button"
                    variant="surface"
                    size="compact"
                    tooltip="Öffnet die Rechnung, dort nehmen Sie die Änderung zurück."
                    onClick={() => openAmend(conflicted)}
                  >
                    Rechnung öffnen
                  </Button>
                ) : undefined
              }
            />
          ) : null}
          {preview && (run.uncountedFamilies ?? 0) > 0 ? (
            <Alert
              type="info"
              message={`${run.uncountedFamilies === 1 ? "Für 1 Familie" : `Für ${run.uncountedFamilies ?? 0} Familien`} gibt es in diesem Monat keine bestätigte Kinderzahl. Die Geschwisterregel zählt dort nur die Kinder mit Vertrag.`}
              action={
                <ButtonLink
                  href={traegerPath("/traeger/abrechnung/kinderzahl")}
                  variant="surface"
                  size="compact"
                  tooltip="Öffnet die Kinderzahl der Familien zum Bestätigen."
                >
                  Kinderzahl prüfen
                </ButtonLink>
              }
            />
          ) : null}
          <CareDuplicatesCard duplicates={run.duplicates} released={!preview} />
          {preview ? <CareUnpricedCard unpriced={run.unpriced} /> : null}
          {preview ? (
            <CareRunMealsCard
              gaps={run.mealGaps}
              unbilled={run.mealUnbilled}
              changed={run.mealChanged}
            />
          ) : null}
          {preview ? <CareDebitHintsCard invoices={run.invoices} /> : null}
          {preview ? (
            <CareExcludedCard
              invoices={excluded}
              returning={returning}
              onInclude={(invoice) => void include(invoice)}
            />
          ) : null}
          {released && run.previewCount > 0 ? (
            <CareLettersSummaryCard
              href={traegerPath(
                `/traeger/abrechnung/monatsbeitraege/${run.id}/schreiben`,
              )}
              letters={letters}
              loadError={lettersLoad.error}
            />
          ) : null}
          {released && run.previewCount > 0 ? (
            <SepaDebitCard kind="care-runs" runId={run.id} />
          ) : null}
          {run.costBearers.length > 0 ? (
            <CareCostBearerCard
              runId={run.id}
              released={!preview}
              totals={run.costBearers}
            />
          ) : null}
          <CareRunInvoices
            invoices={preview ? issuable : run.invoices}
            month={run.billingMonth}
            released={!preview}
            letters={released ? letters?.documents : undefined}
            letterColumns={
              preview
                ? [exclusionColumn(setExcluding, openFamilyBilling, openAmend)]
                : undefined
            }
            onFamilyBilling={preview ? undefined : openFamilyBilling}
          />
        </>
      ) : null}
    </TenantPage>
  );
}
