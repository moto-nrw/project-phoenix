"use client";

// Einen Betrag im Abrechnungslauf korrigieren (#4668): neuer Betrag eines
// Kindes und Pflicht-Begründung. Frühere Korrekturen bleiben im Verlauf.

import { useEffect, useRef, useState } from "react";
import { EditActions } from "~/components/ui/edit-actions";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { Textarea } from "~/components/ui/textarea";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  type BillingChild,
  type BillingRunDetail,
  correctBillingRun,
} from "~/lib/traeger-billing-api";
import { formatEuro } from "./contribution-format";
import { centsToEuroInput, euroInputToCents } from "./holiday-period-model";

const logger = createLogger({ component: "BillingCorrectionModal" });

/** The child whose amount the Abrechnung corrects. */
export interface BillingCorrectionTarget {
  readonly registrationId: string;
  readonly child: BillingChild;
}

export function BillingCorrectionModal({
  runId,
  target,
  onClose,
  onCorrected,
}: Readonly<{
  runId: string;
  target: BillingCorrectionTarget | null;
  onClose: () => void;
  onCorrected: (run: BillingRunDetail) => void;
}>) {
  const [amount, setAmount] = useState("");
  const [reason, setReason] = useState("");
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;
  const { success } = useToast();

  useEffect(() => {
    if (!target) return;
    setAmount(centsToEuroInput(target.child.totalCents));
    setReason("");
    clearErrors();
  }, [target, clearErrors]);

  const submit = async () => {
    if (!target) return;
    formErrors.clear();
    const totalCents = euroInputToCents(amount);
    const fields: Record<string, string> = {};
    if (totalCents === null)
      fields.total_cents = "Bitte geben Sie einen Betrag wie 45,00 an.";
    else if (totalCents === target.child.totalCents)
      fields.total_cents = "Der Betrag ist unverändert.";
    if (!reason.trim()) fields.reason = "Bitte geben Sie eine Begründung an.";
    if (totalCents === null || Object.keys(fields).length > 0) {
      formErrors.invalid("Bitte prüfen Sie Ihre Angaben.", fields);
      return;
    }
    setSaving(true);
    try {
      const run = await correctBillingRun(runId, {
        registrationId: target.registrationId,
        childId: target.child.childId,
        totalCents,
        reason: reason.trim(),
      });
      success("Der Betrag ist korrigiert.");
      onCorrected(run);
    } catch (err) {
      logger.error("billing_correction_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "die Korrektur" });
    } finally {
      setSaving(false);
    }
  };

  const name = target
    ? `${target.child.firstName} ${target.child.lastName}`
    : "";
  return (
    <FormModal
      isOpen={target !== null}
      onClose={onClose}
      closeDisabled={saving}
      isBackdropDismissDisabled
      title={`Betrag für ${name} korrigieren`}
      size="md"
      error={formErrors.error}
      footer={
        <EditActions
          onCancel={onClose}
          onSave={() => formRef.current?.requestSubmit()}
          saving={saving}
          saveLabel="Korrigieren"
        />
      }
    >
      {target ? (
        <form
          ref={formRef}
          noValidate
          className="space-y-4"
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
        >
          <p className="text-sm text-gray-700">
            Jetzt: {formatEuro(target.child.totalCents)}. Die Korrektur steht
            als eigene Position auf der Rechnung.
          </p>
          <Input
            id="billing-correction-amount"
            name="total_cents"
            label="Neuer Betrag in Euro"
            inputMode="decimal"
            value={amount}
            error={formErrors.fieldError("total_cents") ?? undefined}
            onChange={(event) => setAmount(event.target.value)}
          />
          <Textarea
            id="billing-correction-reason"
            name="reason"
            label="Begründung"
            placeholder="Zum Beispiel: Am ersten Ferientag krank, Tag wird erstattet."
            value={reason}
            maxLength={500}
            error={formErrors.fieldError("reason") ?? undefined}
            onChange={(event) => setReason(event.target.value)}
          />
        </form>
      ) : null}
    </FormModal>
  );
}
