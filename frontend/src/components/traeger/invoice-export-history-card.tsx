"use client";

// Was mit einer ausgestellten Rechnung geschah (#4693): Ausstellung mit
// der Zahl der Positionen, jeder Versand an die Familie (E-Mail und
// Elternpostfach), ein Storno, die eingereichte Lastschrift und jede Datei
// für Kämmerei oder Bank mit Nummer, Zeit und Konto. Nur lesen.

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
    case "sent":
      return step.note === "cancellation"
        ? "Storno an die Familie geschickt"
        : "An die Familie geschickt";
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
  if (step.kind === "sent") return "Per E-Mail und im Elternpostfach";
  if (step.kind === "exported" && step.file) return step.file.fileName;
  return null;
}

function StepRow({ step }: Readonly<{ step: InvoiceStep }>) {
  const detail = stepDetail(step);
  return (
    <li className="flex gap-3 py-3 first:pt-0 last:pb-0">
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
        {detail ? (
          <p
            className={`truncate text-sm text-gray-600 ${step.kind === "exported" ? "font-mono" : ""}`}
          >
            {detail}
          </p>
        ) : null}
        <p className="text-sm text-gray-500">
          {exportedAtLabel(step.at)} · {byLabel(step.byYou, step.byAccount)}
        </p>
      </div>
    </li>
  );
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
          ? "Was mit der Rechnung geschah. Sie war noch in keiner Datei für Kämmerei oder Bank."
          : `Was mit der Rechnung geschah. Sie war in ${exports === 1 ? "einer Datei" : `${exports} Dateien`} für Kämmerei oder Bank.`
      }
      loading={!history && !load.error}
    >
      {load.error ? (
        <LoadErrorAlert error={load.error} actionLayout="responsive" />
      ) : null}
      {history ? (
        <ol className="divide-y divide-gray-100">
          {history.steps.map((step, index) => (
            <StepRow key={`${step.kind}-${step.at}-${index}`} step={step} />
          ))}
        </ol>
      ) : null}
    </SectionCard>
  );
}
