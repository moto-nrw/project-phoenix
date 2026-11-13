"use client";

// Die Unterschrift eines Betreuungsvertrags im Träger-Büro (#4679): wer wann
// auf welchem Weg unterschrieben hat, ob die Prüfsummen noch stimmen, und
// solange der Vertrag wartet, die Erinnerungen. Ein Vertrag, den die Eltern
// auf Papier unterschrieben haben, wird hier mit seinem PDF erfasst.

import { Download, FileSignature, Upload } from "lucide-react";
import dynamic from "next/dynamic";
import { useState } from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import {
  formatBerlinDate,
  formatBerlinTime,
  formatDate,
} from "~/lib/date-helpers";
import { downloadBlob } from "~/lib/file-download";
import { useRefreshAfterWrite } from "~/lib/hooks/use-refresh-after-write";
import { createLogger } from "~/lib/logger";
import {
  type CareContractDetail,
  type ContractSignature,
  downloadPaperSignature,
} from "~/lib/traeger-care-contracts-api";

// The window with the date picker and the upload loads only when the office
// records a paper contract.
const PaperSignatureModal = dynamic(
  () =>
    import("./care-contract-paper-modal").then(
      (module) => module.PaperSignatureModal,
    ),
  { ssr: false },
);

const logger = createLogger({ component: "CareContractSignatureCard" });

const CHANNEL_LABELS: Record<ContractSignature["channel"], string> = {
  parent_account: "In der Eltern-App",
  mail_link: "Über den Link in der Mail",
  office: "Auf Papier, erfasst im Träger-Büro",
};

const METHOD_LABELS: Record<ContractSignature["method"], string> = {
  drawn: "Gezeichnet",
  typed: "Getippter Name",
  paper: "Papier mit Unterschrift",
};

/** The card is shown for a contract that waits or is signed. */
export function CareContractSignatureCard({
  contract,
  onChanged,
}: Readonly<{ contract: CareContractDetail; onChanged: () => Promise<void> }>) {
  const [paperOpen, setPaperOpen] = useState(false);
  // The reload after the paper contract stands above the card, also after
  // the dialog closed (#6652).
  const afterPaper = useRefreshAfterWrite(
    "die Anzeige des Vertrags",
    onChanged,
  );
  const signature = contract.signature;
  const waiting =
    contract.status === "entwurf" || contract.status === "versendet";

  return (
    <>
      <SectionCard
        icon={FileSignature}
        title="Unterschrift"
        description={
          signature
            ? "So haben die Eltern den Vertrag unterschrieben."
            : contract.concluded
              ? contract.concluded.method === "mit_anmeldung"
                ? "Der Vertrag ist bereits mit der Annahme der Anmeldung abgeschlossen."
                : "Der Vertrag ist bereits abgeschlossen."
              : contract.status === "versendet"
                ? "Die Eltern unterschreiben in der Eltern-App oder über den Link in der Mail."
                : "Nach dem Senden können die Eltern online unterschreiben."
        }
        titleBadge={
          signature ? (
            <StatusBadge label="Unterschrieben" tone="green" />
          ) : contract.concluded ? (
            <StatusBadge label="Abgeschlossen" tone="green" />
          ) : contract.status === "versendet" ? (
            <StatusBadge label="Wartet auf Eltern" tone="warning" />
          ) : null
        }
        action={
          waiting && !signature ? (
            <Button
              type="button"
              variant="outline"
              size="md"
              tooltip="Öffnet ein Fenster, um den unterschriebenen Vertrag als PDF hochzuladen."
              onClick={() => setPaperOpen(true)}
            >
              <Upload className="mr-2 h-4 w-4" aria-hidden="true" />
              Papiervertrag erfassen
            </Button>
          ) : null
        }
      >
        <LoadErrorAlert error={afterPaper.error} className="mb-4" />
        {signature ? (
          <SignatureDetails contractId={contract.id} signature={signature} />
        ) : (
          <WaitingDetails contract={contract} />
        )}
      </SectionCard>
      {paperOpen ? (
        <PaperSignatureModal
          contract={contract}
          onClose={() => setPaperOpen(false)}
          onSaved={() => {
            setPaperOpen(false);
            return afterPaper.refresh();
          }}
        />
      ) : null}
    </>
  );
}

function SignatureDetails({
  contractId,
  signature,
}: Readonly<{ contractId: string; signature: ContractSignature }>) {
  const actionError = useApiErrorDisplay();
  const paper = signature.method === "paper";

  const downloadPaper = async () => {
    try {
      const file = await downloadPaperSignature(contractId);
      downloadBlob(file.blob, file.filename);
    } catch (err) {
      logger.error("traeger_care_contract_paper_download_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void actionError.show(err, { object: "der Papiervertrag" });
    }
  };

  return (
    <div className="space-y-4">
      <div className="grid gap-4 sm:grid-cols-[minmax(0,1fr)_14rem]">
        <DataGrid columns={1}>
          <DataField label="Unterschrieben von">
            {signature.signerName}
          </DataField>
          <DataField label={paper ? "Unterschrieben am" : "Zeitpunkt"}>
            {paper
              ? formatDate(signature.signedOn)
              : `${formatBerlinDate(signature.signedAt)}, ${formatBerlinTime(signature.signedAt)} Uhr`}
          </DataField>
          <DataField label="Weg">
            {CHANNEL_LABELS[signature.channel]}
            <span className="block text-sm font-normal text-gray-500">
              {METHOD_LABELS[signature.method]}
            </span>
          </DataField>
          {signature.withdrawalEndsOn ? (
            <DataField label="Widerrufsfrist">
              bis {formatDate(signature.withdrawalEndsOn)}
              <span className="block text-sm font-normal text-gray-500">
                {signature.earlyStartRequested
                  ? "Die Eltern wollen, dass die Betreuung schon in der Frist beginnt."
                  : "Die Eltern haben keinen früheren Beginn verlangt."}
              </span>
            </DataField>
          ) : null}
        </DataGrid>
        {paper ? (
          <div className="flex items-start sm:justify-end">
            <Button
              type="button"
              variant="outline"
              size="md"
              tooltip="Lädt das PDF des unterschriebenen Papiervertrags herunter."
              onClick={() => void downloadPaper()}
            >
              <Download className="mr-2 h-4 w-4" aria-hidden="true" />
              Papiervertrag laden
            </Button>
          </div>
        ) : signature.imagePng ? (
          <figure className="flex flex-col gap-1">
            <div className="moto-content-surface flex h-28 items-center justify-center rounded-xl border p-3">
              {/* oxlint-disable-next-line nextjs/no-img-element -- a data URL of the stored signature, no remote image */}
              <img
                src={`data:image/png;base64,${signature.imagePng}`}
                alt={`Unterschrift von ${signature.signerName}`}
                className="max-h-full max-w-full object-contain"
              />
            </div>
            <figcaption className="text-sm text-gray-500">
              Gespeicherte Unterschrift
            </figcaption>
          </figure>
        ) : null}
      </div>
      <Alert
        type={signature.intact ? "success" : "warning"}
        title={signature.intact ? "Prüfsumme stimmt" : "Prüfsumme stimmt nicht"}
        message={
          signature.intact
            ? "Das unterschriebene PDF passt zum gespeicherten Nachweis."
            : "Der gespeicherte Nachweis wurde verändert. Melden Sie das dem moto-Team."
        }
        className="[&>div]:min-w-0"
        details={
          <p className="mt-2 text-sm">
            Unterschriebenes PDF (SHA-256):{" "}
            <span className="font-mono break-all">
              {signature.documentSha256}
            </span>
          </p>
        }
      />
    </div>
  );
}

function WaitingDetails({
  contract,
}: Readonly<{ contract: CareContractDetail }>) {
  if (contract.concluded) {
    return (
      <p className="text-sm text-gray-600">
        Eine weitere Unterschrift ist nicht nötig.
      </p>
    );
  }
  if (contract.status !== "entwurf" && contract.status !== "versendet") {
    return null;
  }
  if (contract.status === "entwurf") {
    return (
      <p className="text-sm text-gray-600">
        Haben die Eltern schon auf Papier unterschrieben? Dann erfassen Sie den
        Vertrag mit „Papiervertrag erfassen“.
      </p>
    );
  }
  return (
    <div className="space-y-3">
      <p className="text-sm text-gray-600">
        Bis zur Unterschrift bekommen die Eltern Erinnerungen per Mail. Nach wie
        vielen Tagen, legen Sie unter „Einstellungen“ → „Allgemein“ →
        „Betreuungsverträge“ fest.
      </p>
      {contract.reminders.length > 0 ? (
        <ol className="space-y-2">
          {contract.reminders.map((reminder) => (
            <li
              key={reminder.number}
              className="flex items-center justify-between gap-3 text-sm"
            >
              <span className="font-medium text-gray-900">
                {reminder.number}. Erinnerung
              </span>
              <span className="text-gray-500">
                {formatBerlinDate(reminder.sentAt)},{" "}
                {formatBerlinTime(reminder.sentAt)} Uhr
              </span>
            </li>
          ))}
        </ol>
      ) : (
        <p className="text-sm text-gray-500">
          Noch keine Erinnerung geschickt.
        </p>
      )}
    </div>
  );
}
