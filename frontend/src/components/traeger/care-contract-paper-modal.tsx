"use client";

// Einen auf Papier unterschriebenen Betreuungsvertrag erfassen (#4679): wer
// unterschrieben hat, an welchem Tag, und das PDF. Danach ist der Vertrag
// aktiv.

import { useState } from "react";

import { Button } from "~/components/ui/button";
import { DatePicker } from "~/components/ui/date-picker";
import { Dropzone } from "~/components/ui/dropzone";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { FieldLabel, RequiredMark } from "~/components/ui/field-label";
import { FormModal } from "~/components/ui/form-modal";
import { useFormModalPending } from "~/components/ui/use-form-modal-pending";
import { Input } from "~/components/ui/input";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { berlinTodayISO, parseISODate, toISODate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  type CareContractDetail,
  recordPaperSignature,
} from "~/lib/traeger-care-contracts-api";

const logger = createLogger({ component: "CareContractPaperModal" });

export function PaperSignatureModal({
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
  const submission = useFormModalPending();
  const saving = submission.pending;
  const formError = useApiFormError();
  const signedOnError = formError.fieldError("signed_on");
  const { success } = useToast();

  const save = async () => {
    formError.clear();
    if (!file || !signedOn || !signerName.trim()) return;
    await submission.run(async (op) => {
      try {
        await recordPaperSignature(contract.id, {
          file,
          signerName,
          signedOn: toISODate(signedOn),
        });
        success("Der Papiervertrag ist erfasst. Der Vertrag ist jetzt aktiv.");
        if (op.isCurrent()) await onSaved();
      } catch (err) {
        logger.error("traeger_care_contract_paper_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!op.isCurrent()) return;
        void formError.show(err, { object: "der Papiervertrag" });
      }
    });
  };

  return (
    <FormModal
      isOpen
      onClose={onClose}
      pending={submission}
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
          <FieldLabel
            htmlFor="care-contract-signed-on"
            required
            invalid={Boolean(signedOnError)}
          >
            Unterschrieben am
          </FieldLabel>
          <DatePicker
            id="care-contract-signed-on"
            name="signed_on"
            invalid={Boolean(signedOnError)}
            ariaDescribedBy={
              signedOnError ? "care-contract-signed-on-error" : undefined
            }
            minDate={parseISODate("2000-01-01")}
            disabled={saving}
            required
            value={signedOn}
            onChange={setSignedOn}
            maxDate={parseISODate(berlinTodayISO())}
            hideClearButton
          />
          <FieldErrorHint
            id="care-contract-signed-on-error"
            message={signedOnError}
          />
        </div>
        <div className="space-y-1.5">
          <span className="block text-sm font-medium text-gray-700">
            Unterschriebener Vertrag <RequiredMark />
          </span>
          <Dropzone
            accept="application/pdf,.pdf"
            legend="Unterschriebener Vertrag"
            label="PDF hierher ziehen"
            dragLabel="PDF hier ablegen"
            buttonLabel={file ? "Andere Datei wählen" : "PDF wählen"}
            hint="PDF, höchstens 10 MB"
            onFile={setFile}
            disabled={saving}
          />
          {file ? (
            <span className="min-w-0 text-sm break-all text-gray-600">
              {file.name}
            </span>
          ) : null}
        </div>
      </div>
    </FormModal>
  );
}
