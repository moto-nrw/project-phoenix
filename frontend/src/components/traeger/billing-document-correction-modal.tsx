"use client";

// Storno oder Gutschrift einer ausgestellten Rechnung (#4690). Beides ist ein
// eigenes Dokument mit eigener Nummer und Pflicht-Begründung; die Rechnung
// selbst ändert sich nie. Der Storno nimmt die ganze Rechnung zurück, die
// Gutschrift einen Betrag für ein Kind.

import { useEffect, useRef, useState } from "react";
import { EditActions } from "~/components/ui/edit-actions";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { Textarea } from "~/components/ui/textarea";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  type BillingChild,
  type BillingDocument,
  cancelBillingDocument,
  creditBillingDocument,
} from "~/lib/traeger-billing-api";
import { formatEuro } from "./contribution-format";
import { centsToEuroInput, euroInputToCents } from "./holiday-period-model";

const logger = createLogger({ component: "BillingDocumentCorrectionModal" });

/** The invoice the Abrechnung cancels, or the child it credits. */
export type BillingDocumentCorrectionTarget =
  | { readonly kind: "cancellation"; readonly document: BillingDocument }
  | {
      readonly kind: "credit";
      readonly document: BillingDocument;
      readonly child: BillingChild;
      /** What the invoice and its earlier Gutschriften leave of the child. */
      readonly leftCents: number;
    };

export function BillingDocumentCorrectionModal({
  runId,
  target,
  onClose,
  onIssued,
}: Readonly<{
  runId: string;
  target: BillingDocumentCorrectionTarget | null;
  onClose: () => void;
  onIssued: (issued: BillingDocument) => void;
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
    setAmount("");
    setReason("");
    clearErrors();
  }, [target, clearErrors]);

  const submit = async () => {
    if (!target) return;
    formErrors.clear();
    const fields: Record<string, string> = {};
    const amountCents =
      target.kind === "credit" ? euroInputToCents(amount) : null;
    if (target.kind === "credit") {
      if (amountCents === null || amountCents <= 0)
        fields.amount_cents = "Bitte geben Sie einen Betrag wie 5,00 an.";
      else if (amountCents > target.leftCents)
        fields.amount_cents = `Höchstens ${formatEuro(target.leftCents)}.`;
    }
    if (!reason.trim()) fields.reason = "Bitte geben Sie eine Begründung an.";
    if (Object.keys(fields).length > 0) {
      formErrors.invalid("Bitte prüfen Sie Ihre Angaben.", fields);
      return;
    }
    setSaving(true);
    try {
      const issued =
        target.kind === "credit"
          ? await creditBillingDocument(runId, target.document.id, {
              childId: target.child.childId,
              amountCents: amountCents ?? 0,
              reason: reason.trim(),
            })
          : await cancelBillingDocument(
              runId,
              target.document.id,
              reason.trim(),
            );
      success(
        target.kind === "credit"
          ? `Gutschrift ${issued.number} ist ausgestellt.`
          : `Storno ${issued.number} ist ausgestellt.`,
      );
      onIssued(issued);
    } catch (err) {
      logger.error("billing_document_correction_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, {
        object: target.kind === "credit" ? "die Gutschrift" : "der Storno",
      });
    } finally {
      setSaving(false);
    }
  };

  const credit = target?.kind === "credit" ? target : null;
  return (
    <FormModal
      isOpen={target !== null}
      onClose={onClose}
      closeDisabled={saving}
      isBackdropDismissDisabled
      title={
        credit
          ? `Gutschrift für ${credit.child.firstName}`
          : `Rechnung ${target?.document.number ?? ""} stornieren?`
      }
      size="md"
      error={formErrors.error}
      footer={
        <EditActions
          onCancel={onClose}
          onSave={() => formRef.current?.requestSubmit()}
          saving={saving}
          saveLabel={credit ? "Gutschreiben" : "Stornieren"}
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
            {credit
              ? `Noch offen für ${credit.child.firstName}: ${formatEuro(credit.leftCents)}. Die Gutschrift bekommt eine eigene Nummer und ein eigenes Schreiben.`
              : `Der Storno nimmt ${formatEuro(target.document.totalCents)} zurück. Er bekommt eine eigene Nummer und ein eigenes Schreiben. Danach kommt die Familie in den nächsten Lauf und bekommt dort eine neue Rechnung.`}
          </p>
          {credit ? (
            <Input
              id="billing-credit-amount"
              name="amount_cents"
              label="Betrag in Euro"
              inputMode="decimal"
              placeholder={centsToEuroInput(Math.min(500, credit.leftCents))}
              value={amount}
              error={formErrors.fieldError("amount_cents") ?? undefined}
              onChange={(event) => setAmount(event.target.value)}
            />
          ) : null}
          <Textarea
            id="billing-document-correction-reason"
            name="reason"
            label="Begründung"
            placeholder={
              credit
                ? "Zum Beispiel: Am ersten Ferientag krank."
                : "Zum Beispiel: Anschrift war falsch. Neue Rechnung folgt."
            }
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
