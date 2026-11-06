"use client";

// Die Unterschrift eines Betreuungsvertrags im Träger-Büro (#4679): wer wann
// auf welchem Weg unterschrieben hat, ob die Prüfsummen noch stimmen, und
// solange der Vertrag wartet, die Erinnerungen. Ein Vertrag, den die Eltern
// auf Papier unterschrieben haben, wird hier mit seinem PDF erfasst.

import {
  Download,
  FileSignature,
  ShieldAlert,
  ShieldCheck,
  Upload,
} from "lucide-react";
import { useRef, useState } from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { DatePicker } from "~/components/ui/date-picker";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import {
  useApiErrorDisplay,
  useApiFormError,
  useToast,
} from "~/contexts/ToastContext";
import {
  berlinTodayISO,
  formatBerlinDate,
  formatBerlinTime,
  formatDate,
  parseISODate,
  toISODate,
} from "~/lib/date-helpers";
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import {
  type CareContractDetail,
  type ContractSignature,
  downloadPaperSignature,
  recordPaperSignature,
} from "~/lib/traeger-care-contracts-api";

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
            : contract.status === "versendet"
              ? "Die Eltern unterschreiben in der Eltern-App oder über den Link in der Mail."
              : "Nach dem Senden können die Eltern online unterschreiben."
        }
        titleBadge={
          signature ? (
            <StatusBadge label="Unterschrieben" tone="green" />
          ) : contract.status === "versendet" ? (
            <StatusBadge label="Wartet auf Eltern" tone="orange" />
          ) : null
        }
        action={
          waiting && !signature ? (
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() => setPaperOpen(true)}
            >
              <Upload className="mr-2 h-4 w-4" aria-hidden="true" />
              Papiervertrag erfassen
            </Button>
          ) : null
        }
      >
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
          onSaved={async () => {
            setPaperOpen(false);
            await onChanged();
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
      {signature.intact ? null : (
        <Alert
          type="warning"
          message="Die Prüfsumme der Unterschrift stimmt nicht mehr. Der gespeicherte Nachweis wurde verändert. Bitte melden Sie das dem moto-Team."
        />
      )}
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
            <figcaption className="text-xs text-gray-500">
              Gespeicherte Unterschrift
            </figcaption>
          </figure>
        ) : null}
      </div>
      <div className="flex items-start gap-2 rounded-xl bg-gray-50 px-3 py-2.5 text-xs text-gray-600">
        {signature.intact ? (
          <ShieldCheck
            className="text-moto-green-strong mt-0.5 h-4 w-4 shrink-0"
            aria-hidden="true"
          />
        ) : (
          <ShieldAlert
            className="text-moto-red-strong mt-0.5 h-4 w-4 shrink-0"
            aria-hidden="true"
          />
        )}
        <span className="min-w-0">
          {signature.intact ? "Prüfsumme stimmt" : "Prüfsumme stimmt nicht"}.
          Unterschriebenes PDF (SHA-256):{" "}
          <span className="font-mono break-all">
            {signature.documentSha256}
          </span>
        </span>
      </div>
    </div>
  );
}

function WaitingDetails({
  contract,
}: Readonly<{ contract: CareContractDetail }>) {
  if (contract.status !== "versendet") {
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
        Bis zur Unterschrift bekommen die Eltern Erinnerungen per Mail. Wann,
        legen Sie unter „Einstellungen“ im Bereich „Betreuungsverträge“ fest.
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

function PaperSignatureModal({
  contract,
  onClose,
  onSaved,
}: Readonly<{
  contract: CareContractDetail;
  onClose: () => void;
  onSaved: () => Promise<void>;
}>) {
  const party = contract.parties[0];
  const [signerName, setSignerName] = useState(
    party ? `${party.firstName} ${party.lastName}`.trim() : "",
  );
  const [signedOn, setSignedOn] = useState<Date | null>(
    parseISODate(berlinTodayISO()),
  );
  const [file, setFile] = useState<File | null>(null);
  const [saving, setSaving] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);
  const formError = useApiFormError();
  const { success } = useToast();

  const save = async () => {
    formError.clear();
    if (!file || !signedOn || !signerName.trim()) return;
    setSaving(true);
    try {
      await recordPaperSignature(contract.id, {
        file,
        signerName,
        signedOn: toISODate(signedOn),
      });
      success("Der Papiervertrag ist erfasst. Der Vertrag ist jetzt aktiv.");
      await onSaved();
    } catch (err) {
      logger.error("traeger_care_contract_paper_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formError.show(err, { object: "der Papiervertrag" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <FormModal
      isOpen
      onClose={onClose}
      closeDisabled={saving}
      title="Papiervertrag erfassen"
      error={formError.error}
      footer={
        <>
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={onClose}
            disabled={saving}
          >
            Abbrechen
          </Button>
          <Button
            type="button"
            variant="success"
            size="md"
            disabled={saving || !file || !signedOn || !signerName.trim()}
            onClick={() => void save()}
          >
            {saving ? "Wird erfasst …" : "Vertrag abschließen"}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <p className="text-sm text-gray-600">
          Für Eltern ohne Internet oder einen Vertrag, den es nur auf Papier
          gibt. Laden Sie den unterschriebenen Vertrag als PDF hoch. Danach ist
          der Vertrag aktiv.
        </p>
        <Input
          name="signer_name"
          label="Unterschrieben von"
          required
          value={signerName}
          maxLength={300}
          onChange={(event) => setSignerName(event.target.value)}
          error={formError.fieldError("signer_name")}
          disabled={saving}
        />
        <div className="space-y-1.5">
          <span className="block text-sm font-medium text-gray-700">
            Unterschrieben am
          </span>
          <DatePicker
            value={signedOn}
            onChange={setSignedOn}
            maxDate={parseISODate(berlinTodayISO())}
            hideClearButton
          />
        </div>
        <div className="space-y-1.5">
          <span className="block text-sm font-medium text-gray-700">
            Unterschriebener Vertrag (PDF, höchstens 10 MB)
          </span>
          <input
            ref={fileRef}
            id="care-contract-paper-upload"
            type="file"
            accept="application/pdf,.pdf"
            className="sr-only"
            tabIndex={-1}
            aria-hidden="true"
            onChange={(event) => setFile(event.target.files?.[0] ?? null)}
          />
          <div className="flex flex-wrap items-center gap-3">
            <Button
              type="button"
              variant="outline"
              size="md"
              disabled={saving}
              onClick={() => fileRef.current?.click()}
            >
              <Upload className="mr-2 h-4 w-4" aria-hidden="true" />
              {file ? "Andere Datei wählen" : "PDF wählen"}
            </Button>
            <span className="min-w-0 text-sm break-all text-gray-600">
              {file ? file.name : "Noch keine Datei gewählt."}
            </span>
          </div>
        </div>
      </div>
    </FormModal>
  );
}
