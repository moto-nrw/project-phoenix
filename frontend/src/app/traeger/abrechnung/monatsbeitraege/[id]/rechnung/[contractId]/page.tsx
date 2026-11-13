"use client";

// Eine Rechnung eines Monatslaufs vor der Ausstellung ändern (#6197):
// Betrag, Empfänger, Anschrift und Positionen, jeweils mit Grund und nur
// für diese Rechnung dieses Laufs. Eine eigene Seite, damit die Seite des
// Laufs Formulare und Verlauf nicht lädt. Nach der Ausstellung zeigt sie
// die Rechnung mit ihren Änderungen und ändert nichts mehr.

import { ChevronDown } from "lucide-react";
import { useParams, useSearchParams } from "next/navigation";
import { Suspense, useCallback, useEffect, useState } from "react";
import { formatMonth } from "~/components/traeger/care-billing-format";
import {
  type CareAmendmentDraft,
  CareInvoiceAmendmentModal,
} from "~/components/traeger/care-invoice-amendment-modal";
import {
  CareAmendedInvoiceCard,
  CareAmendmentHistoryCard,
} from "~/components/traeger/care-invoice-amendments";
import { formatEuro } from "~/components/traeger/contribution-format";
import { InvoiceExportHistoryCard } from "~/components/traeger/invoice-export-history-card";
import { Alert } from "~/components/ui/alert";
import { ButtonLink } from "~/components/ui/button";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { TenantPage } from "~/components/ui/tenant-page";
import { useApiLoadError, useToast } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { createLogger } from "~/lib/logger";
import {
  type CareInvoice,
  type CareRunDetail,
  fetchCareRun,
} from "~/lib/traeger-care-billing-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerCareInvoiceAmendPage" });

function invoiceOf(
  run: CareRunDetail,
  contractId: string,
  service: "care" | "meal",
): CareInvoice | null {
  return (
    run.invoices.find(
      (entry) => entry.contractId === contractId && entry.service === service,
    ) ?? null
  );
}

function CareInvoiceAmendPage() {
  const params = useParams<{ id: string; contractId: string }>();
  const search = useSearchParams();
  const runId = params?.id ?? "";
  const contractId = params?.contractId ?? "";
  const service = search?.get("leistung") === "mensa" ? "meal" : "care";
  const [run, setRun] = useState<CareRunDetail | null>(null);
  const [draft, setDraft] = useState<CareAmendmentDraft | null>(null);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;
  const { success } = useToast();
  const runHref = traegerPath(`/traeger/abrechnung/monatsbeitraege/${runId}`);

  const load = useCallback(async () => {
    try {
      const loaded = await fetchCareRun(runId);
      if (!invoiceOf(loaded, contractId, service)) {
        throw new ApiError("Die Rechnung gehört nicht zu diesem Lauf.", 404);
      }
      setRun(loaded);
      clearLoadError();
    } catch (err) {
      logger.error("traeger_care_invoice_amend_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Rechnung",
        retry: () => void load(),
      });
    }
  }, [runId, contractId, service, showLoadError, clearLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const invoice = run ? invoiceOf(run, contractId, service) : null;
  const open = run?.status === "calculated";
  const amendable = run?.amendable;
  const childName = invoice
    ? `${invoice.childFirstName} ${invoice.childLastName}`.trim()
    : "";
  const conflicts = invoice
    ? (invoice.amendments ?? []).filter(
        (amendment) => amendment.revocation === null && amendment.conflict,
      ).length
    : 0;
  const menu =
    open && amendable
      ? [
          ...(amendable.amount
            ? [
                {
                  label: "Betrag ändern",
                  description: "Mit einer Korrektur-Zeile",
                  kind: "amount" as const,
                },
              ]
            : []),
          ...(amendable.positions
            ? [
                {
                  label: "Position hinzufügen",
                  description: "Zum Beispiel ein Ausflug",
                  kind: "add_position" as const,
                },
              ]
            : []),
          ...(amendable.recipient
            ? [
                {
                  label: "Empfänger ändern",
                  description: "Name und E-Mail, nur diese Rechnung",
                  kind: "recipient" as const,
                },
              ]
            : []),
          ...(amendable.address
            ? [
                {
                  label: "Anschrift ändern",
                  description: "Nur diese Rechnung",
                  kind: "address" as const,
                },
              ]
            : []),
        ]
      : [];
  const allOff = open && amendable && menu.length === 0;
  const someOff =
    open &&
    amendable &&
    menu.length > 0 &&
    !(
      amendable.amount &&
      amendable.positions &&
      amendable.recipient &&
      amendable.address
    );

  return (
    <TenantPage
      title={
        childName
          ? open
            ? `Rechnung ändern: ${childName}`
            : `Rechnung: ${childName}`
          : "Rechnung ändern"
      }
      back
      backHref={runHref}
      backLabel="Zurück zum Monatslauf"
      stats={
        run && invoice ? (
          <span>
            {service === "meal" ? "Mensa" : "Monatsbeitrag"}{" "}
            {formatMonth(run.billingMonth)} · {invoice.schoolName} ·{" "}
            <span className="tabular-nums">
              {formatEuro(invoice.totalCents)}
            </span>
          </span>
        ) : null
      }
      statsLoading={!run && !pageLoad.error}
      info="Hier ändern Sie eine einzelne Rechnung vor dem Ausstellen. Die Änderung gilt nur für diese Rechnung in diesem Lauf. Nach dem Ausstellen geht eine Korrektur nur noch mit Storno und neuer Rechnung."
      actions={
        menu.length > 0 ? (
          <OverflowMenu
            ariaLabel="Rechnung ändern"
            triggerButton={{ variant: "primary", size: "md" }}
            // The arrow says that a menu opens, as on the download menus.
            triggerClassName="gap-2 whitespace-nowrap"
            triggerContent={
              <>
                Ändern
                <ChevronDown className="h-4 w-4" aria-hidden />
              </>
            }
            items={menu.map((entry) => ({
              label: entry.label,
              description: entry.description,
              onClick: () => setDraft({ kind: entry.kind }),
            }))}
          />
        ) : null
      }
      error={pageLoad.error}
      loading={!pageLoad.error && !invoice}
    >
      {run && invoice ? (
        <>
          {!open ? (
            <Alert
              type="info"
              message="Diese Rechnung ist ausgestellt und ändert sich nicht mehr. Für eine Korrektur stornieren Sie sie. Der nächste Lauf des Monats stellt sie neu aus und übernimmt die Änderungen."
              action={
                <ButtonLink
                  href={`${runHref}/schreiben`}
                  variant="surface"
                  size="compact"
                  tooltip="Öffnet die Schreiben des Laufs, dort stornieren Sie Rechnungen."
                >
                  Zu den Schreiben
                </ButtonLink>
              }
              actionLayout="responsive"
            />
          ) : null}
          {conflicts > 0 ? (
            <Alert
              type="warning"
              message={
                conflicts === 1
                  ? "Eine Änderung passt nicht mehr zur Rechnung. Nehmen Sie sie zurück. Erst dann können Sie den Lauf freigeben."
                  : `${conflicts} Änderungen passen nicht mehr zur Rechnung. Nehmen Sie sie zurück. Erst dann können Sie den Lauf freigeben.`
              }
            />
          ) : null}
          {invoice.exclusion ? (
            <Alert
              type="info"
              message={`Diese Rechnung ist ausgeschlossen. Grund: ${invoice.exclusion.reason}. Ihre Änderungen gelten, wenn Sie sie wieder aufnehmen.`}
            />
          ) : null}
          {allOff ? (
            <Alert
              type="info"
              message="Ihr Träger hat das Ändern einzelner Rechnungen ausgeschaltet. Sie können es in den Einstellungen der Abrechnung einschalten."
              action={
                <ButtonLink
                  href={traegerPath("/traeger/einstellungen/abrechnung")}
                  variant="surface"
                  size="compact"
                  tooltip="Öffnet die Einstellungen der Abrechnung, dort schalten Sie das Ändern ein."
                >
                  Zu den Einstellungen
                </ButtonLink>
              }
              actionLayout="responsive"
            />
          ) : null}
          <div className="grid gap-6 lg:grid-cols-[minmax(0,3fr)_minmax(0,2fr)] lg:items-start">
            <CareAmendedInvoiceCard
              invoice={invoice}
              editable={open && (amendable?.positions ?? false)}
              onDraft={setDraft}
            />
            <div className="space-y-6">
              <CareAmendmentHistoryCard
                invoice={invoice}
                editable={open}
                onDraft={setDraft}
              />
              {!open && invoice.id ? (
                <InvoiceExportHistoryCard
                  kind="care-invoices"
                  invoiceId={invoice.id}
                />
              ) : null}
              {someOff ? (
                <p className="px-1 text-sm text-gray-600">
                  Weitere Änderungen schaltet Ihr Träger in den Einstellungen
                  der Abrechnung ein.
                </p>
              ) : null}
            </div>
          </div>
          {open ? (
            <CareInvoiceAmendmentModal
              run={run}
              invoice={invoice}
              draft={draft}
              onClose={() => setDraft(null)}
              onSaved={(next) => {
                setDraft(null);
                setRun(next);
                success("Die Rechnung ist geändert.");
              }}
            />
          ) : null}
        </>
      ) : null}
    </TenantPage>
  );
}

export default function TraegerCareInvoiceAmendPage() {
  return (
    <Suspense fallback={null}>
      <CareInvoiceAmendPage />
    </Suspense>
  );
}
