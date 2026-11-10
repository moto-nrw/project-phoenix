"use client";

// Was mit einer ausgestellten Rechnung geschah (#4693): Ausstellung mit
// der Zahl der Positionen, jeder Versand an die Familie (E-Mail und
// Elternpostfach), ein Storno, die eingereichte Lastschrift und jede Datei
// für Kämmerei, Steuerbüro oder Bank mit Nummer, Zeit und Konto. Nur lesen.

import {
  Ban,
  FileSpreadsheet,
  Landmark,
  Mail,
  ReceiptText,
} from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import {
  FINANCE_FILE_KINDS,
  byLabel,
  exportedAtLabel,
} from "~/components/traeger/finance-export-format";
import { ListRow } from "~/components/ui/list-row";
import { IconTile } from "~/components/ui/icon-tile";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiLoadError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  type InvoiceHistory,
  type InvoiceHistoryKind,
  type InvoiceStep,
  fetchInvoiceHistory,
} from "~/lib/traeger-finance-exports-api";

const logger = createLogger({ component: "InvoiceExportHistoryCard" });

function stepIcon(step: InvoiceStep) {
  switch (step.kind) {
    case "issued":
      return <IconTile size="xs" tone="green" icon={<ReceiptText />} />;
    case "mail_requested":
    case "sent":
      return <IconTile size="xs" tone="blue" icon={<Mail />} />;
    case "cancelled":
      return <IconTile size="xs" tone="red" icon={<Ban />} />;
    case "submitted":
      return <IconTile size="xs" tone="neutral" icon={<Landmark />} />;
    case "exported":
      return <IconTile size="xs" tone="orange" icon={<FileSpreadsheet />} />;
  }
}

function stepTitle(step: InvoiceStep): string {
  switch (step.kind) {
    case "issued":
      return step.positions === 1
        ? "Ausgestellt mit 1 Position"
        : `Ausgestellt mit ${step.positions} Positionen`;
    case "mail_requested":
      return step.note === "cancellation"
        ? "Storno-Versand beauftragt"
        : "Versand beauftragt";
    case "sent":
      return step.note === "cancellation"
        ? "Storno per E-Mail verschickt"
        : "Per E-Mail verschickt";
    case "cancelled":
      return `Storniert mit ${step.note}`;
    case "submitted":
      return "Lastschrift bei der Bank eingereicht";
    case "exported":
      return step.file
        ? `${FINANCE_FILE_KINDS[step.file.format]} Nr. ${step.file.fileNumber}`
        : "Exportiert";
  }
}

function stepDetail(step: InvoiceStep): string | null {
  if (step.kind === "mail_requested")
    return "E-Mail-Versand beauftragt. Den Stand sehen Sie beim Rechnungslauf.";
  if (step.kind === "sent") return "Vom E-Mail-Dienst angenommen.";
  if (step.kind === "exported" && step.file) return step.file.fileName;
  return null;
}

/** The steps of an issued invoice, oldest first; nothing to act on. */
export function InvoiceExportHistoryCard({
  kind,
  invoiceId,
}: Readonly<{ kind: InvoiceHistoryKind; invoiceId: string }>) {
  const [history, setHistory] = useState<InvoiceHistory | null>(null);
  const load = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = load;

  const read = useCallback(async () => {
    try {
      setHistory(await fetchInvoiceHistory(kind, invoiceId));
      clearLoadError();
    } catch (err) {
      logger.error("traeger_invoice_history_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "den Weg der Rechnung",
        retry: () => void read(),
      });
    }
  }, [kind, invoiceId, clearLoadError, showLoadError]);

  useEffect(() => {
    void read();
  }, [read]);

  const exports =
    history?.steps.filter((step) => step.kind === "exported").length ?? 0;

  return (
    <SectionCard
      title="Ausstellung, Versand und Exporte"
      description={
        exports === 0
          ? "Was mit der Rechnung geschah. Sie war noch in keiner Datei für Kämmerei, Steuerbüro oder Bank."
          : `Was mit der Rechnung geschah. Sie war in ${exports === 1 ? "einer Datei" : `${exports} Dateien`} für Kämmerei, Steuerbüro oder Bank.`
      }
      loading={!history && !load.error}
    >
      {load.error ? (
        <LoadErrorAlert error={load.error} actionLayout="responsive" />
      ) : null}
      {history ? (
        <ol className="space-y-2">
          {history.steps.map((step, index) => (
            <li key={`${step.kind}-${step.at}-${index}`}>
              <ListRow className="items-start">
                {stepIcon(step)}
                <div className="min-w-0 flex-1">
                  <p className="flex flex-wrap items-center gap-x-2 gap-y-1 font-medium text-gray-900">
                    {stepTitle(step)}
                    {step.file?.repeatOf ? (
                      <StatusBadge
                        label={`Gleicher Inhalt wie Nr. ${step.file.repeatOf}`}
                        tone="gray"
                        size="xs"
                      />
                    ) : null}
                  </p>
                  {stepDetail(step) ? (
                    <p
                      className={`text-sm wrap-anywhere text-gray-600 ${step.kind === "exported" ? "font-mono" : ""}`}
                    >
                      {stepDetail(step)}
                    </p>
                  ) : null}
                  <p className="text-sm text-gray-500">
                    {exportedAtLabel(step.at)} ·{" "}
                    {byLabel(step.byYou, step.byAccount)}
                  </p>
                </div>
              </ListRow>
            </li>
          ))}
        </ol>
      ) : null}
    </SectionCard>
  );
}
