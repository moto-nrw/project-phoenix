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
import { useParams, useRouter } from "next/navigation";
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
import { CareLettersSummaryCard } from "~/components/traeger/care-letters-summary-card";
import { CareRunMealsCard } from "~/components/traeger/care-run-meals";
import { runDueDatesText } from "~/components/traeger/billing-payment-format";
import { invoiceDateLabel } from "~/components/traeger/care-review-format";
import {
  CareExcludedCard,
  CareReviewExport,
  exclusionColumn,
} from "~/components/traeger/care-run-review-lazy";
import { formatEuro } from "~/components/traeger/contribution-format";
import { HolidayHeadFacts } from "~/components/traeger/holiday-period-head";
import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { ConfirmationModal } from "~/components/ui/modal";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
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
  type CareInvoice,
  type CareLetters,
  type CareRunDetail,
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

  const include = async (invoice: CareInvoice) => {
    if (!run) return;
    setReturning(invoice.contractId);
    try {
      setRun(await includeCareInvoice(run.id, invoice.contractId));
      success(
        `Die Rechnung von ${invoice.childFirstName} ${invoice.childLastName} ist wieder im Lauf.`,
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

  const preview = run?.status === "calculated";
  const canRelease =
    preview &&
    (run.previewCount > 0 ? run.numberRangeSet : run.excludedCount > 0);
  const issuable = run?.invoices.filter((invoice) => !invoice.exclusion) ?? [];
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
      {run ? (
        <>
          {preview ? (
            <>
              <CareExcludeModal
                runId={run.id}
                invoice={excluding}
                onClose={() => setExcluding(null)}
                onExcluded={(next) => {
                  setRun(next);
                  setExcluding(null);
                  success("Die Rechnung ist ausgeschlossen.");
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
                ? "Prüfen Sie die Vorschau. Neue Verträge und Preise kommen bis zur Freigabe dazu. Eine Rechnung, die noch nicht stimmt, schließen Sie über das Menü ihrer Zeile aus."
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
              {run.billed > 0 ? (
                <DataField label="Schon abgerechnet">
                  {run.billed === 1
                    ? "1 Kind aus einem früheren Lauf"
                    : `${run.billed} Kinder aus früheren Läufen`}
                </DataField>
              ) : null}
            </DataGrid>
          </SectionCard>
          {preview && run.previewCount > 0 && !run.numberRangeSet ? (
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
          {preview && (run.uncountedFamilies ?? 0) > 0 ? (
            <Alert
              type="info"
              message={`${run.uncountedFamilies === 1 ? "Für 1 Familie" : `Für ${run.uncountedFamilies ?? 0} Familien`} gibt es in diesem Monat keine bestätigte Kinderzahl. Die Geschwisterregel zählt dort nur die Kinder mit Vertrag.`}
              action={
                <ButtonLink
                  href={traegerPath("/traeger/abrechnung/kinderzahl")}
                  variant="surface"
                  size="compact"
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
          <CareRunInvoices
            invoices={preview ? issuable : run.invoices}
            month={run.billingMonth}
            released={!preview}
            letters={released ? letters?.documents : undefined}
            letterColumns={
              preview
                ? [exclusionColumn(setExcluding, openFamilyBilling)]
                : undefined
            }
            onFamilyBilling={preview ? undefined : openFamilyBilling}
          />
        </>
      ) : null}
    </TenantPage>
  );
}
